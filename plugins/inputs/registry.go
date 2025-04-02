package inputs

import "github.com/shanas-swi/telegraf-v1.16.3"

type Creator func() telegraf.Input

var Inputs = map[string]Creator{}

func Add(name string, creator Creator) {
	Inputs[name] = creator
}
