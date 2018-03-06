package notify

import (
	"github.com/launchdarkly/foundation/ferror_reporting/frollbar"
	"github.com/stvp/rollbar"
)

type Rollbar struct{}

func (r Rollbar) Output(calldepth int, level Level, notification Notification) {
	rollbarLevel := rollbar.ERR
	switch level {
	case LevelError:
		rollbarLevel = rollbar.ERR
	case LevelWarn:
		rollbarLevel = rollbar.WARN
	case LevelInfo:
		rollbarLevel = rollbar.INFO
	case LevelDebug:
		rollbarLevel = rollbar.DEBUG
	}

	stack := notification.Stack()
	if stack != nil {
		// If we have a stack, convert it to rollber's format and use that rather than
		// having rollbar create its own right here, since ours probably has more details.
		var rollbarStack rollbar.Stack
		for _, frame := range stack {
			rollbarStack = append(rollbarStack, rollbar.Frame{
				Filename: frame.File,
				Method:   frame.Name,
				Line:     frame.LineNumber,
			})
		}
		frollbar.NotifyWithStack(rollbarLevel, notification, notification, rollbarStack)
	} else {
		frollbar.Notify(calldepth, rollbarLevel, notification, notification)
	}
}
