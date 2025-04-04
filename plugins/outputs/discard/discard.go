package discard

import (
	"github.com/shanas-swi/telegraf-v1.16.3"
	"github.com/shanas-swi/telegraf-v1.16.3/plugins/outputs"
)

type Discard struct{}

func (d *Discard) Connect() error       { return nil }
func (d *Discard) Close() error         { return nil }
func (d *Discard) SampleConfig() string { return "" }
func (d *Discard) Description() string  { return "Send metrics to nowhere at all" }
func (d *Discard) Write(metrics []telegraf.Metric) error {
	return nil
}

func init() {
	outputs.Add("discard", func() telegraf.Output { return &Discard{} })
}
