package cli

import (
	"fmt"
	"log"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"queuectl/internal/store"
	"queuectl/internal/worker"
)

func NewWorkerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Manage workers",
	}

	var count int
	var dbPath string

	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start a worker process",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.SetOutput(cmd.ErrOrStderr())

			st, err := store.Open(dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			if err := st.RegisterWorker(); err != nil {
				return err
			}

			workerID := fmt.Sprintf("%d", os.Getpid())
			log.Printf("worker %s started, count=%d", workerID, count)

			worker.SetupSignalHandler(func() {
				log.Printf("worker %s draining", workerID)
			})

			worker.StartReaper(st)

			var wg sync.WaitGroup
			for i := 0; i < count; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					claimLoop(st, workerID, id)
				}(i)
			}

			wg.Wait()
			st.DeregisterWorker()
			log.Printf("worker %s stopped", workerID)
			return nil
		},
	}

	startCmd.Flags().IntVar(&count, "count", 1, "Number of concurrent claim loops")
	startCmd.Flags().StringVar(&dbPath, "db", "./queuectl.db", "Path to SQLite database")

	stopCmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop all running workers",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.SetOutput(cmd.ErrOrStderr())

			st, err := store.Open(dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			rows, err := st.GetRunningWorkers()
			if err != nil {
				return err
			}
			defer rows.Close()

			var pids []int
			for rows.Next() {
				var pid int
				var hostname, startedAt, status string
				if err := rows.Scan(&pid, &hostname, &startedAt, &status); err != nil {
					return err
				}
				pids = append(pids, pid)
			}

			for _, pid := range pids {
				if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
					if err == syscall.ESRCH {
						st.DeregisterWorkerByPID(pid)
						log.Printf("worker %d already dead, cleaned up", pid)
					} else {
						log.Printf("signal worker %d: %v", pid, err)
					}
				} else {
					log.Printf("signalled worker %d", pid)
				}
			}

			for i := 0; i < 20; i++ {
				count, err := st.LiveWorkerCount()
				if err != nil {
					return err
				}
				if count == 0 {
					return nil
				}
				time.Sleep(500 * time.Millisecond)
			}

			return fmt.Errorf("timeout waiting for workers to stop")
		},
	}

	stopCmd.Flags().StringVar(&dbPath, "db", "./queuectl.db", "Path to SQLite database")

	cmd.AddCommand(startCmd)
	cmd.AddCommand(stopCmd)

	return cmd
}

func claimLoop(st *store.Store, workerID string, id int) {
	for {
		if worker.IsDraining() {
			return
		}

		jobID, command, err := st.ClaimNext(workerID)
		if err == store.ErrNoJob {
			time.Sleep(1 * time.Second)
			continue
		}
		if err != nil {
			log.Printf("claim error: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		log.Printf("[%d] claimed job %s: %s", id, jobID, command)

		timeout, _ := st.GetJobTimeout(jobID)
		result := worker.RunWithHeartbeat(jobID, st, func() worker.RunResult {
			return worker.RunCommandWithTimeout(command, timeout)
		})

		if result.ExitCode == 0 {
			if err := st.CompleteJob(jobID); err != nil {
				log.Printf("[%d] complete error job %s: %v", id, jobID, err)
			} else {
				log.Printf("[%d] completed job %s", id, jobID)
			}
		} else {
			attempts, maxRetries, backoffBase, err := st.GetJobStats(jobID)
			if err != nil {
				log.Printf("[%d] get job stats error %s: %v", id, jobID, err)
				continue
			}

			if err := st.FailJob(jobID, attempts, maxRetries, backoffBase, result.Error, result.Stdout, result.Stderr); err != nil {
				log.Printf("[%d] fail error job %s: %v", id, jobID, err)
			} else {
				log.Printf("[%d] failed job %s: %s", id, jobID, result.Error)
			}
		}
	}
}
