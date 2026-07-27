package worker

import (
	"time"

	"queuectl/internal/store"
)

func RunWithHeartbeat(jobID string, st *store.Store, fn func() RunResult) RunResult {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				st.TouchHeartbeat(jobID)
			case <-done:
				return
			}
		}
	}()

	result := fn()
	close(done)
	return result
}
