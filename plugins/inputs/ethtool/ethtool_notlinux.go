//go:build !linux
// +build !linux

package ethtool

import (
	"github.com/shanas-swi/telegraf-v1.16.3"
	"github.com/shanas-swi/telegraf-v1.16.3/plugins/inputs"
)

func (e *Ethtool) Init() error {
	e.Log.Warn("Current platform is not supported")
	return nil
}

func (e *Ethtool) Gather(acc telegraf.Accumulator) error {
	return nil
}

func init() {
	inputs.Add(pluginName, func() telegraf.Input {
		return &Ethtool{}
	})
}
