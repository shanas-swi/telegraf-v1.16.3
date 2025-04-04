package trig

import (
	"math"
	"testing"

	"github.com/shanas-swi/telegraf-v1.16.3/testutil"
)

func TestTrig(t *testing.T) {
	s := &Trig{
		Amplitude: 10.0,
	}

	for i := 0.0; i < 10.0; i++ {

		var acc testutil.Accumulator

		sine := math.Sin((i*math.Pi)/5.0) * s.Amplitude
		cosine := math.Cos((i*math.Pi)/5.0) * s.Amplitude

		s.Gather(&acc)

		fields := make(map[string]interface{})
		fields["sine"] = sine
		fields["cosine"] = cosine

		acc.AssertContainsFields(t, "trig", fields)
	}
}
