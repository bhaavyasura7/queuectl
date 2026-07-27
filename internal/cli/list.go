package cli

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"queuectl/internal/store"
	"queuectl/internal/types"
)

func NewListCmd() *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.SetOutput(cmd.ErrOrStderr())

			state, _ := cmd.Flags().GetString("state")
			useJSON, _ := cmd.Flags().GetBool("json")

			st, err := store.Open(dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			rows, err := st.ListJobsByState(state)
			if err != nil {
				return err
			}
			defer rows.Close()

			if useJSON {
				var jobs []map[string]interface{}
				for rows.Next() {
					job, err := store.ScanJob(rows)
					if err != nil {
						return err
					}
					jobs = append(jobs, jobToMap(job))
				}
				if jobs == nil {
					jobs = []map[string]interface{}{}
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				return enc.Encode(jobs)
			}

			for rows.Next() {
				job, err := store.ScanJob(rows)
				if err != nil {
					return err
				}
				fmt.Printf("%s  %-10s  %s\n", job.ID, job.State, job.Command)
			}

			return nil
		},
	}

	cmd.Flags().String("state", "", "Filter by state")
	cmd.Flags().Bool("json", false, "Output as JSON")
	cmd.Flags().StringVar(&dbPath, "db", "./queuectl.db", "Path to SQLite database")
	return cmd
}

func jobToMap(j types.Job) map[string]interface{} {
	return map[string]interface{}{
		"id":              j.ID,
		"command":         j.Command,
		"state":           j.State,
		"priority":        j.Priority,
		"run_at":          j.RunAt,
		"timeout":         j.Timeout,
		"attempts":        j.Attempts,
		"max_retries":     j.MaxRetries,
		"backoff_base":    j.BackoffBase,
		"created_at":      j.CreatedAt,
		"updated_at":      j.UpdatedAt,
		"next_attempt_at": j.NextAttemptAt,
		"claimed_at":      j.ClaimedAt,
		"heartbeat_at":    j.HeartbeatAt,
		"worker_id":       j.WorkerID,
		"last_error":      j.LastError,
		"stdout":          j.Stdout,
		"stderr":          j.Stderr,
	}
}
