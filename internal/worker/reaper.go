package worker

import (
	"log"
	"time"

	"queuectl/internal/store"
)

func StartReaper(st *store.Store) {
	ticker := time.NewTicker(5 * time.Second)

	go func() {
		for range ticker.C {
			if err := st.ReaperScan(15 * time.Second); err != nil {
				log.Printf("reaper error: %v", err)
			}
		}
	}()
}
