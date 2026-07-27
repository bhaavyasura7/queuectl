package cli

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"queuectl/internal/store"
)

func NewDlqCmd() *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "dlq",
		Short: "Dead letter queue operations",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List dead-lettered jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.SetOutput(cmd.ErrOrStderr())

			st, err := store.Open(dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			rows, err := st.ListJobsByState("dead")
			if err != nil {
				return err
			}
			defer rows.Close()

			for rows.Next() {
				job, err := store.ScanJob(rows)
				if err != nil {
					return err
				}
				fmt.Printf("%s  %s  attempts=%d  error=%s\n", job.ID, job.Command, job.Attempts, strPtr(job.LastError))
			}

			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "retry <id>",
		Short: "Retry a dead-lettered job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			log.SetOutput(cmd.ErrOrStderr())

			st, err := store.Open(dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			id := args[0]
			ok, err := st.RetryDlqJob(id)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("job %s not found or not in dead state", id)
			}

			fmt.Printf("retried %s\n", id)
			return nil
		},
	})

	cmd.PersistentFlags().StringVar(&dbPath, "db", "./queuectl.db", "Path to SQLite database")
	return cmd
}

func strPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
