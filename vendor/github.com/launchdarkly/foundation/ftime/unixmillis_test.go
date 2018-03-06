package ftime

import (
	"encoding/json"
	"testing"
	"time"
)

func TestConvertingFromUnixMillis(t *testing.T) {
	fixed, err := time.Parse(time.UnixDate, "Mon Jan 2 15:04:05 MST 2006")

	if err != nil {
		t.Errorf("Unexpected error parsing date: %+v", err)
	}

	um := ToUnixMillis(fixed)
	converted := um.ToTime()

	if !fixed.Equal(converted) {
		t.Errorf("Expected time to be %v, got %v", fixed.UnixNano(), converted.UnixNano())
	}
}

func TestConvertingToUnixMillis(t *testing.T) {
	um := UnixMillis(1442886337000)
	d, err := time.Parse(time.RFC1123, "Tue, 22 Sep 2015 01:45:37 GMT")

	if err != nil {
		t.Errorf("Unexpected error parsing date: %+v", err)
	}

	dum := ToUnixMillis(d)

	if !um.Equals(dum) {
		t.Errorf("Expected time in unix millis to be %v, got %v", um, dum)
	}
}

func TestParsingUnixMillis(t *testing.T) {
	um := UnixMillis(1442886337000)
	pum, err := ParseUnixMillis("1442886337000")

	if err != nil {
		t.Errorf("Unexpected error parsing unix millis: %+v", err)
	}

	if !um.Equals(pum) {
		t.Errorf("Expected time in unix millis to be %v, got %v", um, pum)
	}
}

func TestEquality(t *testing.T) {
	um := UnixMillis(1442886337000)
	um2 := UnixMillis(1442886337001)

	if um.Equals(um2) {
		t.Errorf("%v is not equal to %v, but Equals implementation reports otherwise", um, um2)
	}

}

func TestToString(t *testing.T) {
	um := UnixMillis(1442886337000)
	str := um.ToString()

	if str != "1442886337000" {
		t.Errorf("Expected string value of 1442886337000, but got %s", str)
	}
}

func TestUnmarshalJson(t *testing.T) {
	type foo struct {
		CreationDate UnixMillis `json:"creationDate"`
	}
	var f foo
	s := `{"user":{"key":"gid--1"},"value":true,"kind":"feature","creationDate":1.44685710254e+12,"key":"slack_import"}`
	err := json.Unmarshal([]byte(s), &f)

	if err != nil {
		t.Errorf("Error unmarshalling json: %+v", err)
	}

	if !UnixMillis(1446857102540).Equals(f.CreationDate) {
		t.Errorf("%v is not equal to 1446857102540", f.CreationDate)
	}

}
