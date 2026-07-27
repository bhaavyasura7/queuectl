package cli

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"queuectl/internal/store"
)

func NewOutputCmd() *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "output <id>",
		Short: "Show stdout/stderr for a job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			log.SetOutput(cmd.ErrOrStderr())

			st, err := store.Open(dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			row, err := st.GetJob(args[0])
			if err != nil {
				return err
			}
			j, err := store.ScanRow(row)
			if err != nil {
				return fmt.Errorf("job not found: %s", args[0])
			}

			fmt.Printf("State:  %s\n", j.State)
			fmt.Printf("Error:  %s\n", strPtr(j.LastError))
			fmt.Printf("\nSTDOUT:\n%s\n", strPtr(j.Stdout))
			fmt.Printf("STDERR:\n%s\n", strPtr(j.Stderr))
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "./queuectl.db", "Path to SQLite database")
	return cmd
}
