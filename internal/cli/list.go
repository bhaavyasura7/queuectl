package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.ErrOrStderr(), "not yet implemented")
			return nil
		},
	}

	cmd.Flags().String("state", "", "Filter by state")
	cmd.Flags().Bool("json", false, "Output as JSON")

	return cmd
}
