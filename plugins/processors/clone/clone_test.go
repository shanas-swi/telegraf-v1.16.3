package clone

import (
	"testing"
	"time"

	"github.com/shanas-swi/telegraf-v1.16.3"
	"github.com/shanas-swi/telegraf-v1.16.3/metric"
	"github.com/stretchr/testify/assert"
)

func createTestMetric() telegraf.Metric {
	metric, _ := metric.New("m1",
		map[string]string{"metric_tag": "from_metric"},
		map[string]interface{}{"value": int64(1)},
		time.Now(),
	)
	return metric
}

func calculateProcessedTags(processor Clone, metric telegraf.Metric) map[string]string {
	processed := processor.Apply(metric)
	return processed[0].Tags()
}

func TestRetainsTags(t *testing.T) {
	processor := Clone{}

	tags := calculateProcessedTags(processor, createTestMetric())

	value, present := tags["metric_tag"]
	assert.True(t, present, "Tag of metric was not present")
	assert.Equal(t, "from_metric", value, "Value of Tag was changed")
}

func TestAddTags(t *testing.T) {
	processor := Clone{Tags: map[string]string{"added_tag": "from_config", "another_tag": ""}}

	tags := calculateProcessedTags(processor, createTestMetric())

	value, present := tags["added_tag"]
	assert.True(t, present, "Additional Tag of metric was not present")
	assert.Equal(t, "from_config", value, "Value of Tag was changed")
	assert.Equal(t, 3, len(tags), "Should have one previous and two added tags.")
}

func TestOverwritesPresentTagValues(t *testing.T) {
	processor := Clone{Tags: map[string]string{"metric_tag": "from_config"}}

	tags := calculateProcessedTags(processor, createTestMetric())

	value, present := tags["metric_tag"]
	assert.True(t, present, "Tag of metric was not present")
	assert.Equal(t, 1, len(tags), "Should only have one tag.")
	assert.Equal(t, "from_config", value, "Value of Tag was not changed")
}

func TestOverridesName(t *testing.T) {
	processor := Clone{NameOverride: "overridden"}

	processed := processor.Apply(createTestMetric())

	assert.Equal(t, "overridden", processed[0].Name(), "Name was not overridden")
	assert.Equal(t, "m1", processed[1].Name(), "Original metric was modified")
}

func TestNamePrefix(t *testing.T) {
	processor := Clone{NamePrefix: "Pre-"}

	processed := processor.Apply(createTestMetric())

	assert.Equal(t, "Pre-m1", processed[0].Name(), "Prefix was not applied")
	assert.Equal(t, "m1", processed[1].Name(), "Original metric was modified")
}

func TestNameSuffix(t *testing.T) {
	processor := Clone{NameSuffix: "-suff"}

	processed := processor.Apply(createTestMetric())

	assert.Equal(t, "m1-suff", processed[0].Name(), "Suffix was not applied")
	assert.Equal(t, "m1", processed[1].Name(), "Original metric was modified")
}
