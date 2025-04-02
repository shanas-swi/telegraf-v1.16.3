package health

import "github.com/shanas-swi/telegraf-v1.16.3"

type Contains struct {
	Field string `toml:"field"`
}

func (c *Contains) Check(metrics []telegraf.Metric) bool {
	success := false
	for _, m := range metrics {
		ok := m.HasField(c.Field)
		if ok {
			success = true
		}
	}

	return success
}
