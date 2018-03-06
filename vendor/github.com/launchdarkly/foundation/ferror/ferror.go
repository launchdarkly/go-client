package ferror

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-errors/errors"
)

// See https://github.com/stripe/stripe-go/blob/master/error.go for inspiration

const (
	InvalidRequest         string = "invalid_request"
	OptimisticLockingError string = "optimistic_locking_error"
	InternalError          string = "internal_service_error"
	NotFound               string = "not_found"
	Unauthorized           string = "unauthorized"
	Forbidden              string = "forbidden"
)

type Error struct {
	Code       string              `json:"code"`
	Message    string              `json:"message"`
	StatusCode int                 `json:"-"`
	Underlying string              `json:"-"`
	Stack      []errors.StackFrame `json:"-"`
	RequestId  string              `json:"-"`
}

func NewInvalidRequest(message string, status int, underlying error, requestId string) *Error {
	return NewError(InvalidRequest, message, status, underlying, requestId)
}

func NewInternalError(message string, underlying error, requestId string) *Error {
	err := NewError(InternalError, message, http.StatusInternalServerError, underlying, requestId)
	if stackErr, ok := underlying.(*errors.Error); ok {
		err.Stack = stackErr.StackFrames()
	} else {
		// If underlying doesn't already have a stack, wrap it with one here.
		stackErr = errors.Wrap(underlying, 1)
		underlying = stackErr
		err.Stack = stackErr.StackFrames()
	}
	return err
}

func NewOptimisticLockingError(message string, underlying error, requestId string) *Error {
	return NewError(OptimisticLockingError, message, http.StatusConflict, underlying, requestId)
}

func NewNotFoundError(message string, underlying error, requestId string) *Error {
	return NewError(NotFound, message, http.StatusNotFound, underlying, requestId)
}

func NewUnauthorizedError(message string, underlying error, requestId string) *Error {
	return NewError(Unauthorized, message, http.StatusUnauthorized, underlying, requestId)
}

func NewForbiddenError(message string, underlying error, requestId string) *Error {
	return NewError(Forbidden, message, http.StatusForbidden, underlying, requestId)
}

func NewJsonUnmarshalError(underlying error, requestId string) *Error {
	switch underlying.(type) {
	case nil:
		return nil
	case *json.SyntaxError:
		return NewError(InvalidRequest, "Unable to parse JSON", http.StatusBadRequest, underlying, requestId)
	case *json.UnmarshalTypeError:
		return NewError(InvalidRequest, "Unexpected JSON representation", http.StatusBadRequest, underlying, requestId)
	default:
		return NewInternalError("Unable to read JSON", underlying, requestId)
	}

}

func NewError(code, message string, status int, underlying error, requestId string) *Error {
	var underlyingMsg string

	if underlying != nil {
		underlyingMsg = underlying.Error()
	}

	return &Error{
		Code:       code,
		Message:    message,
		StatusCode: status,
		Underlying: underlyingMsg,
		RequestId:  requestId,
	}
}

func (e Error) Log() string {
	var causedBy, stackTrace string

	if e.Underlying != "" {
		causedBy = "\n--- Caused by " + e.Underlying
	}
	if len(e.Stack) > 0 {
		stackTrace = "\n--- Stack Trace\n"
		for _, frame := range e.Stack {
			stackTrace += frame.String()
		}
	}

	return fmt.Sprintf("[%d, %s, Req %s] %s %s %s",
		e.StatusCode, e.Code, e.RequestId, e.Message, causedBy, stackTrace)
}

func (e *Error) Error() string {
	return string(e.Code)
}

func (e *Error) Json() []byte {
	ret, _ := json.Marshal(e)
	return ret
}

func FromError(err error, requestId string) *Error {
	if err == nil {
		return nil
	}
	if ferr, ok := err.(*Error); ok {
		return ferr
	} else {
		return NewInternalError("Internal error", err, requestId)
	}
}
