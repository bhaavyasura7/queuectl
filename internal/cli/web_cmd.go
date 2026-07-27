package cli

import (
	"fmt"
	"log"
	"net/http"

	"github.com/spf13/cobra"
	"queuectl/internal/store"
	"queuectl/internal/web"
)

func NewWebCmd() *cobra.Command {
	var dbPath string
	var port string

	cmd := &cobra.Command{
		Use:   "web",
		Short: "Start web dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.SetOutput(cmd.ErrOrStderr())

			st, err := store.Open(dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			srv := web.NewServer(st)
			addr := fmt.Sprintf(":%s", port)
			log.Printf("dashboard: http://localhost%s", addr)
			return http.ListenAndServe(addr, srv.Handler())
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "./queuectl.db", "Path to SQLite database")
	cmd.Flags().StringVar(&port, "port", "9090", "Listen port")
	return cmd
}
