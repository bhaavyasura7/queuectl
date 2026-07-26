package cli

import (
	"fmt"
	"log"
	"os"
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

			workerID := fmt.Sprintf("%d", os.Getpid())
			log.Printf("worker %s started, count=%d", workerID, count)

			for i := 0; i < count; i++ {
				go claimLoop(st, workerID, i)
			}

			select {}
		},
	}

	startCmd.Flags().IntVar(&count, "count", 1, "Number of concurrent claim loops")
	startCmd.Flags().StringVar(&dbPath, "db", "./queuectl.db", "Path to SQLite database")

	cmd.AddCommand(startCmd)
	cmd.AddCommand(&cobra.Command{
		Use:   "stop",
		Short: "Stop all running workers",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.ErrOrStderr(), "not yet implemented")
			return nil
		},
	})

	return cmd
}

func claimLoop(st *store.Store, workerID string, id int) {
	for {
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

		result := worker.RunCommand(command)

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
