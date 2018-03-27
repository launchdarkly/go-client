package ldclient

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type TestUpdateProcessor struct{}

func (u TestUpdateProcessor) Initialized() bool     { return true }
func (u TestUpdateProcessor) Close()                {}
func (u TestUpdateProcessor) Start(chan<- struct{}) {}

func TestOfflineModeAlwaysReturnsDefaultValue(t *testing.T) {
	config := Config{
		BaseUri:       "https://localhost:3000",
		Capacity:      1000,
		FlushInterval: 5 * time.Second,
		Logger:        log.New(os.Stderr, "[LaunchDarkly]", log.LstdFlags),
		Timeout:       1500 * time.Millisecond,
		Stream:        true,
		Offline:       true,
	}
	client, _ := MakeCustomClient("api_key", config, 0)
	defer client.Close()
	client.config.Offline = true
	key := "foo"
	user := User{Key: &key}

	//BoolVariation
	actual, err := client.BoolVariation("featureKey", user, true)
	assert.NoError(t, err)
	assert.True(t, actual)

	//IntVariation
	expectedInt := 100
	actualInt, err := client.IntVariation("featureKey", user, expectedInt)
	assert.NoError(t, err)
	assert.Equal(t, expectedInt, actualInt)

	//Float64Variation
	expectedFloat64 := 100.0
	actualFloat64, err := client.Float64Variation("featureKey", user, expectedFloat64)
	assert.NoError(t, err)
	assert.Equal(t, expectedFloat64, actualFloat64)

	//StringVariation
	expectedString := "expected"
	actualString, err := client.StringVariation("featureKey", user, expectedString)
	assert.NoError(t, err)
	assert.Equal(t, expectedString, actualString)

	//JsonVariation
	expectedJsonString := `{"fieldName":"fieldValue"}`
	expectedJson := json.RawMessage([]byte(expectedJsonString))
	actualJson, err := client.JsonVariation("featureKey", user, expectedJson)
	assert.NoError(t, err)
	assert.Equal(t, string([]byte(expectedJson)), string([]byte(actualJson)))

	client.Close()
}

func TestBoolVariation(t *testing.T) {
	expected := true

	variations := make([]interface{}, 2)
	variations[0] = false
	variations[1] = expected

	client := makeClientWithFeatureFlag(variations)
	defer client.Close()

	userKey := "userKey"
	actual, err := client.BoolVariation("validFeatureKey", User{Key: &userKey}, false)

	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestIntVariation(t *testing.T) {
	expected := float64(100)

	variations := make([]interface{}, 2)
	variations[0] = float64(-1)
	variations[1] = expected

	client := makeClientWithFeatureFlag(variations)
	defer client.Close()

	userKey := "userKey"
	actual, err := client.IntVariation("validFeatureKey", User{Key: &userKey}, 10000)

	assert.NoError(t, err)
	assert.Equal(t, int(expected), actual)
}

func TestFloat64Variation(t *testing.T) {
	expected := 100.01

	variations := make([]interface{}, 2)
	variations[0] = -1.0
	variations[1] = expected

	client := makeClientWithFeatureFlag(variations)
	defer client.Close()

	userKey := "userKey"
	actual, err := client.Float64Variation("validFeatureKey", User{Key: &userKey}, 0.0)

	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestJsonVariation(t *testing.T) {
	expectedJsonString := `{"jsonFieldName2":"fallthroughValue"}`

	var variations []interface{}
	json.Unmarshal([]byte(fmt.Sprintf(`[{"jsonFieldName1" : "jsonFieldValue"},%s]`, expectedJsonString)), &variations)

	client := makeClientWithFeatureFlag(variations)
	defer client.Close()

	userKey := "userKey"
	var actual json.RawMessage
	actual, err := client.JsonVariation("validFeatureKey", User{Key: &userKey}, []byte(`{"default":"default"}`))

	assert.NoError(t, err)
	assert.Equal(t, expectedJsonString, string(actual))
}

func TestSecureModeHash(t *testing.T) {
	expected := "aa747c502a898200f9e4fa21bac68136f886a0e27aec70ba06daf2e2a5cb5597"
	key := "Message"
	config := DefaultConfig
	config.Offline = true

	client, _ := MakeCustomClient("secret", config, 0*time.Second)

	hash := client.SecureModeHash(User{Key: &key})

	assert.Equal(t, expected, hash)
}

// Creates LdClient loaded with one feature flag with key: "validFeatureKey".
// Variations param should have at least 2 items with variations[1] being the expected
// fallthrough value when passing in a valid user
func makeClientWithFeatureFlag(variations []interface{}) *LDClient {
	config := Config{
		BaseUri:               "https://localhost:3000",
		Capacity:              1000,
		FlushInterval:         5 * time.Second,
		Logger:                log.New(os.Stderr, "[LaunchDarkly]", log.LstdFlags),
		Timeout:               1500 * time.Millisecond,
		Stream:                true,
		Offline:               false,
		SendEvents:            false,
		UserKeysFlushInterval: 30 * time.Second,
	}

	client := LDClient{
		sdkKey:          "sdkKey",
		config:          config,
		eventProcessor:  newEventProcessor("sdkKey", config, nil),
		updateProcessor: TestUpdateProcessor{},
		store:           NewInMemoryFeatureStore(nil),
	}
	featureFlag := featureFlagWithVariations(variations)

	client.store.Upsert(Features, &featureFlag)
	return &client
}

func featureFlagWithVariations(variations []interface{}) FeatureFlag {
	fallThroughVariation := 1

	return FeatureFlag{
		Key:         "validFeatureKey",
		Version:     1,
		On:          true,
		Fallthrough: VariationOrRollout{Variation: &fallThroughVariation},
		Variations:  variations,
	}
}
