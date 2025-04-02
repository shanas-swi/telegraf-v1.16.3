//go:build !linux
// +build !linux

package dmcache

import (
	"github.com/shanas-swi/telegraf-v1.16.3"
)

func (c *DMCache) Gather(acc telegraf.Accumulator) error {
	return nil
}

func dmSetupStatus() ([]string, error) {
	return []string{}, nil
}
