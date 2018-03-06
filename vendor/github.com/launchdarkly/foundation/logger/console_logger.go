package logger

import (
	"os"
)

func newConsoleLogger(level LogLevel) LevelLogger {
	return LevelLogger{
		writer: os.Stderr,
		level:  level,
	}
}
