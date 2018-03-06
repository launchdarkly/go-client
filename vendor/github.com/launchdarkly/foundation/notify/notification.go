package notify

import (
	"fmt"

	"github.com/go-errors/errors"
	"github.com/launchdarkly/foundation/ferror"
	"github.com/launchdarkly/foundation/ferror_reporting"
)

// Notifications can have a message, an error, a stack, and/or an error context.
type Notification interface {
	Stack() []errors.StackFrame
	error
	ferror_reporting.ErrorContexter
	fmt.Stringer
}

type notificationImpl struct {
	error
	message string
	ferror_reporting.ErrorContexter
}

func NewNotification(ctx ferror_reporting.ErrorContexter, err error, message string) Notification {
	return notificationImpl{err, message, ctx}
}

func NewErrorNotification(ctx ferror_reporting.ErrorContexter, err error) Notification {
	return NewNotification(ctx, err, "")
}

func NewMessageNotification(ctx ferror_reporting.ErrorContexter, message string) Notification {
	return NewNotification(ctx, nil, message)
}

func (n notificationImpl) ErrorContext() map[string]interface{} {
	if n.ErrorContexter != nil {
		return n.ErrorContexter.ErrorContext()
	} else {
		return map[string]interface{}{}
	}
}

func (n notificationImpl) String() string {
	if n.error != nil {
		return n.error.Error()
	} else {
		return n.message
	}
}

func (n notificationImpl) Stack() []errors.StackFrame {
	switch err := n.error.(type) {
	case (*errors.Error):
		return err.StackFrames()
	case *ferror.Error:
		return err.Stack
	default:
		return nil
	}
}
