package logger

import (
	"fmt"

	fs "github.com/launchdarkly/foundation/statuschecks"
)

type LDPrefixLogger struct {
	prefix     string
	baseLogger LDLogger
}

type LDPrefixMessageLogger struct {
	messageLogger LDMessageLogger
	prefixLogger  *LDPrefixLogger
}

func (l LDPrefixMessageLogger) Output(calldepth int, s string) error {
	return l.messageLogger.Output(calldepth+2, fmt.Sprintf("%s%s", l.prefixLogger.prefix, s))
}

func (l LDPrefixMessageLogger) Println(v ...interface{}) {
	l.Output(2, fmt.Sprintln(v...))
}

func (l LDPrefixMessageLogger) Printf(format string, v ...interface{}) {
	l.Output(2, fmt.Sprintf(format, v...))
}

func (l LDPrefixLogger) prefixMessageLogger(logger LDMessageLogger) LDPrefixMessageLogger {
	return LDPrefixMessageLogger{messageLogger: logger, prefixLogger: &l}
}

func (l LDPrefixLogger) Debug() LDMessageLogger {
	return l.prefixMessageLogger(l.baseLogger.Debug())
}

func (l LDPrefixLogger) Info() LDMessageLogger {
	return l.prefixMessageLogger(l.baseLogger.Info())
}

func (l LDPrefixLogger) Warn() LDMessageLogger {
	return l.prefixMessageLogger(l.baseLogger.Warn())
}

func (l LDPrefixLogger) Error() LDMessageLogger {
	return l.prefixMessageLogger(l.baseLogger.Error())
}

func (l LDPrefixLogger) Status() fs.ServiceStatus {
	return l.baseLogger.Status()
}

func (l LDPrefixLogger) IsDebug() bool {
	return l.baseLogger.IsDebug()
}

func NewPrefixLogger(l LDLogger, prefix string) LDPrefixLogger {
	return LDPrefixLogger{
		prefix:     prefix,
		baseLogger: l,
	}
}
