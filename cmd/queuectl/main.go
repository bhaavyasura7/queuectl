package main

import (
	"os"

	"github.com/spf13/cobra"
	"queuectl/internal/cli"
)

func main() {
	root := &cobra.Command{
		Use:   "queuectl",
		Short: "SQLite-backed job queue CLI",
	}

	root.AddCommand(cli.NewEnqueueCmd())
	root.AddCommand(cli.NewWorkerCmd())
	root.AddCommand(cli.NewStatusCmd())
	root.AddCommand(cli.NewListCmd())
	root.AddCommand(cli.NewDlqCmd())
	root.AddCommand(cli.NewConfigCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
