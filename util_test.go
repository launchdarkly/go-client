package ldclient

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseDateZero(t *testing.T) {
	expectedTimeStamp := "1970-01-01T00:00:00Z"
	expected, _ := time.Parse(time.RFC3339Nano, expectedTimeStamp)
	testParseTime(t, expected, expected)
	testParseTime(t, 0, expected)
	testParseTime(t, 0.0, expected)
	testParseTime(t, expectedTimeStamp, expected)
}

func TestParseUtcTimestamp(t *testing.T) {
	expectedTimeStamp := "2016-04-16T22:57:31.684Z"
	expected, _ := time.Parse(time.RFC3339Nano, expectedTimeStamp)
	testParseTime(t, expected, expected)
	testParseTime(t, 1460847451684, expected)
	testParseTime(t, 1460847451684.0, expected)
	testParseTime(t, expectedTimeStamp, expected)
}

func TestParseTimezone(t *testing.T) {
	expectedTimeStamp := "2016-04-16T17:09:12.759-07:00"
	expected, _ := time.Parse(time.RFC3339Nano, expectedTimeStamp)
	testParseTime(t, expected, expected)
	testParseTime(t, 1460851752759, expected)
	testParseTime(t, 1460851752759.0, expected)
	testParseTime(t, expectedTimeStamp, expected)
}

func TestParseTimezoneNoMillis(t *testing.T) {
	expectedTimeStamp := "2016-04-16T17:09:12-07:00"
	expected, _ := time.Parse(time.RFC3339Nano, expectedTimeStamp)
	testParseTime(t, expected, expected)
	testParseTime(t, 1460851752000, expected)
	testParseTime(t, 1460851752000.0, expected)
	testParseTime(t, expectedTimeStamp, expected)
}

func TestParseTimestampBeforeEpoch(t *testing.T) {
	expectedTimeStamp := "1969-12-31T23:57:56.544-00:00"
	expected, _ := time.Parse(time.RFC3339Nano, expectedTimeStamp)
	testParseTime(t, expected, expected)
	testParseTime(t, -123456, expected)
	testParseTime(t, -123456.0, expected)
	testParseTime(t, expectedTimeStamp, expected)
}

func testParseTime(t *testing.T, input interface{}, expected time.Time) {
	actual := ParseTime(input)
	if actual == nil {
		t.Errorf("Got unexpected nil result when parsing: %+v", input)
		return
	}

	if !actual.Equal(expected) {
		t.Errorf("Got unexpected result: %+v Expected: %+v when parsing: %+v", actual, expected, input)
	}
}

func TestParseFloat64(t *testing.T) {
	testParseFloat64(t, 123, 123.0)
	testParseFloat64(t, 123.0, 123.0)

	testParseFloat64(t, -20, -20.0)
	testParseFloat64(t, -20.0, -20.0)

	testParseFloat64(t, 4e-2, .04)
	testParseFloat64(t, 4.0e-2, .04)

	testParseFloat64(t, "1000", 1000)
	testParseFloat64(t, "1.0", 1.0)
}

func testParseFloat64(t *testing.T, input interface{}, expected float64) {
	actual := ParseFloat64(input)
	if actual == nil {
		t.Errorf("Got unexpected nil result. Expected: %+v when parsing: %+v", expected, input)
		return
	}
	if *actual != expected {
		t.Errorf("Got unexpected result: %+v Expected: %+v when parsing: %+v", *actual, expected, input)
	}
}

func TestParseBadNumber(t *testing.T) {
	testParseBadNumber(t, nil)
	testParseBadNumber(t, "")
	testParseBadNumber(t, "a")
	testParseBadNumber(t, "-")
	testParseBadNumber(t, nil)
	testParseBadNumber(t, "1,000.0")
	testParseBadNumber(t, "$1000")
}

func testParseBadNumber(t *testing.T, input interface{}) {
	actual := ParseFloat64(input)
	if actual != nil {
		t.Errorf("Expected nil result, but instead got: %+v when parsing: %+v", actual, input)
	}
}

func TestToJsonRawMessage(t *testing.T) {
	expectedJsonString := `{"FieldName":"fieldValue","NumericField":1.02}`

	type expected struct {
		FieldName    string  `json:"FieldName"`
		NumericField float64 `json:"NumericField"`
	}
	expectedStruct := expected{FieldName: "fieldValue", NumericField: 1.02}

	inputs := [3]interface{}{
		json.RawMessage([]byte(expectedJsonString)),
		[]byte(expectedJsonString),
		expectedStruct,
	}

	for _, input := range inputs {
		actual, err := ToJsonRawMessage(input)
		if err != nil {
			t.Errorf("Got unexpected error: %+v", err.Error())
		}
		if expectedJsonString != string(actual) {
			t.Errorf("Got unexpected result: %+v but was expecting: %+v", string(actual), expectedJsonString)
		}
	}
}
