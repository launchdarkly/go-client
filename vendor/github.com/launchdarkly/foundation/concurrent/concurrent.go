package concurrent

import (
	"os"
	"runtime"
	"sync"

	"github.com/launchdarkly/foundation/ferror_reporting"
	"github.com/launchdarkly/foundation/logger"
	"github.com/launchdarkly/go-metrics"
)

var (
	panicCounter = metrics.GetOrRegisterCounter("panics", metrics.DefaultRegistry)
)

func GoSafely(fn func()) {
	go func() {

		defer func() {
			if err := recover(); err != nil {
				stack := make([]byte, 1024*8)
				stack = stack[:runtime.Stack(stack, false)]

				f := "PANIC: %s\n%s"
				logger.Logger.Error().Printf(f, err, stack)
				panicCounter.Inc(1)
			}
		}()

		fn()
	}()
}

// Convenience function for using GoSafely with a sync.WaitGroup.
// Takes a sync.WaitGroup, increments it, and calls fn via GoSafely()
// Don't forget to call wg.Wait() to actually wait for fn to return.
func AwaitSafely(wg *sync.WaitGroup, fn func()) {
	wg.Add(1)
	GoSafely(func() {
		defer wg.Done()
		fn()
	})
}

var (
	eager = false
)

func init() {
	if os.Getenv("BACKGROUND_JOBS_ARE_EAGER") == "1" {
		eager = true
	}
}

func InBackground(reporter ferror_reporting.ErrorReporter, fn func()) {
	if eager {
		fn()
	} else {
		ErrorTrackInParallel(reporter, fn)
	}
}

func ErrorTrackInParallel(reporter ferror_reporting.ErrorReporter, fn func()) {
	GoSafely(func() {
		if reporter != nil {
			defer reporter.AutoNotify()
		}
		fn()
	})
}

// Convenience function for using GoSafely with a sync.WaitGroup.
// Takes a sync.WaitGroup, increments it, and calls fn via GoSafely()
// Don't forget to call wg.Wait() to actually wait for fn to return.
func AwaitErrorTrackInParallel(reporter ferror_reporting.ErrorReporter, wg *sync.WaitGroup, fn func()) {
	wg.Add(1)
	ErrorTrackInParallel(reporter, func() {
		defer wg.Done()
		fn()
	})
}
