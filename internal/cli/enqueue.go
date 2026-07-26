package cli

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"queuectl/internal/store"
)

type enqueueInput struct {
	Command     string  `json:"command"`
	Priority    string  `json:"priority,omitempty"`
	MaxRetries  *int    `json:"max_retries,omitempty"`
	BackoffBase *float64 `json:"backoff_base,omitempty"`
	RunAt       *string `json:"run_at,omitempty"`
	Timeout     *int    `json:"timeout,omitempty"`
}

func NewEnqueueCmd() *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "enqueue <json>",
		Short: "Enqueue a new job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var input enqueueInput
			if err := json.Unmarshal([]byte(args[0]), &input); err != nil {
				return fmt.Errorf("invalid JSON: %w", err)
			}
			if input.Command == "" {
				return fmt.Errorf("command field is required")
			}
			if input.Priority == "" {
				input.Priority = "normal"
			}

			st, err := store.Open(dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			maxRetries, backoffBase, err := resolveConfig(st, input.MaxRetries, input.BackoffBase)
			if err != nil {
				return err
			}

			id := generateID()
			now := time.Now().UTC().Format(time.RFC3339)

			if err := st.InsertJob(id, input.Command, now, input.Priority, input.RunAt, input.Timeout, maxRetries, backoffBase); err != nil {
				return fmt.Errorf("insert job: %w", err)
			}

			fmt.Println(id)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "./queuectl.db", "Path to SQLite database")
	return cmd
}

func resolveConfig(st *store.Store, maxRetries *int, backoffBase *float64) (int, float64, error) {
	if maxRetries != nil && backoffBase != nil {
		return *maxRetries, *backoffBase, nil
	}

	if maxRetries == nil {
		v, err := st.GetConfig("max-retries")
		if err != nil {
			return 0, 0, err
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid config max-retries: %s", v)
		}
		maxRetries = &n
	}
	if backoffBase == nil {
		v, err := st.GetConfig("backoff-base")
		if err != nil {
			return 0, 0, err
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid config backoff-base: %s", v)
		}
		backoffBase = &f
	}

	return *maxRetries, *backoffBase, nil
}

func generateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
