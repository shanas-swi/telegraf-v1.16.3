package temp

import (
	"fmt"
	"strings"

	"github.com/shanas-swi/telegraf-v1.16.3"
	"github.com/shanas-swi/telegraf-v1.16.3/plugins/inputs"
	"github.com/shanas-swi/telegraf-v1.16.3/plugins/inputs/system"
)

type Temperature struct {
	ps system.PS
}

func (t *Temperature) Description() string {
	return "Read metrics about temperature"
}

const sampleConfig = ""

func (t *Temperature) SampleConfig() string {
	return sampleConfig
}

func (t *Temperature) Gather(acc telegraf.Accumulator) error {
	temps, err := t.ps.Temperature()
	if err != nil {
		if strings.Contains(err.Error(), "not implemented yet") {
			return fmt.Errorf("plugin is not supported on this platform: %v", err)
		}
		return fmt.Errorf("error getting temperatures info: %s", err)
	}
	for _, temp := range temps {
		tags := map[string]string{
			"sensor": temp.SensorKey,
		}
		fields := map[string]interface{}{
			"temp": temp.Temperature,
		}
		acc.AddFields("temp", fields, tags)
	}
	return nil
}

func init() {
	inputs.Add("temp", func() telegraf.Input {
		return &Temperature{ps: system.NewSystemPS()}
	})
}
