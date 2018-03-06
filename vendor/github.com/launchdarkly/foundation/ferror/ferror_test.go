package ferror

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewJsonUnmarshalError(t *testing.T) {

	object := struct{ test string }{"foo"}

	specs := []struct {
		json               string
		unmarshalTarget    interface{}
		expectedStatusCode int
		expectedError      string
		expectedMessage    string
	}{
		{"}}", &object, http.StatusBadRequest, "invalid_request", "Unable to parse JSON"},
		{"[]", &object, http.StatusBadRequest, "invalid_request", "Unexpected JSON representation"},
		{"{}", nil, http.StatusInternalServerError, "internal_service_error", "Unable to read JSON"},
	}
	for _, spec := range specs {
		jsonErr := json.Unmarshal([]byte(spec.json), spec.unmarshalTarget)
		err := NewJsonUnmarshalError(jsonErr, "reqId")
		if assert.EqualError(t, err, spec.expectedError) {
			assert.Equal(t, spec.expectedMessage, err.Message)
			assert.Equal(t, spec.expectedStatusCode, err.StatusCode)
			assert.Equal(t, "reqId", err.RequestId)
		}
	}
}

func TestNewJsonUnmarshalErrorSuccess(t *testing.T) {
	err := NewJsonUnmarshalError(nil, "")
	assert.NoError(t, err)
}

func TestInternalErrorStackTrace(t *testing.T) {
	err := NewInternalError("Some error", nil, "")
	// This function should end up in the stack trace in the log
	logEntry := err.Log()
	assert.Contains(t, logEntry, "TestInternalErrorStackTrace")
}

func TestFromError(t *testing.T) {
	forbiddenErr := NewForbiddenError("test", nil, "")
	assert.Equal(t, forbiddenErr, FromError(forbiddenErr, ""))

	ferr := FromError(nil, "")
	assert.NoError(t, ferr)

	err := errors.New("failed!")
	ferr = FromError(err, "my-request-id")
	assert.EqualError(t, ferr, "internal_service_error")
	if assert.IsType(t, ferr, NewInternalError("", nil, "")) {
		assert.Equal(t, "my-request-id", ferr.RequestId)
		assert.Equal(t, "failed!", ferr.Underlying)
	}
}
