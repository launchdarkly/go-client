package logger

import (
	"bytes"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrefixLoggerWithOutput(t *testing.T) {
	var buffer bytes.Buffer
	l := NewPrefixLogger(testLDLogger{log.New(&buffer, "", log.Lshortfile)}, "my-prefix: ")

	l.Info().Println("msg1")
	line, err := buffer.ReadString('\n')
	assert.Contains(t, line, "my-prefix: msg1")
	assert.Contains(t, line, "prefix_logger_test")
	assert.NoError(t, err)

	l.Info().Printf("msg%d", 2)
	line, err = buffer.ReadString('\n')
	assert.NoError(t, err)
	assert.Contains(t, line, "my-prefix: msg2")
	assert.Contains(t, line, "prefix_logger_test")
}
