package shim

import "github.com/shanas-swi/telegraf-v1.16.3"

// inputShim implements the MetricMaker interface.
type inputShim struct {
	Input telegraf.Input
}

func (i inputShim) LogName() string {
	return ""
}

func (i inputShim) MakeMetric(m telegraf.Metric) telegraf.Metric {
	return m // don't need to do anything to it.
}

func (i inputShim) Log() telegraf.Logger {
	return nil
}
