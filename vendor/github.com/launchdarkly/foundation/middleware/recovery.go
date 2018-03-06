package middleware

// based on https://github.com/codegangsta/negroni/blob/master/recovery.go

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"runtime"

	"github.com/justinas/alice"
	"github.com/launchdarkly/foundation/logger"
)

// Recovery is a Negroni middleware that recovers from any panics and writes a 500 if there was one.
type Recovery struct {
	Logger     logger.LDLogger
	PrintStack bool
	StackAll   bool
	StackSize  int
}

// NewRecovery returns a new instance of Recovery
func NewRecovery() *Recovery {
	return &Recovery{
		Logger:     logger.Logger,
		PrintStack: true,
		StackAll:   false,
		StackSize:  1024 * 8,
	}
}

func (rec *Recovery) ServeHTTP(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	// We eagerly get the dumped http request so we can log the body in case of a panic.
	dumpBytes, dumpErr := httputil.DumpRequest(r, true)

	defer func() {
		if err := recover(); err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			stack := make([]byte, rec.StackSize)
			stack = stack[:runtime.Stack(stack, rec.StackAll)]

			f := "PANIC: %s\n%s"
			var reqMsg string
			if dumpErr != nil {
				rec.Logger.Error().Printf("Error when dumping http request: %s", dumpErr)
				reqMsg = logger.ReqMsg(r)
			} else {
				reqMsg = string(dumpBytes)
			}
			rec.Logger.Error().Printf("Panic when handling request: %s", reqMsg)
			rec.Logger.Error().Printf(f, err, stack)

			if rec.PrintStack {
				fmt.Fprintf(rw, f, err, stack)
			}
		}
	}()

	next(rw, r)
}

func RecoveryHandler(recovery Recovery) alice.Constructor {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recovery.ServeHTTP(w, r, h.ServeHTTP)
		})
	}
}
