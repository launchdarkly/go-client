package dogfood

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	lddog "gopkg.in/launchdarkly/go-client.v3"
)

type ExampleJSON struct {
	Field1 string `json:"field1"`
	Field2 string `json:"field2"`
}

func TestJsonVariationWithOfflineClientReturnsDefault(t *testing.T) {
	key := "key"
	config := lddog.DefaultConfig
	config.Offline = true
	client, err := lddog.MakeCustomClient("no-key", config, 5*time.Second)
	assert.NoError(t, err)

	defaultJSON, err := json.Marshal(ExampleJSON{"value1", "value2"})
	assert.NoError(t, err)

	variation, err := JsonVariation("test", lddog.User{Key: &key}, defaultJSON)

	var result ExampleJSON
	json.Unmarshal(variation, &result)

	assert.Equal(t, "value1", result.Field1)
	assert.Equal(t, "value2", result.Field2)

	client.Close()
	client = nil
}

func TestJsonVariationFromConfigFile(t *testing.T) {

	flags := `{
	"foo" : "b",
	"bar" : {"field1": "value1", "field2": "value2"}
}`
	key := "key"

	err := json.Unmarshal([]byte(flags), &configFlags)
	assert.NoError(t, err)

	zeroJSON, err := json.Marshal(ExampleJSON{})
	assert.NoError(t, err)

	variation, err := JsonVariation("bar", lddog.User{Key: &key}, zeroJSON)

	var result ExampleJSON
	json.Unmarshal(variation, &result)

	assert.Equal(t, "value1", result.Field1)
	assert.Equal(t, "value2", result.Field2)
}

func TestIntVariationFromConfigFile(t *testing.T) {

	flags := `{
	"foo" : "b",
	"bar" : 123,
	"bar-float": 123.4
}`
	key := "key"

	err := json.Unmarshal([]byte(flags), &configFlags)
	assert.NoError(t, err)

	variation, err := IntVariation("bar", lddog.User{Key: &key}, 456)
	assert.NoError(t, err)
	assert.Equal(t, variation, 123)

	variation, err = IntVariation("bar-float", lddog.User{Key: &key}, 456)
	assert.EqualError(t, err, "Dogfood flag bar-float has a non-int (float64) value in flag configuration file")
	assert.Equal(t, variation, 456)

	variation, err = IntVariation("missing", lddog.User{Key: &key}, 456)
	assert.EqualError(t, err, "Dogfood flag missing does not exist in flag configuration file")
	assert.Equal(t, variation, 456)
}
