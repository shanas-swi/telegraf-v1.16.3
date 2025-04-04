package printer

import (
	"fmt"

	"github.com/shanas-swi/telegraf-v1.16.3"
	"github.com/shanas-swi/telegraf-v1.16.3/plugins/processors"
	"github.com/shanas-swi/telegraf-v1.16.3/plugins/serializers"
	"github.com/shanas-swi/telegraf-v1.16.3/plugins/serializers/influx"
)

type Printer struct {
	serializer serializers.Serializer
}

var sampleConfig = `
`

func (p *Printer) SampleConfig() string {
	return sampleConfig
}

func (p *Printer) Description() string {
	return "Print all metrics that pass through this filter."
}

func (p *Printer) Apply(in ...telegraf.Metric) []telegraf.Metric {
	for _, metric := range in {
		octets, err := p.serializer.Serialize(metric)
		if err != nil {
			continue
		}
		fmt.Printf("%s", octets)
	}
	return in
}

func init() {
	processors.Add("printer", func() telegraf.Processor {
		return &Printer{
			serializer: influx.NewSerializer(),
		}
	})
}
