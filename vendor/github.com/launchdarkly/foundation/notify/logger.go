package notify

import (
	"fmt"

	"github.com/launchdarkly/foundation/logger"
)

type Logger struct {
	logger   logger.LDLogger
	logStack bool
}

func NewDefaultLogger() Logger {
	return Logger{logger.Logger, false}
}

func NewDefaultLoggerWithStack() Logger {
	return Logger{logger.Logger, true}
}

func NewLogger(logger logger.LDLogger) Logger {
	return Logger{logger, false}
}

func NewLoggerWithStack(logger logger.LDLogger) Logger {
	return Logger{logger, true}
}

func (l Logger) Output(calldepth int, level Level, notification Notification) {
	var msgLogger logger.LDMessageLogger
	switch level {
	case LevelError:
		msgLogger = l.logger.Error()
	case LevelWarn:
		msgLogger = l.logger.Warn()
	case LevelInfo:
		msgLogger = l.logger.Info()
	case LevelDebug:
		msgLogger = l.logger.Debug()
	default:
		msgLogger = l.logger.Error()
	}

	logEntry := fmt.Sprintf("%s %v\n", notification.String(), notification.ErrorContext())
	if l.logStack {
		for _, frame := range notification.Stack() {
			logEntry += frame.String()
		}
	}

	// Add 2 to calldepth to account for the logger.
	msgLogger.Output(calldepth+2, logEntry)
}
