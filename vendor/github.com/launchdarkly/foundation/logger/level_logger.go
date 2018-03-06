package logger

import (
	"bytes"
	"fmt"
	"io"
	"log"

	fs "github.com/launchdarkly/foundation/statuschecks"
)

type LevelLogger struct {
	writer io.Writer
	level  LogLevel
}

func (l LevelLogger) Debug() LDMessageLogger {
	return l.getMessageLoggerForLevel(LevelDebug)
}

func (l LevelLogger) Info() LDMessageLogger {
	return l.getMessageLoggerForLevel(LevelInfo)
}

func (l LevelLogger) Warn() LDMessageLogger {
	return l.getMessageLoggerForLevel(LevelWarn)
}

func (l LevelLogger) Error() LDMessageLogger {
	return l.getMessageLoggerForLevel(LevelError)
}

func (l LevelLogger) getMessageLoggerForLevel(level LogLevel) LDMessageLogger {
	if l.level <= level {
		prefix := fmt.Sprintf("%s: ", level.String())
		return log.New(l.writer, prefix, log.Ldate|log.Ltime|log.Lshortfile)
	} else {
		return NoOpLogger
	}
}

func (l LevelLogger) Status() fs.ServiceStatus {
	return fs.HealthyService()
}

func (l LevelLogger) IsDebug() bool {
	return l.level <= LevelDebug
}

func MakeTestLevelLogger() (LevelLogger, *bytes.Buffer) {
	var buf bytes.Buffer
	return LevelLogger{
		writer: &buf,
		level:  LevelDebug,
	}, &buf
}
