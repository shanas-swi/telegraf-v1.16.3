package hddtemp

import (
	"testing"

	hddtemp "github.com/shanas-swi/telegraf-v1.16.3/plugins/inputs/hddtemp/go-hddtemp"
	"github.com/shanas-swi/telegraf-v1.16.3/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFetcher struct {
}

func (h *mockFetcher) Fetch(address string) ([]hddtemp.Disk, error) {
	return []hddtemp.Disk{
		{
			DeviceName:  "Disk1",
			Model:       "Model1",
			Temperature: 13,
			Unit:        "C",
		},
		{
			DeviceName:  "Disk2",
			Model:       "Model2",
			Temperature: 14,
			Unit:        "C",
		},
	}, nil

}
func newMockFetcher() *mockFetcher {
	return &mockFetcher{}
}

func TestFetch(t *testing.T) {
	hddtemp := &HDDTemp{
		fetcher: newMockFetcher(),
		Address: "localhost",
		Devices: []string{"*"},
	}

	acc := &testutil.Accumulator{}
	err := hddtemp.Gather(acc)

	require.NoError(t, err)
	assert.Equal(t, acc.NFields(), 2)

	var tests = []struct {
		fields map[string]interface{}
		tags   map[string]string
	}{
		{
			map[string]interface{}{
				"temperature": int32(13),
			},
			map[string]string{
				"device": "Disk1",
				"model":  "Model1",
				"unit":   "C",
				"status": "",
				"source": "localhost",
			},
		},
		{
			map[string]interface{}{
				"temperature": int32(14),
			},
			map[string]string{
				"device": "Disk2",
				"model":  "Model2",
				"unit":   "C",
				"status": "",
				"source": "localhost",
			},
		},
	}

	for _, test := range tests {
		acc.AssertContainsTaggedFields(t, "hddtemp", test.fields, test.tags)
	}

}
