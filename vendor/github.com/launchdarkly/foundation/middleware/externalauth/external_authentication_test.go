package externalauth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	acc "github.com/launchdarkly/foundation/accounts"
	"github.com/launchdarkly/foundation/ferror"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	mgo "gopkg.in/mgo.v2"
)

func TestAuthTokenWithLeadingSdkPrefix(t *testing.T) {
	apiKey := "sdk-" + uuid.NewRandom().String()
	token, err := FetchAuthToken(makeRequest(apiKey, true))

	assert.Nil(t, err, "Unexpected error fetching auth token")
	assert.Equal(t, apiKey, token, "Token doesn't match expected value")
}

func TestAuthTokenWithLeadingMobPrefix(t *testing.T) {
	apiKey := "mob-" + uuid.NewRandom().String()
	token, err := FetchAuthToken(makeRequest(apiKey, true))

	assert.Nil(t, err, "Unexpected error fetching auth token")
	assert.Equal(t, apiKey, token, "Token doesn't match expected value")
}

func TestAuthTokenWithNoLeadingPrefix(t *testing.T) {
	apiKey := uuid.NewRandom().String()
	token, err := FetchAuthToken(makeRequest(apiKey, true))

	assert.Nil(t, err, "Unexpected error fetching auth token")
	assert.Equal(t, apiKey, token, "Token doesn't match expected value")
}

func TestAuthTokenWithLeadingSdkPrefixAndNoApiKey(t *testing.T) {
	apiKey := "sdk-" + uuid.NewRandom().String()
	token, err := FetchAuthToken(makeRequest(apiKey, false))

	assert.Nil(t, err, "Unexpected error fetching auth token")
	assert.Equal(t, apiKey, token, "Token doesn't match expected value")
}

func TestAuthTokenWithLeadingMobPrefixAndNoApiKey(t *testing.T) {
	apiKey := "mob-" + uuid.NewRandom().String()
	token, err := FetchAuthToken(makeRequest(apiKey, false))

	assert.Nil(t, err, "Unexpected error fetching auth token")
	assert.Equal(t, apiKey, token, "Token doesn't match expected value")
}

func TestAuthTokenWithNoLeadingPrefixAndNoApiKey(t *testing.T) {
	apiKey := uuid.NewRandom().String()
	token, err := FetchAuthToken(makeRequest(apiKey, false))

	assert.Nil(t, err, "Unexpected error fetching auth token")
	assert.Equal(t, apiKey, token, "Token doesn't match expected value")
}

func TestAuthTokenWithLeadingGobbledygook(t *testing.T) {
	apiKey := "api_keyeeee " + uuid.NewRandom().String()
	token, err := FetchAuthToken(makeRequest(apiKey, false))

	assert.NotNil(t, err, "Expected error fetching auth token but got "+token)
}

func TestAuthTokenWithInvalidLeadingPrefixAndNoApiKey(t *testing.T) {
	apiKey := "sdkk-" + uuid.NewRandom().String()
	token, err := FetchAuthToken(makeRequest(apiKey, false))

	assert.NotNil(t, err, "Expected error fetching auth token but got "+token)
}

func TestAuthTokenWithInvalidUuidAndNoApiKey(t *testing.T) {
	apiKey := "sdk-XXX-XXX"
	token, err := FetchAuthToken(makeRequest(apiKey, false))

	assert.NotNil(t, err, "Expected error fetching auth token but got "+token)
}

func TestAuthTokenUnauthorizedHeaders(t *testing.T) {
	var err error
	mongoSession, err = mgo.Dial("mongodb://localhost")
	if !assert.NoError(t, err) {
		return
	}
	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "missing token",
			token: "",
		},
		{
			name:  "supplied token",
			token: "sdk-" + uuid.NewRandom().String(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFinder := mockAccountFinderNotFound()
			w := httptest.NewRecorder()
			serveHttpWithSdkToken(mockWithSdkContext, mockFinder.finder, w, makeRequest(tt.token, false))
			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Equal(t, "max-age=31536000", w.Header().Get("Cache-Control"))
		})
	}
}

func TestErrorCodes(t *testing.T) {
	var err error
	mongoSession, err = mgo.Dial("mongodb://localhost")
	assert.NoError(t, err)
	apiKey := "sdk-" + uuid.NewRandom().String()

	tests := []struct {
		err          *ferror.Error
		expectedCode int
	}{
		{ferror.NewNotFoundError("", nil, ""), http.StatusNotFound},
		{ferror.NewInternalError("", nil, ""), http.StatusInternalServerError},
		{ferror.NewForbiddenError("", nil, ""), http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			w := httptest.NewRecorder()
			mockSdkHandler := func(res http.ResponseWriter, req *http.Request, _ SdkContext) error {
				return tt.err
			}
			serveHttpWithSdkToken(mockSdkHandler, alwaysFindAccount, w, makeRequest(apiKey, false))
			assert.Equal(t, tt.expectedCode, w.Code)
		})
	}
}

var mockWithSdkContext = func(res http.ResponseWriter, req *http.Request, _ SdkContext) error {
	res.Write(nil)
	return nil
}

type mockAccountFinder struct {
	passedTokens []string
	finder       findAccountContextByToken
}

func mockAccountFinderNotFound() mockAccountFinder {
	ret := mockAccountFinder{}
	ret.finder = func(db *mgo.Database, reqId, token string) (acc.AccountListing, *acc.Environment, *ferror.Error) {
		ret.passedTokens = append(ret.passedTokens, token)
		return acc.AccountListing{}, nil, ferror.NewUnauthorizedError(
			"Invalid key",
			nil,
			"1234")
	}
	return ret
}

func makeRequest(token string, addApiKey bool) *http.Request {
	if addApiKey {
		token = "api_key " + token
	}

	req, _ := http.NewRequest("GET", "http://www.google.com", nil)
	req.Header.Add("Authorization", token)

	return req
}

func alwaysFindAccount(db *mgo.Database, reqId, token string) (acc.AccountListing, *acc.Environment, *ferror.Error) {
	return acc.AccountListing{}, &acc.Environment{}, nil
}
