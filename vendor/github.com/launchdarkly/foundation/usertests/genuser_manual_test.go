package usertests

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// This will pump 40K random users into your environment.
func TestManualGenUser(t *testing.T) {
	t.SkipNow()

	//client, err := MakeStagingLDClient("your staging sdk key")
	client, err := MakeProdLDClient(" your prod sdk key")
	assert.NoError(t, err)

	for i := 0; i < 100; i++ {
		for i := 0; i < 100; i++ {
			client.BoolVariation(flagKey, GenUser(), false)
			client.BoolVariation(flagKey, GenUserUUIDKey(), false)
			client.BoolVariation(flagKey, GenUserNoName(), false)
			client.BoolVariation(flagKey, GenUserUUIDKeyNoName(), false)
		}
		client.Flush()
	}

	client.Close()
}
