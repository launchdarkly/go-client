package ldclient

// An update processor that can fall back to using polling
// if the streaming connection fails
type fallbackProcessor struct {
	config Config
	sp     *streamProcessor
	pp     *pollingProcessor
}

func newFallbackProcessor(config Config, sp *streamProcessor, pp *pollingProcessor) *fallbackProcessor {
	return &fallbackProcessor{config, sp, pp}
}

func (fp *fallbackProcessor) initialized() bool {
	return fp.sp.initialized() || fp.pp.initialized()
}

// Close both the streaming and polling processor
func (fp *fallbackProcessor) close() {
	fp.pp.close()
	fp.sp.close()
}

func (fp *fallbackProcessor) start(ch chan<- bool) {
	pollInit := make(chan bool)
	streamInit := make(chan bool)
	fp.sp.start(streamInit)

	fp.pp.shouldPoll = func() bool {
		isConnected := fp.sp.IsConnected()

		if !isConnected {
			fp.config.Logger.Println("Client is not connected to stream. Polling for feature updates")
		}

		return !isConnected
	}

	fp.pp.start(pollInit)

	go func() {
		select {
		case <-streamInit:
			fp.config.Logger.Println("Initialized stream processor in fallback processor")
			ch <- true
		case <-pollInit:
			fp.config.Logger.Println("Initialized polling processor in fallback processor")
			ch <- true
		}
	}()

}
