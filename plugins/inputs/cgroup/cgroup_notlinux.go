//go:build !linux
// +build !linux

package cgroup

import (
	"github.com/shanas-swi/telegraf-v1.16.3"
)

func (g *CGroup) Gather(acc telegraf.Accumulator) error {
	return nil
}
