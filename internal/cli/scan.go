package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/cli/ui"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/config"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/models"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/notify"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/cloud"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/scan"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/state/backend"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/store"
)

var globalConfigPath string

// NewRootCommand creates the root CLI command.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "drift",
		Short: "Advanced Terraform drift detection — compare state against live cloud infrastructure",
		Long:  ui.Banner(),
	}
	root.PersistentFlags().StringVar(&globalConfigPath, "config", "", "Path to drift.yaml configuration file")
	root.PersistentFlags().BoolVar(&ui.NoColor, "no-color", false, "Disable colorized output")

	root.AddCommand(NewScanCommand())
	root.AddCommand(NewReportCommand())
	root.AddCommand(NewServeCommand())
	root.AddCommand(NewScheduleCommand())
	root.AddCommand(NewWorkspacesCommand())
	root.AddCommand(NewVersionCommand())
	return root
}

// NewScanCommand creates the scan subcommand.
func NewScanCommand() *cobra.Command {
	var (
		stateFile       string
		stateBucket     string
		stateKey        string
		stateRegion     string
		workspace       string
		provider        string
		region          string
		subscriptionID  string
		projectID       string
		output          string
		dbPath          string
		persist         bool
		detectUnmanaged bool
		ignoreRules     []string
		quiet           bool
	)

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Run a drift scan comparing Terraform state to live cloud resources",
		Example: `  drift scan --state-file ./terraform.tfstate --provider aws --region us-east-1
  drift scan --config drift.yaml --workspace prod --output rich
  drift scan --state-bucket my-tf-state --state-key prod/terraform.tfstate --detect-unmanaged`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			opts, webhookURLs, err := resolveScanOptions(workspace, stateFile, stateBucket, stateKey, stateRegion,
				provider, region, subscriptionID, projectID, detectUnmanaged, ignoreRules, globalConfigPath)
			if err != nil {
				return err
			}

			var spin *spinner.Spinner
			if !quiet && isRichOutput(output) {
				spin = spinner.New(spinner.CharSets[11], 100*time.Millisecond)
				spin.Suffix = " Scanning infrastructure for drift..."
				spin.Start()
			} else if !quiet {
				ui.Info(fmt.Sprintf("Scanning workspace %q (%s)...", opts.Workspace, opts.Provider))
			}

			adapter, err := cloud.NewAdapterFromScanOptions(ctx, opts)
			if err != nil {
				if spin != nil {
					spin.Stop()
				}
				return err
			}

			runner := scan.NewRunner(nil, adapter)
			report, err := runner.Run(ctx, opts)
			if spin != nil {
				spin.Stop()
			}
			if err != nil {
				return err
			}

			if len(webhookURLs) > 0 {
				if err := notify.NewClient().NotifyDrift(ctx, webhookURLs, report); err != nil {
					ui.Warn(fmt.Sprintf("webhook notification failed: %v", err))
				}
			}

			savePath := dbPath
			if persist {
				if savePath == "" {
					savePath = "drift.db"
				}
				if globalConfigPath != "" {
					if cfg, err := config.Load(globalConfigPath); err == nil && cfg.Server.Database != "" && savePath == "drift.db" {
						savePath = cfg.Server.Database
					}
				}
				s, err := store.Open(savePath)
				if err != nil {
					return err
				}
				defer s.Close()
				if err := s.SaveReport(report); err != nil {
					return err
				}
				if !quiet {
					ui.Success(fmt.Sprintf("Scan saved to %s (id: %s)", savePath, report.ScanID))
				}
			}

			return renderReport(report, output)
		},
	}

	cmd.Flags().StringVar(&stateFile, "state-file", "", "Path to local Terraform state JSON file")
	cmd.Flags().StringVar(&stateBucket, "state-bucket", "", "S3 bucket for remote Terraform state")
	cmd.Flags().StringVar(&stateKey, "state-key", "", "S3 object key for remote Terraform state")
	cmd.Flags().StringVar(&stateRegion, "state-region", "", "AWS region for S3 state backend")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace name (from config or default)")
	cmd.Flags().StringVar(&provider, "provider", "", "Cloud provider: aws, azure, gcp")
	cmd.Flags().StringVar(&region, "region", "", "Cloud region for API calls")
	cmd.Flags().StringVar(&subscriptionID, "subscription-id", "", "Azure subscription ID")
	cmd.Flags().StringVar(&projectID, "project-id", "", "GCP project ID")
	cmd.Flags().StringVar(&output, "output", "rich", "Output format: rich, json, table")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path for persisting scan results")
	cmd.Flags().BoolVar(&persist, "persist", false, "Persist scan results to the database")
	cmd.Flags().BoolVar(&detectUnmanaged, "detect-unmanaged", false, "Detect cloud resources not in Terraform state")
	cmd.Flags().StringSliceVar(&ignoreRules, "ignore", nil, "Additional attribute names to ignore")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "Suppress progress output")

	return cmd
}

func resolveScanOptions(workspace, stateFile, stateBucket, stateKey, stateRegion,
	provider, region, subscriptionID, projectID string,
	detectUnmanaged bool, ignoreRules []string, configPath string) (models.ScanOptions, []string, error) {

	var webhookURLs []string
	opts := models.ScanOptions{
		IgnoreRules:     ignoreRules,
		DetectUnmanaged: detectUnmanaged,
	}

	if configPath != "" {
		cfg, err := config.Load(configPath)
		if err != nil {
			return opts, nil, err
		}
		webhookURLs = cfg.ActiveWebhooks()
		if workspace == "" && len(cfg.Workspaces) == 1 {
			workspace = cfg.Workspaces[0].Name
		}
		if workspace != "" {
			ws, err := cfg.GetWorkspace(workspace)
			if err != nil {
				return opts, nil, err
			}
			opts = ws.ToScanOptions(cfg.Drift)
			opts.IgnoreRules = append(opts.IgnoreRules, ignoreRules...)
			if detectUnmanaged {
				opts.DetectUnmanaged = true
			}
		}
		opts.Concurrency = cfg.Drift.Concurrency
	}

	if workspace != "" {
		opts.Workspace = workspace
	} else if opts.Workspace == "" {
		opts.Workspace = "default"
	}
	if provider != "" {
		opts.Provider = provider
	}
	if region != "" {
		opts.Region = region
	}
	if subscriptionID != "" {
		opts.SubscriptionID = subscriptionID
	}
	if projectID != "" {
		opts.ProjectID = projectID
	}
	if detectUnmanaged {
		opts.DetectUnmanaged = true
	}
	if len(ignoreRules) > 0 {
		opts.IgnoreRules = ignoreRules
	}

	if stateBucket != "" {
		opts.State = models.StateSource{
			Type:   "s3",
			Bucket: stateBucket,
			Key:    stateKey,
			Region: stateRegion,
		}
	} else if stateFile != "" {
		opts.StateFile = stateFile
		opts.State = models.StateSource{Type: "local", Path: stateFile}
	}

	if opts.State.Type == "" && opts.StateFile == "" && opts.State.Path == "" && opts.State.Bucket == "" {
		return opts, webhookURLs, fmt.Errorf("state source required: use --state-file, --state-bucket/--state-key, or --config with a workspace")
	}
	if opts.Provider == "" {
		opts.Provider = "aws"
	}

	_ = backend.Describe(opts.State, opts.StateFile)
	return opts, webhookURLs, nil
}

func isRichOutput(output string) bool {
	return output == "" || output == "rich"
}
