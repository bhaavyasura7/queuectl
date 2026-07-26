package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewDlqCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dlq",
		Short: "Dead letter queue operations",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List dead-lettered jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.ErrOrStderr(), "not yet implemented")
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "retry <id>",
		Short: "Retry a dead-lettered job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.ErrOrStderr(), "not yet implemented")
			return nil
		},
	})

	return cmd
}
