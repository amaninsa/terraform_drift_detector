package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/cli/ui"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/config"
)

// NewScheduleCommand lists configured scan schedules.
func NewScheduleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "View configured drift scan schedules",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List cron schedules from config",
		RunE: func(cmd *cobra.Command, args []string) error {
			if globalConfigPath == "" {
				return fmt.Errorf("--config is required")
			}
			cfg, err := config.Load(globalConfigPath)
			if err != nil {
				return err
			}
			if len(cfg.Schedules) == 0 {
				ui.Info("No schedules configured")
				return nil
			}
			fmt.Printf("%-16s %s\n", "WORKSPACE", "CRON")
			fmt.Println("------------------------------------------")
			for _, s := range cfg.Schedules {
				fmt.Printf("%-16s %s\n", s.Workspace, s.Cron)
			}
			return nil
		},
	})
	return cmd
}
