package outputs

import (
	"github.com/shanas-swi/telegraf-v1.16.3"
)

type Creator func() telegraf.Output

var Outputs = map[string]Creator{}

func Add(name string, creator Creator) {
	Outputs[name] = creator
}
