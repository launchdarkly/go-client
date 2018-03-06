package usertests

import (
	"time"

	ld "github.com/launchdarkly/go-client-private"
)

const (
	flagKey = "usertest-flag"

	eventsCapacity = 5000
	flushInterval  = 1 * time.Second

	stagingBaseUri   = "https://ld-stg.global.ssl.fastly.net"
	stagingEventsUri = "https://events-stg.launchdarkly.com"
	stagingStreamUri = "https://stream-stg.launchdarkly.com"

	localBaseUri   = "http://127.0.0.1"
	localEventsUri = "http://127.0.0.1:4040/api/events"
	localStreamUri = "http://127.0.0.1:5050"
)

func MakeLocalLDClient(sdkKey string) (*ld.LDClient, error) {
	config := makeBasicConfig()

	config.BaseUri = localBaseUri
	config.EventsUri = localEventsUri
	config.StreamUri = localStreamUri

	return ld.MakeCustomClient(sdkKey, config, 10*time.Second)
}

func MakeStagingLDClient(sdkKey string) (*ld.LDClient, error) {
	config := makeBasicConfig()

	config.BaseUri = stagingBaseUri
	config.EventsUri = stagingEventsUri
	config.StreamUri = stagingStreamUri

	return ld.MakeCustomClient(sdkKey, config, 10*time.Second)
}

func MakeProdLDClient(sdkKey string) (*ld.LDClient, error) {
	config := makeBasicConfig()
	return ld.MakeCustomClient(sdkKey, config, 10*time.Second)
}

func makeBasicConfig() ld.Config {
	config := ld.DefaultConfig
	config.Capacity = eventsCapacity
	config.FlushInterval = flushInterval
	return config
}
