package prometheus

import (
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/shanas-swi/telegraf-v1.16.3"
	"github.com/shanas-swi/telegraf-v1.16.3/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleTextFormat = `# HELP go_gc_duration_seconds A summary of the GC invocation durations.
# TYPE go_gc_duration_seconds summary
go_gc_duration_seconds{quantile="0"} 0.00010425500000000001
go_gc_duration_seconds{quantile="0.25"} 0.000139108
go_gc_duration_seconds{quantile="0.5"} 0.00015749400000000002
go_gc_duration_seconds{quantile="0.75"} 0.000331463
go_gc_duration_seconds{quantile="1"} 0.000667154
go_gc_duration_seconds_sum 0.0018183950000000002
go_gc_duration_seconds_count 7
# HELP go_goroutines Number of goroutines that currently exist.
# TYPE go_goroutines gauge
go_goroutines 15
# HELP test_metric An untyped metric with a timestamp
# TYPE test_metric untyped
test_metric{label="value"} 1.0 1490802350000
`
const sampleSummaryTextFormat = `# HELP go_gc_duration_seconds A summary of the GC invocation durations.
# TYPE go_gc_duration_seconds summary
go_gc_duration_seconds{quantile="0"} 0.00010425500000000001
go_gc_duration_seconds{quantile="0.25"} 0.000139108
go_gc_duration_seconds{quantile="0.5"} 0.00015749400000000002
go_gc_duration_seconds{quantile="0.75"} 0.000331463
go_gc_duration_seconds{quantile="1"} 0.000667154
go_gc_duration_seconds_sum 0.0018183950000000002
go_gc_duration_seconds_count 7
`
const sampleGaugeTextFormat = `
# HELP go_goroutines Number of goroutines that currently exist.
# TYPE go_goroutines gauge
go_goroutines 15 1490802350000
`

func TestPrometheusGeneratesMetrics(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, sampleTextFormat)
	}))
	defer ts.Close()

	p := &Prometheus{
		Log:    testutil.Logger{},
		URLs:   []string{ts.URL},
		URLTag: "url",
	}

	var acc testutil.Accumulator

	err := acc.GatherError(p.Gather)
	require.NoError(t, err)

	assert.True(t, acc.HasFloatField("go_gc_duration_seconds", "count"))
	assert.True(t, acc.HasFloatField("go_goroutines", "gauge"))
	assert.True(t, acc.HasFloatField("test_metric", "value"))
	assert.True(t, acc.HasTimestamp("test_metric", time.Unix(1490802350, 0)))
	assert.False(t, acc.HasTag("test_metric", "address"))
	assert.True(t, acc.TagValue("test_metric", "url") == ts.URL+"/metrics")
}

func TestPrometheusGeneratesMetricsWithHostNameTag(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, sampleTextFormat)
	}))
	defer ts.Close()

	p := &Prometheus{
		Log:                testutil.Logger{},
		KubernetesServices: []string{ts.URL},
		URLTag:             "url",
	}
	u, _ := url.Parse(ts.URL)
	tsAddress := u.Hostname()

	var acc testutil.Accumulator

	err := acc.GatherError(p.Gather)
	require.NoError(t, err)

	assert.True(t, acc.HasFloatField("go_gc_duration_seconds", "count"))
	assert.True(t, acc.HasFloatField("go_goroutines", "gauge"))
	assert.True(t, acc.HasFloatField("test_metric", "value"))
	assert.True(t, acc.HasTimestamp("test_metric", time.Unix(1490802350, 0)))
	assert.True(t, acc.TagValue("test_metric", "address") == tsAddress)
	assert.True(t, acc.TagValue("test_metric", "url") == ts.URL)
}

func TestPrometheusGeneratesMetricsAlthoughFirstDNSFails(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, sampleTextFormat)
	}))
	defer ts.Close()

	p := &Prometheus{
		Log:                testutil.Logger{},
		URLs:               []string{ts.URL},
		KubernetesServices: []string{"http://random.telegraf.local:88/metrics"},
	}

	var acc testutil.Accumulator

	err := acc.GatherError(p.Gather)
	require.NoError(t, err)

	assert.True(t, acc.HasFloatField("go_gc_duration_seconds", "count"))
	assert.True(t, acc.HasFloatField("go_goroutines", "gauge"))
	assert.True(t, acc.HasFloatField("test_metric", "value"))
	assert.True(t, acc.HasTimestamp("test_metric", time.Unix(1490802350, 0)))
}

func TestPrometheusGeneratesSummaryMetricsV2(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, sampleSummaryTextFormat)
	}))
	defer ts.Close()

	p := &Prometheus{
		URLs:          []string{ts.URL},
		URLTag:        "url",
		MetricVersion: 2,
	}

	var acc testutil.Accumulator

	err := acc.GatherError(p.Gather)
	require.NoError(t, err)

	assert.True(t, acc.TagSetValue("prometheus", "quantile") == "0")
	assert.True(t, acc.HasFloatField("prometheus", "go_gc_duration_seconds_sum"))
	assert.True(t, acc.HasFloatField("prometheus", "go_gc_duration_seconds_count"))
	assert.True(t, acc.TagValue("prometheus", "url") == ts.URL+"/metrics")

}

func TestSummaryMayContainNaN(t *testing.T) {
	const data = `# HELP go_gc_duration_seconds A summary of the GC invocation durations.
# TYPE go_gc_duration_seconds summary
go_gc_duration_seconds{quantile="0"} NaN
go_gc_duration_seconds{quantile="1"} NaN
go_gc_duration_seconds_sum 42.0
go_gc_duration_seconds_count 42
`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, data)
	}))
	defer ts.Close()

	p := &Prometheus{
		URLs:          []string{ts.URL},
		URLTag:        "",
		MetricVersion: 2,
	}

	var acc testutil.Accumulator

	err := p.Gather(&acc)
	require.NoError(t, err)

	expected := []telegraf.Metric{
		testutil.MustMetric(
			"prometheus",
			map[string]string{
				"quantile": "0",
			},
			map[string]interface{}{
				"go_gc_duration_seconds": math.NaN(),
			},
			time.Unix(0, 0),
			telegraf.Summary,
		),
		testutil.MustMetric(
			"prometheus",
			map[string]string{
				"quantile": "1",
			},
			map[string]interface{}{
				"go_gc_duration_seconds": math.NaN(),
			},
			time.Unix(0, 0),
			telegraf.Summary,
		),
		testutil.MustMetric(
			"prometheus",
			map[string]string{},
			map[string]interface{}{
				"go_gc_duration_seconds_sum":   42.0,
				"go_gc_duration_seconds_count": 42.0,
			},
			time.Unix(0, 0),
			telegraf.Summary,
		),
	}

	testutil.RequireMetricsEqual(t, expected, acc.GetTelegrafMetrics(),
		testutil.IgnoreTime(), testutil.SortMetrics())
}

func TestPrometheusGeneratesGaugeMetricsV2(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, sampleGaugeTextFormat)
	}))
	defer ts.Close()

	p := &Prometheus{
		URLs:          []string{ts.URL},
		URLTag:        "url",
		MetricVersion: 2,
	}

	var acc testutil.Accumulator

	err := acc.GatherError(p.Gather)
	require.NoError(t, err)

	assert.True(t, acc.HasFloatField("prometheus", "go_goroutines"))
	assert.True(t, acc.TagValue("prometheus", "url") == ts.URL+"/metrics")
	assert.True(t, acc.HasTimestamp("prometheus", time.Unix(1490802350, 0)))
}
