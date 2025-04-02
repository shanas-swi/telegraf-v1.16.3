package parallel

import "github.com/shanas-swi/telegraf-v1.16.3"

type Parallel interface {
	Enqueue(telegraf.Metric)
	Stop()
}
