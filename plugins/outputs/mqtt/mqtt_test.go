package mqtt

import (
	"testing"

	"github.com/shanas-swi/telegraf-v1.16.3/plugins/serializers"
	"github.com/shanas-swi/telegraf-v1.16.3/testutil"

	"github.com/stretchr/testify/require"
)

func TestConnectAndWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	var url = testutil.GetLocalHost() + ":1883"
	s, _ := serializers.NewInfluxSerializer()
	m := &MQTT{
		Servers:    []string{url},
		serializer: s,
	}

	// Verify that we can connect to the MQTT broker
	err := m.Connect()
	require.NoError(t, err)

	// Verify that we can successfully write data to the mqtt broker
	err = m.Write(testutil.MockMetrics())
	require.NoError(t, err)
}
