package cli

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"queuectl/internal/store"
)

func NewConfigCmd() *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value (max-retries, backoff-base)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			log.SetOutput(cmd.ErrOrStderr())

			key, value := args[0], args[1]
			if key != "max-retries" && key != "backoff-base" {
				return fmt.Errorf("unknown config key: %s (valid: max-retries, backoff-base)", key)
			}

			st, err := store.Open(dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			if err := st.SetConfig(key, value); err != nil {
				return err
			}

			fmt.Printf("set %s = %s\n", key, value)
			return nil
		},
	})

	cmd.PersistentFlags().StringVar(&dbPath, "db", "./queuectl.db", "Path to SQLite database")
	return cmd
}
