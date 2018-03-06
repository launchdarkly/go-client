package middleware

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dustin/go-jsonpointer"
	lddog "gopkg.in/launchdarkly/go-client.v3"
	"gopkg.in/mgo.v2/bson"
)

func TestToDogUserHeaderAndBack(t *testing.T) {
	key := "expected-key"
	user := &lddog.User{Key: &key}

	headerVal := ToDogUserHeader(user)

	decodedUser, err := FromDogUserHeader(headerVal)

	assert.NoError(t, err)

	assert.Equal(t, user, decodedUser, "Expected user marshaling / unmarshaling to be idempotent")
}

func TestFromDogUserWithEmptyHeaderReturnsNilUser(t *testing.T) {
	decodedUser, err := FromDogUserHeader("")

	assert.NoError(t, err)
	assert.Nil(t, decodedUser)
}

func TestFromDogUserWithInvalidHeaderReturnsError(t *testing.T) {
	decodedUser, err := FromDogUserHeader("I really doubt this is a valid base64 encoding of anything")

	assert.Error(t, err)
	assert.Nil(t, decodedUser)
}

func TestToDogUserNilReturnsEmptyString(t *testing.T) {
	headerVal := ToDogUserHeader(nil)

	assert.Empty(t, headerVal)
}

func TestAddHeaderWithNilUserDoesNothing(t *testing.T) {
	headers := http.Header{}
	AddDogUserHeader(headers, nil)
	assert.Equal(t, http.Header{}, headers)
}

func TestAddHeaderWithUserAddsExpectedHeader(t *testing.T) {
	headers := http.Header{}
	key := "expected-key"
	user := &lddog.User{Key: &key}

	AddDogUserHeader(headers, user)

	assert.Equal(t, "eyJrZXkiOiJleHBlY3RlZC1rZXkifQ==", headers.Get(DOG_USER_HDR))
}

func TestAuthContext_ErrorContextWithMember(t *testing.T) {
	memberId := bson.NewObjectId()
	accountId := bson.NewObjectId()
	environmentId := bson.NewObjectId()
	projectId := bson.NewObjectId()

	ctx := AuthContext{
		AuthKind:    SESSION_AUTH,
		AccountId:   accountId,
		Environment: environmentId,
		Project:     projectId,
		Member: &MemberInfo{
			Id:       memberId,
			Username: "my-username",
		},
	}
	errCtx := ctx.ErrorContext()
	assert.Equal(t, SESSION_AUTH, errCtx["authKind"])
	assert.Equal(t, accountId.Hex(), errCtx["account.id"])
	assert.Equal(t, memberId.Hex(), errCtx["member.id"])
	assert.Equal(t, memberId.Hex(), jsonpointer.Get(errCtx, "/person/id"))
	assert.Equal(t, "my-username", jsonpointer.Get(errCtx, "/person/username"))
	assert.Equal(t, projectId.Hex(), errCtx["proj.id"])
	assert.Equal(t, environmentId.Hex(), errCtx["environment.id"])
}

func TestAuthContext_ErrorContextWithoutMember(t *testing.T) {
	accountId := bson.NewObjectId()
	environmentId := bson.NewObjectId()
	projectId := bson.NewObjectId()

	ctx := AuthContext{
		AuthKind:    SESSION_AUTH,
		AccountId:   accountId,
		Environment: environmentId,
		Project:     projectId,
	}
	errCtx := ctx.ErrorContext()
	assert.Equal(t, accountId.Hex(), errCtx["account.id"])
	assert.Equal(t, projectId.Hex(), errCtx["proj.id"])
	assert.Equal(t, environmentId.Hex(), errCtx["environment.id"])
}

func TestAuthContext_ErrorContextBlank(t *testing.T) {
	ctx := AuthContext{}
	errCtx := ctx.ErrorContext()
	assert.NotEmpty(t, errCtx)
}

func TestPrivateContext_ErrorContextWithMember(t *testing.T) {
	memberId := bson.NewObjectId()
	accountId := bson.NewObjectId()
	environmentId := bson.NewObjectId()
	projectId := bson.NewObjectId()

	ctx := PrivateAuthContext{
		OnBehalfOfAcct: &accountId,
		OnBehalfOfMbr: &MemberInfo{
			Id:       memberId,
			Username: "my-username",
		},
		Project:     &projectId,
		Environment: &environmentId,
	}

	errCtx := ctx.ErrorContext()
	assert.Equal(t, accountId.Hex(), errCtx["account.id"])
	assert.Equal(t, memberId.Hex(), errCtx["member.id"])
	assert.Equal(t, memberId.Hex(), jsonpointer.Get(errCtx, "/person/id"))
	assert.Equal(t, "my-username", jsonpointer.Get(errCtx, "/person/username"))
	assert.Equal(t, projectId.Hex(), errCtx["proj.id"])
	assert.Equal(t, environmentId.Hex(), errCtx["environment.id"])
}

func TestPrivateContext_ErrorContextWithoutMember(t *testing.T) {
	accountId := bson.NewObjectId()
	environmentId := bson.NewObjectId()
	projectId := bson.NewObjectId()

	ctx := PrivateAuthContext{
		OnBehalfOfAcct: &accountId,
		Project:        &projectId,
		Environment:    &environmentId,
	}

	errCtx := ctx.ErrorContext()
	assert.Equal(t, accountId.Hex(), errCtx["account.id"])
	assert.Equal(t, projectId.Hex(), errCtx["proj.id"])
	assert.Equal(t, environmentId.Hex(), errCtx["environment.id"])
}

func TestPrivateContext_ErrorContextBlank(t *testing.T) {
	ctx := PrivateAuthContext{}
	errCtx := ctx.ErrorContext()
	assert.Equal(t, errCtx, map[string]interface{}{})
}
