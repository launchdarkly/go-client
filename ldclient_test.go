package ldclient

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"testing"
	"time"
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
	var closer io.Closer
	client, _ := MakeCustomClient("api_key", config, 0)
	closer = client
	defer closer.Close()
	client.config.Offline = true
	key := "foo"
	user := User{Key: &key}

	//Toggle
	expected := true
	actual, err := client.Toggle("featureKey", user, expected)
	if err != nil {
		t.Errorf("Unexpected error in Toggle: %+v", err)
	}
	if actual != expected {
		t.Errorf("Offline mode should return default value, but doesn't")
	}

	//IntVariation
	expectedInt := 100
	actualInt, err := client.IntVariation("featureKey", user, expectedInt)
	if err != nil {
		t.Errorf("Unexpected error in IntVariation: %+v", err)
	}
	if actualInt != expectedInt {
		t.Errorf("Offline mode should return default value: %+v, instead returned: %+v", expectedInt, actualInt)
	}

	//Float64Variation
	expectedFloat64 := 100.0
	actualFloat64, err := client.Float64Variation("featureKey", user, expectedFloat64)
	if err != nil {
		t.Errorf("Unexpected error in Float64Variation: %+v", err)
	}
	if actualFloat64 != expectedFloat64 {
		t.Errorf("Offline mode should return default value, but doesn't")
	}

	//StringVariation
	expectedString := "expected"
	actualString, err := client.StringVariation("featureKey", user, expectedString)
	if err != nil {
		t.Errorf("Unexpected error in StringVariation: %+v", err)
	}
	if actualString != expectedString {
		t.Errorf("Offline mode should return default value, but doesn't")
	}

	//JsonVariation
	expectedJsonString := `{"fieldName":"fieldValue"}`
	expectedJson := json.RawMessage([]byte(expectedJsonString))
	actualJson, err := client.JsonVariation("featureKey", user, expectedJson)
	if err != nil {
		t.Errorf("Unexpected error in JsonVariation: %+v", err)
	}
	if string([]byte(actualJson)) != string([]byte(expectedJson)) {
		t.Errorf("Offline mode should return default value (%+v), instead got: %+v", expectedJson, actualJson)
	}

	client.Close()
}

func TestToggle(t *testing.T) {
	expected := true

	variations := make([]interface{}, 2)
	variations[0] = false
	variations[1] = expected

	client := makeClientWithFeatureFlag(variations)
	defer client.Close()

	userKey := "userKey"
	actual, err := client.Toggle("validFeatureKey", User{Key: &userKey}, false)

	if err != nil {
		t.Errorf("Unexpected error when calling Toggle: %+v", err)
	}
	if actual != expected {
		t.Errorf("Got unexpected result when calling Toggle: %+v but expected: %+v", actual, expected)
	}
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

	if err != nil {
		t.Errorf("Unexpected error when calling IntVariation: %+v", err)
	}
	if actual != int(expected) {
		t.Errorf("Got unexpected result when calling IntVariation: %+v but expected: %+v", actual, expected)
	}
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

	if err != nil {
		t.Errorf("Unexpected error when calling Float64Variation: %+v", err)
	}
	if actual != expected {
		t.Errorf("Got unexpected result when calling Float64Variation: %+v but expected: %+v", actual, expected)
	}
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

	if err != nil {
		t.Errorf("Unexpected error when calling JsonVariation: %+v", err)
	}
	if string(actual) != expectedJsonString {
		t.Errorf("Got unexpected result when calling JsonVariation: %+v but expected: %+v", string(actual), expectedJsonString)
	}
}

func TestSecureModeHash(t *testing.T) {
	expected := "aa747c502a898200f9e4fa21bac68136f886a0e27aec70ba06daf2e2a5cb5597"
	key := "Message"
	config := DefaultConfig
	config.Offline = true

	client, _ := MakeCustomClient("secret", config, 0*time.Second)

	hash := client.SecureModeHash(User{Key: &key})

	if hash != expected {
		t.Errorf("Got unexpected result when calling SecureModeHash: %s but expected %s", hash, expected)
	}
}

// Creates LdClient loaded with one feature flag with key: "validFeatureKey".
// Variations param should have at least 2 items with variations[1] being the expected
// fallthrough value when passing in a valid user
func makeClientWithFeatureFlag(variations []interface{}) *LDClient {
	config := Config{
		BaseUri:       "https://localhost:3000",
		Capacity:      1000,
		FlushInterval: 5 * time.Second,
		Logger:        log.New(os.Stderr, "[LaunchDarkly]", log.LstdFlags),
		Timeout:       1500 * time.Millisecond,
		Stream:        true,
		Offline:       false,
		SendEvents:    false,
	}

	client := LDClient{
		sdkKey:          "sdkKey",
		config:          config,
		eventProcessor:  newEventProcessor("sdkKey", config),
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
