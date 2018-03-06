package notify

import (
	"testing"

	"github.com/launchdarkly/foundation/logger"
	"github.com/stretchr/testify/assert"
)

func TestLogger(t *testing.T) {
	var loggerSink Sink

	logger, buf := logger.MakeTestLevelLogger()
	loggerSink = NewLogger(logger)

	loggerSink.Output(1, LevelWarn, NewMessageNotification(FakeErrorContexter{12345}, "some warning message"))

	assert.Contains(t, buf.String(), "WARN")
	assert.Contains(t, buf.String(), "some warning message")
	assert.Contains(t, buf.String(), "12345")
}
