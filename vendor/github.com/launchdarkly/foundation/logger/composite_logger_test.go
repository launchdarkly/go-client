package logger

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompositeLogger(t *testing.T) {
	var bufferA bytes.Buffer
	var bufferB bytes.Buffer
	loggerA := testLDLogger{LevelLogger{writer: &bufferA, level: LevelDebug}.Error()}
	loggerB := testLDLogger{LevelLogger{writer: &bufferB, level: LevelDebug}.Warn()}
	l := compositeLogger{
		[]LDLogger{loggerA, loggerB},
	}

	l.Error().Println("msg2")
	lineA, err := bufferA.ReadString('\n')
	assert.NoError(t, err)
	assert.Contains(t, lineA, "msg2")
	assert.Contains(t, lineA, "composite_logger_test")
	lineB, err := bufferB.ReadString('\n')
	assert.NoError(t, err)
	assert.Contains(t, lineB, "msg2")
	assert.Contains(t, lineB, "composite_logger_test")
}
