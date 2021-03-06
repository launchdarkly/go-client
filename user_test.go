package ldclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewUser(t *testing.T) {
	user := NewUser("some-key")
	k, _ := user.valueOf("key")
	assert.Equal(t, "some-key", k)
}

func TestNewAnonymousUser(t *testing.T) {
	user := NewAnonymousUser("some-key")

	k, _ := user.valueOf("key")
	assert.Equal(t, "some-key", k)

	anonymous, _ := user.valueOf("anonymous")
	assert.Equal(t, true, anonymous)
}
