package logger

import (
	"fmt"

	fs "github.com/launchdarkly/foundation/statuschecks"
)

type compositeLogger struct {
	chain []LDLogger
}
type compositeMessageLogger struct {
	chain []LDMessageLogger
}

func (l compositeLogger) Debug() LDMessageLogger {
	messageLoggers := make([]LDMessageLogger, 0)
	for _, logger := range l.chain {
		messageLoggers = append(messageLoggers, logger.Debug())
	}
	return compositeMessageLogger{messageLoggers}
}

func (l compositeLogger) Info() LDMessageLogger {
	messageLoggers := make([]LDMessageLogger, 0)
	for _, logger := range l.chain {
		messageLoggers = append(messageLoggers, logger.Info())
	}
	return compositeMessageLogger{messageLoggers}
}

func (l compositeLogger) Warn() LDMessageLogger {
	messageLoggers := make([]LDMessageLogger, 0)
	for _, logger := range l.chain {
		messageLoggers = append(messageLoggers, logger.Warn())
	}
	return compositeMessageLogger{messageLoggers}
}

func (l compositeLogger) Error() LDMessageLogger {
	messageLoggers := make([]LDMessageLogger, 0)
	for _, logger := range l.chain {
		messageLoggers = append(messageLoggers, logger.Error())
	}
	return compositeMessageLogger{messageLoggers}
}

func (l compositeLogger) Status() fs.ServiceStatus {
	for _, value := range l.chain {
		if !value.Status().IsHealthy() {
			return fs.DegradedService()
		}
	}
	return fs.HealthyService()
}

func (l compositeLogger) IsDebug() bool {
	for _, value := range l.chain {
		if value.IsDebug() {
			return true
		}
	}
	return false
}

func (ls compositeMessageLogger) Println(v ...interface{}) {
	ls.Output(2, fmt.Sprintln(v...))
}

func (ls compositeMessageLogger) Printf(format string, v ...interface{}) {
	ls.Output(2, fmt.Sprintf(format, v...))
}

func (ls compositeMessageLogger) Output(calldepth int, s string) error {
	var lastError error
	for _, l := range ls.chain {
		var err error
		err = l.Output(calldepth+2, s)
		if err != nil {
			lastError = err
		}
	}
	return lastError
}
