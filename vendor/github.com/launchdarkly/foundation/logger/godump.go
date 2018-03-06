package logger

import (
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

func registerGodump() {
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGQUIT)
		buf := make([]byte, 1<<20)
		for {
			<-sigs
			runtime.Stack(buf, true)
			Info.Printf("--- goroutine dump ---\n%s\n--- end ---\n", buf)
		}
	}()
}
