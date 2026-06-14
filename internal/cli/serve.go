package cli

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/api"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/store"
)

// NewServeCommand starts the API server and dashboard.
func NewServeCommand() *cobra.Command {
	var (
		port   int
		dbPath string
		webDir string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the drift detection API and dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store.Open(dbPath)
			if err != nil {
				return err
			}
			defer s.Close()

			srv := &api.Server{Store: s}
			if webDir != "" {
				srv.StaticFS = http.Dir(webDir)
			}

			addr := fmt.Sprintf(":%d", port)
			fmt.Printf("Drift detector listening on http://localhost%s\n", addr)
			return http.ListenAndServe(addr, srv.NewMux())
		},
	}

	cmd.Flags().IntVar(&port, "port", 8080, "HTTP port")
	cmd.Flags().StringVar(&dbPath, "db", "drift.db", "SQLite database path")
	cmd.Flags().StringVar(&webDir, "web", "web", "Dashboard static files directory")

	return cmd
}
