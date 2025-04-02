package aggregators

import "github.com/shanas-swi/telegraf-v1.16.3"

type Creator func() telegraf.Aggregator

var Aggregators = map[string]Creator{}

func Add(name string, creator Creator) {
	Aggregators[name] = creator
}
