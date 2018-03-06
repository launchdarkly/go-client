package concurrent

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGoSafelyRecoversFromPanic(t *testing.T) {
	t.Parallel()
	GoSafely(func() {
		panic("Uh oh, something bad happened")
	})
}

func TestGoSafelyExecutesClosure(t *testing.T) {
	t.Parallel()
	ch := make(chan int)

	GoSafely(func() {
		ch <- 42
	})

	res := <-ch
	assert.Equal(t, 42, res, "Failed to communicate value back from closure")
}

func TestAwaitSafelyRecoversFromPanic(t *testing.T) {
	t.Parallel()
	wg := sync.WaitGroup{}
	AwaitSafely(&wg, func() {
		panic("Uh oh, something bad happened")
	})
}

func TestAwaitSafelyExecutesClosure(t *testing.T) {
	t.Parallel()
	ch := make(chan int)

	wg := sync.WaitGroup{}
	AwaitSafely(&wg, func() {
		ch <- 42
	})
	AwaitSafely(&wg, func() {
		panic("Uh oh, something bad happened")
	})
	res := <-ch
	wg.Wait()
	close(ch)
	assert.Equal(t, 42, res, "Failed to communicate value back from closure")
}

func TestAwaitSafelyExecutesInsideForLoop(t *testing.T) {
	expectedCount := 4
	expectedSum := 6
	ch := make(chan int, expectedCount)
	wg := sync.WaitGroup{}

	for i := 0; i < expectedCount; i++ {
		local_i := i
		AwaitSafely(&wg, func() {
			time.Sleep(time.Millisecond * 50)
			t.Logf("In AwaitSafely: %d", local_i)
			ch <- local_i
		})
	}

	wg.Wait()
	close(ch)
	count := 0
	sum := 0
	for i := range ch {
		count++
		sum += i
		t.Logf("Result: %d", i)
	}
	assert.Equal(t, expectedCount, count)
	assert.Equal(t, expectedSum, sum)
}

func TestInBackgroundEagerExecution(t *testing.T) {
	// WILL NOT WORK IN PARALLEL!
	eager = true
	defer func() { eager = false }()

	ch := make(chan int)

	go func() {
		assert.Equal(t, 1, <-ch, "background task finished early")
		assert.Equal(t, 2, <-ch)
	}()

	InBackground(nil, func() {
		time.Sleep(time.Second) // Forgive me for making this test take a second
		ch <- 1
	})

	ch <- 2
}

type errorReporter struct {
	ch chan int
}

func (r errorReporter) AutoNotify() {
	r.ch <- 42
}

func TestInBackgroundWithPanic(t *testing.T) {
	t.Parallel()

	reporter := errorReporter{make(chan int)}
	InBackground(reporter, func() {
		panic("uh oh")
	})

	assert.Equal(t, 42, <-reporter.ch)
}

func TestInBackgroundWithoutPanic(t *testing.T) {
	t.Parallel()

	ch := make(chan int)
	InBackground(errorReporter{}, func() {
		ch <- 42
	})

	assert.Equal(t, 42, <-ch)
}

func TestErrorTrackInParallelWithPanic(t *testing.T) {
	t.Parallel()

	reporter := errorReporter{make(chan int)}
	ErrorTrackInParallel(reporter, func() {
		panic("uh oh")
	})

	assert.Equal(t, 42, <-reporter.ch)
}

func TestErrorTrackInParallelWithoutPanic(t *testing.T) {
	t.Parallel()

	ch := make(chan int)
	ErrorTrackInParallel(errorReporter{}, func() {
		ch <- 42
	})

	assert.Equal(t, 42, <-ch)
}

func TestErrorTrackInParallelWithoutReporter(t *testing.T) {
	t.Parallel()

	ErrorTrackInParallel(nil, func() {
		panic("uh oh")
	})
}
