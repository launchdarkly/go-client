package logger

import (
	"github.com/launchdarkly/foundation/statuschecks"
)

type testLDLogger struct {
	Logger LDMessageLogger
}

func (t testLDLogger) Debug() LDMessageLogger {
	return t.Logger
}

func (t testLDLogger) Info() LDMessageLogger {
	return t.Logger
}

func (t testLDLogger) Warn() LDMessageLogger {
	return t.Logger
}

func (t testLDLogger) Error() LDMessageLogger {
	return t.Logger
}

func (testLDLogger) Status() (s statuschecks.ServiceStatus) {
	return
}

func (testLDLogger) IsDebug() bool {
	return false
}
