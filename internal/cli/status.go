package cli

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"queuectl/internal/store"
)

func NewStatusCmd() *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show queue status",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.SetOutput(cmd.ErrOrStderr())

			st, err := store.Open(dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			rows, err := st.CountJobsByState()
			if err != nil {
				return err
			}
			defer rows.Close()

			counts := map[string]int{
				"pending": 0, "processing": 0, "completed": 0, "failed": 0, "dead": 0,
			}
			total := 0

			for rows.Next() {
				var state string
				var count int
				if err := rows.Scan(&state, &count); err != nil {
					return err
				}
				counts[state] = count
				total += count
			}

			liveWorkers, err := st.LiveWorkerCount()
			if err != nil {
				return err
			}

			fmt.Printf("pending:     %d\n", counts["pending"])
			fmt.Printf("processing:  %d\n", counts["processing"])
			fmt.Printf("completed:   %d\n", counts["completed"])
			fmt.Printf("failed:      %d\n", counts["failed"])
			fmt.Printf("dead:        %d\n", counts["dead"])
			fmt.Printf("────────────────\n")
			fmt.Printf("total:       %d\n", total)
			fmt.Printf("live workers: %d\n", liveWorkers)

			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "./queuectl.db", "Path to SQLite database")
	return cmd
}
