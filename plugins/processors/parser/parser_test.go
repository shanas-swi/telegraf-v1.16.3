package parser

import (
	"testing"
	"time"

	"github.com/shanas-swi/telegraf-v1.16.3"
	"github.com/shanas-swi/telegraf-v1.16.3/metric"
	"github.com/shanas-swi/telegraf-v1.16.3/plugins/parsers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// compares metrics without comparing time
func compareMetrics(t *testing.T, expected, actual []telegraf.Metric) {
	assert.Equal(t, len(expected), len(actual))
	for i, metric := range actual {
		require.Equal(t, expected[i].Name(), metric.Name())
		require.Equal(t, expected[i].Fields(), metric.Fields())
		require.Equal(t, expected[i].Tags(), metric.Tags())
	}
}

func Metric(v telegraf.Metric, err error) telegraf.Metric {
	if err != nil {
		panic(err)
	}
	return v
}

func TestApply(t *testing.T) {
	tests := []struct {
		name         string
		parseFields  []string
		config       parsers.Config
		dropOriginal bool
		merge        string
		input        telegraf.Metric
		expected     []telegraf.Metric
	}{
		{
			name:         "parse one field drop original",
			parseFields:  []string{"sample"},
			dropOriginal: true,
			config: parsers.Config{
				DataFormat: "json",
				TagKeys: []string{
					"ts",
					"lvl",
					"msg",
					"method",
				},
			},
			input: Metric(
				metric.New(
					"singleField",
					map[string]string{
						"some": "tag",
					},
					map[string]interface{}{
						"sample": `{"ts":"2018-07-24T19:43:40.275Z","lvl":"info","msg":"http request","method":"POST"}`,
					},
					time.Unix(0, 0))),
			expected: []telegraf.Metric{
				Metric(metric.New(
					"singleField",
					map[string]string{
						"ts":     "2018-07-24T19:43:40.275Z",
						"lvl":    "info",
						"msg":    "http request",
						"method": "POST",
					},
					map[string]interface{}{},
					time.Unix(0, 0))),
			},
		},
		{
			name:         "parse one field with merge",
			parseFields:  []string{"sample"},
			dropOriginal: false,
			merge:        "override",
			config: parsers.Config{
				DataFormat: "json",
				TagKeys: []string{
					"ts",
					"lvl",
					"msg",
					"method",
				},
			},
			input: Metric(
				metric.New(
					"singleField",
					map[string]string{
						"some": "tag",
					},
					map[string]interface{}{
						"sample": `{"ts":"2018-07-24T19:43:40.275Z","lvl":"info","msg":"http request","method":"POST"}`,
					},
					time.Unix(0, 0))),
			expected: []telegraf.Metric{
				Metric(metric.New(
					"singleField",
					map[string]string{
						"some":   "tag",
						"ts":     "2018-07-24T19:43:40.275Z",
						"lvl":    "info",
						"msg":    "http request",
						"method": "POST",
					},
					map[string]interface{}{
						"sample": `{"ts":"2018-07-24T19:43:40.275Z","lvl":"info","msg":"http request","method":"POST"}`,
					},
					time.Unix(0, 0))),
			},
		},
		{
			name:         "parse one field keep",
			parseFields:  []string{"sample"},
			dropOriginal: false,
			config: parsers.Config{
				DataFormat: "json",
				TagKeys: []string{
					"ts",
					"lvl",
					"msg",
					"method",
				},
			},
			input: Metric(
				metric.New(
					"singleField",
					map[string]string{
						"some": "tag",
					},
					map[string]interface{}{
						"sample": `{"ts":"2018-07-24T19:43:40.275Z","lvl":"info","msg":"http request","method":"POST"}`,
					},
					time.Unix(0, 0))),
			expected: []telegraf.Metric{
				Metric(metric.New(
					"singleField",
					map[string]string{
						"some": "tag",
					},
					map[string]interface{}{
						"sample": `{"ts":"2018-07-24T19:43:40.275Z","lvl":"info","msg":"http request","method":"POST"}`,
					},
					time.Unix(0, 0))),
				Metric(metric.New(
					"singleField",
					map[string]string{
						"ts":     "2018-07-24T19:43:40.275Z",
						"lvl":    "info",
						"msg":    "http request",
						"method": "POST",
					},
					map[string]interface{}{},
					time.Unix(0, 0))),
			},
		},
		{
			name:        "parse one field keep with measurement name",
			parseFields: []string{"message"},
			config: parsers.Config{
				DataFormat: "influx",
			},
			dropOriginal: false,
			input: Metric(
				metric.New(
					"influxField",
					map[string]string{},
					map[string]interface{}{
						"message": "deal,computer_name=hosta message=\"stuff\" 1530654676316265790",
					},
					time.Unix(0, 0))),
			expected: []telegraf.Metric{
				Metric(metric.New(
					"influxField",
					map[string]string{},
					map[string]interface{}{
						"message": "deal,computer_name=hosta message=\"stuff\" 1530654676316265790",
					},
					time.Unix(0, 0))),
				Metric(metric.New(
					"deal",
					map[string]string{
						"computer_name": "hosta",
					},
					map[string]interface{}{
						"message": "stuff",
					},
					time.Unix(0, 0))),
			},
		},
		{
			name:         "parse one field override replaces name",
			parseFields:  []string{"message"},
			dropOriginal: false,
			merge:        "override",
			config: parsers.Config{
				DataFormat: "influx",
			},
			input: Metric(
				metric.New(
					"influxField",
					map[string]string{
						"some": "tag",
					},
					map[string]interface{}{
						"message": "deal,computer_name=hosta message=\"stuff\" 1530654676316265790",
					},
					time.Unix(0, 0))),
			expected: []telegraf.Metric{
				Metric(metric.New(
					"deal",
					map[string]string{
						"computer_name": "hosta",
						"some":          "tag",
					},
					map[string]interface{}{
						"message": "stuff",
					},
					time.Unix(0, 0))),
			},
		},
		{
			name:         "parse grok field",
			parseFields:  []string{"grokSample"},
			dropOriginal: true,
			config: parsers.Config{
				DataFormat:   "grok",
				GrokPatterns: []string{"%{COMBINED_LOG_FORMAT}"},
			},
			input: Metric(
				metric.New(
					"success",
					map[string]string{},
					map[string]interface{}{
						"grokSample": "127.0.0.1 - - [11/Dec/2013:00:01:45 -0800] \"GET /xampp/status.php HTTP/1.1\" 200 3891 \"http://cadenza/xampp/navi.php\" \"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.9; rv:25.0) Gecko/20100101 Firefox/25.0\"",
					},
					time.Unix(0, 0))),
			expected: []telegraf.Metric{
				Metric(metric.New(
					"success",
					map[string]string{
						"resp_code": "200",
						"verb":      "GET",
					},
					map[string]interface{}{
						"resp_bytes":   int64(3891),
						"auth":         "-",
						"request":      "/xampp/status.php",
						"referrer":     "http://cadenza/xampp/navi.php",
						"agent":        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.9; rv:25.0) Gecko/20100101 Firefox/25.0",
						"client_ip":    "127.0.0.1",
						"ident":        "-",
						"http_version": float64(1.1),
					},
					time.Unix(0, 0))),
			},
		},
		{
			name:         "parse two fields [replace]",
			parseFields:  []string{"field_1", "field_2"},
			dropOriginal: true,
			config: parsers.Config{
				DataFormat: "json",
				TagKeys:    []string{"lvl", "err"},
			},
			input: Metric(
				metric.New(
					"bigMeasure",
					map[string]string{},
					map[string]interface{}{
						"field_1": `{"lvl":"info","msg":"http request"}`,
						"field_2": `{"err":"fatal","fatal":"security threat"}`,
					},
					time.Unix(0, 0))),
			expected: []telegraf.Metric{
				Metric(metric.New(
					"bigMeasure",
					map[string]string{
						"lvl": "info",
					},
					map[string]interface{}{},
					time.Unix(0, 0))),
				Metric(metric.New(
					"bigMeasure",
					map[string]string{
						"err": "fatal",
					},
					map[string]interface{}{},
					time.Unix(0, 0))),
			},
		},
		{
			name:         "parse two fields [merge]",
			parseFields:  []string{"field_1", "field_2"},
			dropOriginal: false,
			merge:        "override",
			config: parsers.Config{
				DataFormat: "json",
				TagKeys:    []string{"lvl", "msg", "err", "fatal"},
			},
			input: Metric(
				metric.New(
					"bigMeasure",
					map[string]string{},
					map[string]interface{}{
						"field_1": `{"lvl":"info","msg":"http request"}`,
						"field_2": `{"err":"fatal","fatal":"security threat"}`,
					},
					time.Unix(0, 0))),
			expected: []telegraf.Metric{
				Metric(metric.New(
					"bigMeasure",
					map[string]string{
						"lvl":   "info",
						"msg":   "http request",
						"err":   "fatal",
						"fatal": "security threat",
					},
					map[string]interface{}{
						"field_1": `{"lvl":"info","msg":"http request"}`,
						"field_2": `{"err":"fatal","fatal":"security threat"}`,
					},
					time.Unix(0, 0))),
			},
		},
		{
			name:         "parse two fields [keep]",
			parseFields:  []string{"field_1", "field_2"},
			dropOriginal: false,
			config: parsers.Config{
				DataFormat: "json",
				TagKeys:    []string{"lvl", "msg", "err", "fatal"},
			},
			input: Metric(
				metric.New(
					"bigMeasure",
					map[string]string{},
					map[string]interface{}{
						"field_1": `{"lvl":"info","msg":"http request"}`,
						"field_2": `{"err":"fatal","fatal":"security threat"}`,
					},
					time.Unix(0, 0))),
			expected: []telegraf.Metric{
				Metric(metric.New(
					"bigMeasure",
					map[string]string{},
					map[string]interface{}{
						"field_1": `{"lvl":"info","msg":"http request"}`,
						"field_2": `{"err":"fatal","fatal":"security threat"}`,
					},
					time.Unix(0, 0))),
				Metric(metric.New(
					"bigMeasure",
					map[string]string{
						"lvl": "info",
						"msg": "http request",
					},
					map[string]interface{}{},
					time.Unix(0, 0))),
				Metric(metric.New(
					"bigMeasure",
					map[string]string{
						"err":   "fatal",
						"fatal": "security threat",
					},
					map[string]interface{}{},
					time.Unix(0, 0))),
			},
		},
		{
			name:         "Fail to parse one field but parses other [keep]",
			parseFields:  []string{"good", "bad"},
			dropOriginal: false,
			config: parsers.Config{
				DataFormat: "json",
				TagKeys:    []string{"lvl"},
			},
			input: Metric(
				metric.New(
					"success",
					map[string]string{},
					map[string]interface{}{
						"good": `{"lvl":"info"}`,
						"bad":  "why",
					},
					time.Unix(0, 0))),
			expected: []telegraf.Metric{
				Metric(metric.New(
					"success",
					map[string]string{},
					map[string]interface{}{
						"good": `{"lvl":"info"}`,
						"bad":  "why",
					},
					time.Unix(0, 0))),
				Metric(metric.New(
					"success",
					map[string]string{
						"lvl": "info",
					},
					map[string]interface{}{},
					time.Unix(0, 0))),
			},
		},
		{
			name:         "Fail to parse one field but parses other [keep] v2",
			parseFields:  []string{"bad", "good", "ok"},
			dropOriginal: false,
			config: parsers.Config{
				DataFormat: "json",
				TagKeys:    []string{"lvl", "thing"},
			},
			input: Metric(
				metric.New(
					"success",
					map[string]string{},
					map[string]interface{}{
						"bad":  "why",
						"good": `{"lvl":"info"}`,
						"ok":   `{"thing":"thang"}`,
					},
					time.Unix(0, 0))),
			expected: []telegraf.Metric{
				Metric(metric.New(
					"success",
					map[string]string{},
					map[string]interface{}{
						"bad":  "why",
						"good": `{"lvl":"info"}`,
						"ok":   `{"thing":"thang"}`,
					},
					time.Unix(0, 0))),
				Metric(metric.New(
					"success",
					map[string]string{
						"lvl": "info",
					},
					map[string]interface{}{},
					time.Unix(0, 0))),
				Metric(metric.New(
					"success",
					map[string]string{
						"thing": "thang",
					},
					map[string]interface{}{},
					time.Unix(0, 0))),
			},
		},
		{
			name:         "Fail to parse one field but parses other [merge]",
			parseFields:  []string{"good", "bad"},
			dropOriginal: false,
			merge:        "override",
			config: parsers.Config{
				DataFormat: "json",
				TagKeys:    []string{"lvl"},
			},
			input: Metric(
				metric.New(
					"success",
					map[string]string{
						"a": "tag",
					},
					map[string]interface{}{
						"good": `{"lvl":"info"}`,
						"bad":  "why",
					},
					time.Unix(0, 0))),
			expected: []telegraf.Metric{
				Metric(metric.New(
					"success",
					map[string]string{
						"a":   "tag",
						"lvl": "info",
					},
					map[string]interface{}{
						"good": `{"lvl":"info"}`,
						"bad":  "why",
					},
					time.Unix(0, 0))),
			},
		},
		{
			name:         "Fail to parse one field but parses other [replace]",
			parseFields:  []string{"good", "bad"},
			dropOriginal: true,
			config: parsers.Config{
				DataFormat: "json",
				TagKeys:    []string{"lvl"},
			},
			input: Metric(
				metric.New(
					"success",
					map[string]string{
						"thing": "tag",
					},
					map[string]interface{}{
						"good": `{"lvl":"info"}`,
						"bad":  "why",
					},
					time.Unix(0, 0))),
			expected: []telegraf.Metric{
				Metric(metric.New(
					"success",
					map[string]string{
						"lvl": "info",
					},
					map[string]interface{}{},
					time.Unix(0, 0))),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := Parser{
				Config:       tt.config,
				ParseFields:  tt.parseFields,
				DropOriginal: tt.dropOriginal,
				Merge:        tt.merge,
			}

			output := parser.Apply(tt.input)
			t.Logf("Testing: %s", tt.name)
			compareMetrics(t, tt.expected, output)
		})
	}
}

func TestBadApply(t *testing.T) {
	tests := []struct {
		name        string
		parseFields []string
		config      parsers.Config
		input       telegraf.Metric
		expected    []telegraf.Metric
	}{
		{
			name:        "field not found",
			parseFields: []string{"bad_field"},
			config: parsers.Config{
				DataFormat: "json",
			},
			input: Metric(
				metric.New(
					"bad",
					map[string]string{},
					map[string]interface{}{
						"some_field": 5,
					},
					time.Unix(0, 0))),
			expected: []telegraf.Metric{
				Metric(metric.New(
					"bad",
					map[string]string{},
					map[string]interface{}{
						"some_field": 5,
					},
					time.Unix(0, 0))),
			},
		},
		{
			name:        "non string field",
			parseFields: []string{"some_field"},
			config: parsers.Config{
				DataFormat: "json",
			},
			input: Metric(
				metric.New(
					"bad",
					map[string]string{},
					map[string]interface{}{
						"some_field": 5,
					},
					time.Unix(0, 0))),
			expected: []telegraf.Metric{
				Metric(metric.New(
					"bad",
					map[string]string{},
					map[string]interface{}{
						"some_field": 5,
					},
					time.Unix(0, 0))),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := Parser{
				Config:      tt.config,
				ParseFields: tt.parseFields,
			}

			output := parser.Apply(tt.input)

			compareMetrics(t, output, tt.expected)
		})
	}
}

// Benchmarks

func getMetricFields(metric telegraf.Metric) interface{} {
	key := "field3"
	if value, ok := metric.Fields()[key]; ok {
		return value
	}
	return nil
}

func getMetricFieldList(metric telegraf.Metric) interface{} {
	key := "field3"
	fields := metric.FieldList()
	for _, field := range fields {
		if field.Key == key {
			return field.Value
		}
	}
	return nil
}

func BenchmarkFieldListing(b *testing.B) {
	metric := Metric(metric.New(
		"test",
		map[string]string{
			"some": "tag",
		},
		map[string]interface{}{
			"field0": `{"ts":"2018-07-24T19:43:40.275Z","lvl":"info","msg":"http request","method":"POST"}`,
			"field1": `{"ts":"2018-07-24T19:43:40.275Z","lvl":"info","msg":"http request","method":"POST"}`,
			"field2": `{"ts":"2018-07-24T19:43:40.275Z","lvl":"info","msg":"http request","method":"POST"}`,
			"field3": `{"ts":"2018-07-24T19:43:40.275Z","lvl":"info","msg":"http request","method":"POST"}`,
			"field4": `{"ts":"2018-07-24T19:43:40.275Z","lvl":"info","msg":"http request","method":"POST"}`,
			"field5": `{"ts":"2018-07-24T19:43:40.275Z","lvl":"info","msg":"http request","method":"POST"}`,
			"field6": `{"ts":"2018-07-24T19:43:40.275Z","lvl":"info","msg":"http request","method":"POST"}`,
		},
		time.Unix(0, 0)))

	for n := 0; n < b.N; n++ {
		getMetricFieldList(metric)
	}
}

func BenchmarkFields(b *testing.B) {
	metric := Metric(metric.New(
		"test",
		map[string]string{
			"some": "tag",
		},
		map[string]interface{}{
			"field0": `{"ts":"2018-07-24T19:43:40.275Z","lvl":"info","msg":"http request","method":"POST"}`,
			"field1": `{"ts":"2018-07-24T19:43:40.275Z","lvl":"info","msg":"http request","method":"POST"}`,
			"field2": `{"ts":"2018-07-24T19:43:40.275Z","lvl":"info","msg":"http request","method":"POST"}`,
			"field3": `{"ts":"2018-07-24T19:43:40.275Z","lvl":"info","msg":"http request","method":"POST"}`,
			"field4": `{"ts":"2018-07-24T19:43:40.275Z","lvl":"info","msg":"http request","method":"POST"}`,
			"field5": `{"ts":"2018-07-24T19:43:40.275Z","lvl":"info","msg":"http request","method":"POST"}`,
			"field6": `{"ts":"2018-07-24T19:43:40.275Z","lvl":"info","msg":"http request","method":"POST"}`,
		},
		time.Unix(0, 0)))

	for n := 0; n < b.N; n++ {
		getMetricFields(metric)
	}
}
