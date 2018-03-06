package async

import (
	"time"

	"github.com/launchdarkly/foundation/concurrent"
	status "github.com/launchdarkly/foundation/statuschecks"
)

type FutureStatusRep chan status.StatusRep
type FutureServiceStatus chan status.ServiceStatus

// Runs the given status check function asynchronously. If it takes longer than
// 200ms to get the the status, it will be considered down.
func MakeFutureStatusRep(f func() status.StatusRep) FutureStatusRep {
	ret := make(FutureStatusRep)
	c := make(FutureStatusRep)
	concurrent.GoSafely(func() {
		c <- f()
	})
	timer := time.NewTimer(200 * time.Millisecond)
	concurrent.GoSafely(func() {
		select {
		case s := <-c:
			ret <- s
		case <-timer.C:
			ret <- status.DownServiceRep()
		}
	})
	return ret
}

// Runs the given status check function asynchronously. If it takes longer than
// 200ms to get the the status, it will be considered down.
func MakeFutureServiceStatus(f func() status.ServiceStatus) FutureServiceStatus {
	ret := make(FutureServiceStatus)
	c := make(FutureServiceStatus)
	concurrent.GoSafely(func() {
		c <- f()
	})
	timer := time.NewTimer(200 * time.Millisecond)
	concurrent.GoSafely(func() {
		select {
		case s := <-c:
			ret <- s
		case <-timer.C:
			ret <- status.DownService()
		}
	})
	return ret
}

func FutureCheckStatus(resource string) FutureStatusRep {
	return MakeFutureStatusRep(func() status.StatusRep {
		return status.CheckStatus(resource)
	})
}
