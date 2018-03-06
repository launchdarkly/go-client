package ferror_reporting

import (
	"context"
	"net/http"
	"reflect"
)

const ERROR_CONTEXT_NAME = "errorContext"

type ErrorContexter interface {
	ErrorContext() map[string]interface{}
}

// Use a private type so we discover any collisions with our "errorContext" key on the request context
type errorContext struct {
	contexts **[]ErrorContexter
}

func MakeErrorContext() errorContext {
	contexts := &[]ErrorContexter{}
	return errorContext{contexts: &contexts}
}

func (c errorContext) Add(newCtx ErrorContexter) errorContext {
	newContexts := append(**c.contexts, newCtx)
	*c.contexts = &newContexts
	return c
}

// Return all the contexts combined
func (c errorContext) ErrorContext() map[string]interface{} {
	fields := map[string]interface{}{}
	for _, ctx := range **c.contexts {
		for k, v := range ctx.ErrorContext() {
			fields[k] = v
		}
	}
	return fields
}

func MergeErrorContexts(ctxs ...ErrorContexter) ErrorContexter {
	merged := MakeErrorContext()
	for _, errorContext := range ctxs {
		merged.Add(errorContext)
	}
	return merged
}

func WithErrorContext(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue((*r).Context(), ERROR_CONTEXT_NAME, MakeErrorContext()))
}

func GetRequestErrorContext(r *http.Request) errorContext {
	if ctx := r.Context().Value(ERROR_CONTEXT_NAME); ctx == nil {
		return MakeErrorContext()
	} else {
		return ctx.(errorContext)
	}
}

type RequestErrorContext struct{ *http.Request }

func (r RequestErrorContext) ErrorContext() map[string]interface{} {
	context := r.Context().Value(ERROR_CONTEXT_NAME).(errorContext).ErrorContext()
	if context != nil {
		// The type of the request error context is private so we need to do some magic to the map type
		mapType := reflect.TypeOf(map[string]interface{}{})
		return reflect.ValueOf(context).Convert(mapType).Interface().(map[string]interface{})
	} else {
		return nil
	}
}
