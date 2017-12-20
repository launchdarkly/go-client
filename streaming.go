package ldclient

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	es "github.com/launchdarkly/eventsource"
)

const (
	putEvent           = "put"
	patchEvent         = "patch"
	deleteEvent        = "delete"
	indirectPatchEvent = "indirect/patch"
)

type streamProcessor struct {
	store              FeatureStore
	requestor          *requestor
	stream             *es.Stream
	config             Config
	sdkKey             string
	setInitializedOnce sync.Once
	isInitialized      bool
	halt               chan struct{}
}

type featurePatchData struct {
	Path string      `json:"path"`
	Data FeatureFlag `json:"data"`
}

type featureDeleteData struct {
	Path    string `json:"path"`
	Version int    `json:"version"`
}

func (sp *streamProcessor) initialized() bool {
	return sp.isInitialized
}

func (sp *streamProcessor) start(closeWhenReady chan<- struct{}) {
	sp.config.Logger.Printf("Starting LaunchDarkly streaming connection")
	go sp.subscribe(closeWhenReady)
}

func (sp *streamProcessor) events(closeWhenReady chan<- struct{}) {
	for {
		select {
		case event, ok := <-sp.stream.Events:
			if !ok {
				sp.config.Logger.Printf("Event stream closed.")
				return
			}
			switch event.Event() {
			case putEvent:
				var features map[string]*FeatureFlag
				if err := json.Unmarshal([]byte(event.Data()), &features); err != nil {
					sp.config.Logger.Printf("Unexpected error unmarshalling feature json: %+v", err)
				} else {
					sp.store.Init(features)
					sp.setInitializedOnce.Do(func() {
						sp.config.Logger.Printf("Started LaunchDarkly streaming client")
						sp.isInitialized = true
						close(closeWhenReady)
					})
				}
			case patchEvent:
				var patch featurePatchData
				if err := json.Unmarshal([]byte(event.Data()), &patch); err != nil {
					sp.config.Logger.Printf("Unexpected error unmarshalling feature patch json: %+v", err)
				} else {
					key := strings.TrimLeft(patch.Path, "/")
					sp.store.Upsert(key, patch.Data)
				}
			case indirectPatchEvent:
				key := event.Data()
				if feature, err := sp.requestor.requestFlag(key); err != nil {
					sp.config.Logger.Printf("Unexpected error requesting feature: %+v", err)
				} else {
					sp.store.Upsert(key, *feature)
				}
			case deleteEvent:
				var data featureDeleteData
				if err := json.Unmarshal([]byte(event.Data()), &data); err != nil {
					sp.config.Logger.Printf("Unexpected error unmarshalling feature delete json: %+v", err)
				} else {
					key := strings.TrimLeft(data.Path, "/")
					sp.store.Delete(key, data.Version)

				}
			}
		case <-sp.halt:
			return
		}
	}
}

func newStreamProcessor(sdkKey string, config Config, requestor *requestor) updateProcessor {
	sp := &streamProcessor{
		store:     config.FeatureStore,
		config:    config,
		sdkKey:    sdkKey,
		requestor: requestor,
		halt:      make(chan struct{}),
	}

	return sp
}

func (sp *streamProcessor) subscribe(closeWhenReady chan<- struct{}) {
	for {
		req, _ := http.NewRequest("GET", sp.config.StreamUri+"/flags", nil)
		req.Header.Add("Authorization", sp.sdkKey)
		req.Header.Add("User-Agent", "GoClient/"+Version)
		sp.config.Logger.Printf("Connecting to LaunchDarkly stream using URL: %s", req.URL.String())

		if stream, err := es.SubscribeWithRequest("", req); err != nil {
			sp.config.Logger.Printf("Error subscribing to stream: %+v using URL: %s", err, req.URL.String())

			// Halt immediately if we've been closed already
			select {
			case <-sp.halt:
				return
			default:
				time.Sleep(2 * time.Second)
			}

		} else {
			sp.stream = stream
			sp.stream.Logger = sp.config.Logger

			go sp.events(closeWhenReady)
			go sp.errors()

			return
		}
	}
}

func (sp *streamProcessor) errors() {
	for {
		select {
		case err, ok := <-sp.stream.Errors:
			if !ok {
				sp.config.Logger.Printf("Event error stream closed.")
				return
			}
			if err != io.EOF {
				sp.config.Logger.Printf("Error encountered processing stream: %+v", err)
			}
		case <-sp.halt:
			return
		}
	}
}

func (sp *streamProcessor) close() {
	sp.config.Logger.Printf("Closing event stream.")
	// TODO: enable this when we trust stream.Close() never to panic (see https://github.com/donovanhide/eventsource/pull/33)
	// sp.stream.Close()
	close(sp.halt)
}
