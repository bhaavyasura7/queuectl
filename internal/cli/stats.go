package cli

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"queuectl/internal/store"
)

func NewStatsCmd() *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show queue statistics",
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

			var total, completed, failed, dead int
			for rows.Next() {
				var state string
				var count int
				rows.Scan(&state, &count)
				total += count
				switch state {
				case "completed":
					completed = count
				case "failed":
					failed = count
				case "dead":
					dead = count
				}
			}

			fmt.Printf("total jobs:      %d\n", total)
			fmt.Printf("completed:       %d (%.1f%%)\n", completed, pct(completed, total))
			fmt.Printf("failed:          %d (%.1f%%)\n", failed, pct(failed, total))
			fmt.Printf("dead:            %d (%.1f%%)\n", dead, pct(dead, total))

			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "./queuectl.db", "Path to SQLite database")
	return cmd
}

func pct(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}
