package eventsource

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
)

type publication struct {
	id, event, data string
	retry           int64
}

func (s *publication) Id() string    { return s.id }
func (s *publication) Event() string { return s.event }
func (s *publication) Data() string  { return s.data }
func (s *publication) Retry() int64  { return s.retry }

// A Decoder is capable of reading Events from a stream.
type Decoder struct {
	*bufio.Reader
}

// NewDecoder returns a new Decoder instance that reads events
// with the given io.Reader.
func NewDecoder(r io.Reader) *Decoder {
	dec := &Decoder{bufio.NewReader(newNormaliser(r))}
	return dec
}

func (dec *Decoder) readString(inputCh chan string, errCh chan error) {
	result, err := dec.ReadString('\n')

	if err != nil {
		errCh <- err
	} else {
		inputCh <- result
	}
}

func (dec *Decoder) peek(peekCh chan bool, errCh chan error) {
	_, err := dec.Peek(1)

	if err != nil {
		errCh <- err
	} else {
		peekCh <- true
	}
}

// Decode reads the next Event from a stream (and will block until one
// comes in).
// Graceful disconnects (between events) are indicated by an io.EOF error.
// Any error occuring mid-event is considered non-graceful and will
// show up as some other error (most likely io.ErrUnexpectedEOF).
// Decoding will return either a successfully parsed Event, a comment, or
// an error.
func (dec *Decoder) Decode(reconnectAfter time.Duration) (Event, *string, error) {

	// peek ahead before we start a new event so we can return EOFs
	var err error
	if reconnectAfter > 0 {
		peekCh := make(chan bool, 1)
		errCh := make(chan error, 1)
		go dec.peek(peekCh, errCh)
		select {
		case <-peekCh:
		case err = <-errCh:
		case <-time.After(reconnectAfter):
			err = errors.New("Timeout encountered waiting for next event")
		}
	} else {
		_, err = dec.Peek(1)
	}

	if err == io.ErrUnexpectedEOF {
		err = io.EOF
	}
	if err != nil {
		return nil, nil, err
	}
	pub := new(publication)
	for {
		var line string
		var err error

		if reconnectAfter > 0 {
			inputCh := make(chan string, 1)
			errCh := make(chan error, 1)
			go dec.readString(inputCh, errCh)

			select {
			case line = <-inputCh:
			case err = <-errCh:
			case <-time.After(reconnectAfter):
				err = errors.New("Timeout encountered while parsing event")
			}
		} else {
			line, err = dec.ReadString('\n')
		}
		if err != nil {
			return nil, nil, err
		}
		if line == "\n" {
			break
		}
		line = strings.TrimSuffix(line, "\n")
		if strings.HasPrefix(line, ":") {
			comment := line[1:]
			return nil, &comment, nil
		}
		sections := strings.SplitN(line, ":", 2)
		field, value := sections[0], ""
		if len(sections) == 2 {
			value = strings.TrimPrefix(sections[1], " ")
		}
		switch field {
		case "event":
			pub.event = value
		case "data":
			pub.data += value + "\n"
		case "id":
			pub.id = value
		case "retry":
			pub.retry, _ = strconv.ParseInt(value, 10, 64)
		}
	}
	pub.data = strings.TrimSuffix(pub.data, "\n")
	return pub, nil, nil
}
