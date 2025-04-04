package wireless

import (
	"github.com/shanas-swi/telegraf-v1.16.3"
	"github.com/shanas-swi/telegraf-v1.16.3/plugins/inputs"
)

// Wireless is used to store configuration values.
type Wireless struct {
	HostProc string          `toml:"host_proc"`
	Log      telegraf.Logger `toml:"-"`
}

var sampleConfig = `
  ## Sets 'proc' directory path
  ## If not specified, then default is /proc
  # host_proc = "/proc"
`

// Description returns information about the plugin.
func (w *Wireless) Description() string {
	return "Monitor wifi signal strength and quality"
}

// SampleConfig displays configuration instructions.
func (w *Wireless) SampleConfig() string {
	return sampleConfig
}

func init() {
	inputs.Add("wireless", func() telegraf.Input {
		return &Wireless{}
	})
}
