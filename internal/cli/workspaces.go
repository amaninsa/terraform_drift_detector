package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/cli/ui"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/config"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/state/backend"
)

// NewWorkspacesCommand lists configured workspaces.
func NewWorkspacesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "workspaces",
		Short: "List configured Terraform workspaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			if globalConfigPath == "" {
				return fmt.Errorf("--config is required")
			}
			cfg, err := config.Load(globalConfigPath)
			if err != nil {
				return err
			}
			if len(cfg.Workspaces) == 0 {
				ui.Info("No workspaces configured")
				return nil
			}
			fmt.Printf("%-12s %-10s %-14s %s\n", "NAME", "PROVIDER", "REGION", "STATE")
			fmt.Println("----------------------------------------------------------------")
			for _, ws := range cfg.Workspaces {
				fmt.Printf("%-12s %-10s %-14s %s\n",
					ws.Name,
					ws.Provider,
					ws.Region,
					backend.Describe(ws.State, ""),
				)
			}
			return nil
		},
	}
}
