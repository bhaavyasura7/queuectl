package worker

import (
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
)

var draining atomic.Bool

func IsDraining() bool {
	return draining.Load()
}

func SetDraining() {
	draining.Store(true)
}

func SetupSignalHandler(onSignal func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, os.Interrupt)

	go func() {
		<-ch
		SetDraining()
		if onSignal != nil {
			onSignal()
		}
	}()
}
