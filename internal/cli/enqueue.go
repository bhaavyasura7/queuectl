package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewEnqueueCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enqueue <json>",
		Short: "Enqueue a new job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.ErrOrStderr(), "not yet implemented")
			return nil
		},
	}
}
