package dmcache

import (
	"github.com/shanas-swi/telegraf-v1.16.3"
	"github.com/shanas-swi/telegraf-v1.16.3/plugins/inputs"
)

type DMCache struct {
	PerDevice        bool `toml:"per_device"`
	getCurrentStatus func() ([]string, error)
}

var sampleConfig = `
  ## Whether to report per-device stats or not
  per_device = true
`

func (c *DMCache) SampleConfig() string {
	return sampleConfig
}

func (c *DMCache) Description() string {
	return "Provide a native collection for dmsetup based statistics for dm-cache"
}

func init() {
	inputs.Add("dmcache", func() telegraf.Input {
		return &DMCache{
			PerDevice:        true,
			getCurrentStatus: dmSetupStatus,
		}
	})
}
