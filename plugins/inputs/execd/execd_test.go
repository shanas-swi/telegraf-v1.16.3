//go:build !windows
// +build !windows

package execd

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shanas-swi/telegraf-v1.16.3/agent"
	"github.com/shanas-swi/telegraf-v1.16.3/config"
	"github.com/shanas-swi/telegraf-v1.16.3/metric"
	"github.com/shanas-swi/telegraf-v1.16.3/models"
	"github.com/shanas-swi/telegraf-v1.16.3/testutil"
	"github.com/stretchr/testify/require"

	"github.com/shanas-swi/telegraf-v1.16.3/plugins/parsers"
	"github.com/shanas-swi/telegraf-v1.16.3/plugins/serializers"

	"github.com/shanas-swi/telegraf-v1.16.3"
)

func TestSettingConfigWorks(t *testing.T) {
	cfg := `
	[[inputs.execd]]
		command = ["a", "b", "c"]
		restart_delay = "1m"
		signal = "SIGHUP"
	`
	conf := config.NewConfig()
	require.NoError(t, conf.LoadConfigData([]byte(cfg)))

	require.Len(t, conf.Inputs, 1)
	inp, ok := conf.Inputs[0].Input.(*Execd)
	require.True(t, ok)
	require.EqualValues(t, []string{"a", "b", "c"}, inp.Command)
	require.EqualValues(t, 1*time.Minute, inp.RestartDelay)
	require.EqualValues(t, "SIGHUP", inp.Signal)
}

func TestExternalInputWorks(t *testing.T) {
	influxParser, err := parsers.NewInfluxParser()
	require.NoError(t, err)

	exe, err := os.Executable()
	require.NoError(t, err)

	e := &Execd{
		Command:      []string{exe, "-counter"},
		RestartDelay: config.Duration(5 * time.Second),
		parser:       influxParser,
		Signal:       "STDIN",
		Log:          testutil.Logger{},
	}

	metrics := make(chan telegraf.Metric, 10)
	defer close(metrics)
	acc := agent.NewAccumulator(&TestMetricMaker{}, metrics)

	require.NoError(t, e.Start(acc))
	require.NoError(t, e.Gather(acc))

	// grab a metric and make sure it's a thing
	m := readChanWithTimeout(t, metrics, 10*time.Second)

	e.Stop()

	require.Equal(t, "counter", m.Name())
	val, ok := m.GetField("count")
	require.True(t, ok)
	require.EqualValues(t, 0, val)
}

func TestParsesLinesContainingNewline(t *testing.T) {
	parser, err := parsers.NewInfluxParser()
	require.NoError(t, err)

	metrics := make(chan telegraf.Metric, 10)
	defer close(metrics)
	acc := agent.NewAccumulator(&TestMetricMaker{}, metrics)

	e := &Execd{
		RestartDelay: config.Duration(5 * time.Second),
		parser:       parser,
		Signal:       "STDIN",
		acc:          acc,
		Log:          testutil.Logger{},
	}

	cases := []struct {
		Name  string
		Value string
	}{
		{
			Name:  "no-newline",
			Value: "my message",
		}, {
			Name:  "newline",
			Value: "my\nmessage",
		},
	}

	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			line := fmt.Sprintf("event message=\"%v\" 1587128639239000000", test.Value)

			e.cmdReadOut(strings.NewReader(line))

			m := readChanWithTimeout(t, metrics, 1*time.Second)

			require.Equal(t, "event", m.Name())
			val, ok := m.GetField("message")
			require.True(t, ok)
			require.Equal(t, test.Value, val)
		})
	}
}

func readChanWithTimeout(t *testing.T, metrics chan telegraf.Metric, timeout time.Duration) telegraf.Metric {
	to := time.NewTimer(timeout)
	defer to.Stop()
	select {
	case m := <-metrics:
		return m
	case <-to.C:
		require.FailNow(t, "timeout waiting for metric")
	}
	return nil
}

type TestMetricMaker struct{}

func (tm *TestMetricMaker) Name() string {
	return "TestPlugin"
}

func (tm *TestMetricMaker) LogName() string {
	return tm.Name()
}

func (tm *TestMetricMaker) MakeMetric(metric telegraf.Metric) telegraf.Metric {
	return metric
}

func (tm *TestMetricMaker) Log() telegraf.Logger {
	return models.NewLogger("TestPlugin", "test", "")
}

var counter = flag.Bool("counter", false,
	"if true, act like line input program instead of test")

func TestMain(m *testing.M) {
	flag.Parse()
	if *counter {
		runCounterProgram()
		os.Exit(0)
	}
	code := m.Run()
	os.Exit(code)
}

func runCounterProgram() {
	i := 0
	serializer, err := serializers.NewInfluxSerializer()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERR InfluxSerializer failed to load")
		os.Exit(1)
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		metric, _ := metric.New("counter",
			map[string]string{},
			map[string]interface{}{
				"count": i,
			},
			time.Now(),
		)
		i++

		b, err := serializer.Serialize(metric)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERR %v\n", err)
			os.Exit(1)
		}
		fmt.Fprint(os.Stdout, string(b))
	}

}
