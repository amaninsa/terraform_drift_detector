package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/api"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/cli/ui"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/config"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/models"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/cloud"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/scan"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/scheduler"
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
		Short: "Start the drift detection API, dashboard, and scheduler",
		RunE: func(cmd *cobra.Command, args []string) error {
			var cfg *config.Config
			if globalConfigPath != "" {
				var err error
				cfg, err = config.Load(globalConfigPath)
				if err != nil {
					return err
				}
				if port == 8080 && cfg.Server.Port != 0 {
					port = cfg.Server.Port
				}
				if dbPath == "drift.db" && cfg.Server.Database != "" {
					dbPath = cfg.Server.Database
				}
			}

			s, err := store.Open(dbPath)
			if err != nil {
				return err
			}
			defer s.Close()

			var sched *scheduler.Scheduler
			if cfg != nil && len(cfg.Schedules) > 0 {
				scanFn := func(ctx context.Context, opts models.ScanOptions) (*models.DriftReport, error) {
					adapter, err := cloud.NewAdapterFromScanOptions(ctx, opts)
					if err != nil {
						return nil, err
					}
					return scan.NewRunner(nil, adapter).Run(ctx, opts)
				}
				sched = scheduler.New(cfg, s, scanFn)
				if err := sched.Start(); err != nil {
					return err
				}
				defer sched.Stop()
				ui.Info(fmt.Sprintf("Scheduler started with %d job(s)", len(cfg.Schedules)))
			}

			srv := &api.Server{Store: s, Config: cfg}
			if webDir != "" {
				srv.StaticFS = http.Dir(webDir)
			}

			addr := fmt.Sprintf(":%d", port)
			ui.Success(fmt.Sprintf("Drift detector listening on http://localhost%s", addr))

			httpServer := &http.Server{Addr: addr, Handler: srv.NewMux()}
			go func() {
				if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					ui.Error(err.Error())
					os.Exit(1)
				}
			}()

			stop := make(chan os.Signal, 1)
			signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
			<-stop
			ui.Info("Shutting down...")
			return httpServer.Shutdown(context.Background())
		},
	}

	cmd.Flags().IntVar(&port, "port", 8080, "HTTP port")
	cmd.Flags().StringVar(&dbPath, "db", "drift.db", "SQLite database path")
	cmd.Flags().StringVar(&webDir, "web", "web", "Dashboard static files directory")

	return cmd
}
