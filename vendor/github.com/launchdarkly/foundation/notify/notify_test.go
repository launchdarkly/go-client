package notify

import (
	"testing"

	"github.com/go-errors/errors"
	"github.com/stretchr/testify/assert"
)

type result struct {
	calldepth int
	level     Level
	message   string
	ctx       map[string]interface{}
	stack     []errors.StackFrame
}

var results []result

type BufferSink struct{}

func (b BufferSink) Output(calldepth int, level Level, notification Notification) {
	results = append(results, result{calldepth, level, notification.String(), notification.ErrorContext(), notification.Stack()})
}

func initialize() {
	Initialize(Config{
		Route{
			MinLevel: LevelError,
			Pattern:  "mongod",
			Sinks:    []Sink{},
		},
		Route{
			MinLevel: LevelError,
			Sinks:    []Sink{BufferSink{}},
		},
		Route{
			MinLevel: LevelWarn,
			Sinks:    []Sink{BufferSink{}},
		},
	})
}

func TestBeforeInitialization(t *testing.T) {
	// Shouldn't error
	Info.Fmt(nil, "test")
}

func TestNotify(t *testing.T) {
	initialize()
	results = nil
	Error.Fmt(FakeErrorContexter{1}, "This is an error")
	Error.Fmt(FakeErrorContexter{2}, "mongod: This is an error")
	Warn.Fmt(FakeErrorContexter{3}, "mongod: This is a warning")
	Warn.Fmt(FakeErrorContexter{4}, "This is a warning")
	Info.Fmt(FakeErrorContexter{4}, "This is info")
	Warn.Fmt(FakeErrorContexter{6}, "This is a warning")
	Warn.Fmt(nil, "This is a warning with no context")

	assert.Equal(t, []result{
		{2, LevelError, "This is an error", map[string]interface{}{"test": 1}, nil},
		{2, LevelWarn, "mongod: This is a warning", map[string]interface{}{"test": 3}, nil},
		{2, LevelWarn, "This is a warning", map[string]interface{}{"test": 4}, nil},
		{2, LevelWarn, "This is a warning", map[string]interface{}{"test": 6}, nil},
		{2, LevelWarn, "This is a warning with no context", map[string]interface{}{}, nil},
	}, results)
}
