package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	awsprovider "github.com/terraform-drift-detector/terraform_drift_detector/internal/providers/aws"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/models"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/providers"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/scan"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/store"
)

// NewScanCommand creates the scan subcommand.
func NewScanCommand() *cobra.Command {
	var (
		stateFile   string
		workspace   string
		provider    string
		region      string
		output      string
		dbPath      string
		persist     bool
		ignoreRules []string
	)

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Run a drift scan comparing Terraform state to live cloud resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			opts := models.ScanOptions{
				Workspace:   workspace,
				StateFile:   stateFile,
				Provider:    provider,
				Region:      region,
				IgnoreRules: ignoreRules,
			}

			adapter, err := newAdapter(ctx, provider, region)
			if err != nil {
				return err
			}

			runner := scan.NewRunner(nil, adapter)
			report, err := runner.Run(ctx, opts)
			if err != nil {
				return err
			}

			if persist || dbPath != "" {
				path := dbPath
				if path == "" {
					path = "drift.db"
				}
				s, err := store.Open(path)
				if err != nil {
					return err
				}
				defer s.Close()
				if err := s.SaveReport(report); err != nil {
					return err
				}
			}

			return renderReport(report, output)
		},
	}

	cmd.Flags().StringVar(&stateFile, "state-file", "", "Path to Terraform state JSON file (required)")
	cmd.Flags().StringVar(&workspace, "workspace", "default", "Workspace name for the scan")
	cmd.Flags().StringVar(&provider, "provider", "aws", "Cloud provider (aws)")
	cmd.Flags().StringVar(&region, "region", "", "Cloud region for API calls")
	cmd.Flags().StringVar(&output, "output", "json", "Output format: json, table")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path for persisting scan results")
	cmd.Flags().BoolVar(&persist, "persist", false, "Persist scan results to the database")
	cmd.Flags().StringSliceVar(&ignoreRules, "ignore", nil, "Additional attribute names to ignore during comparison")
	_ = cmd.MarkFlagRequired("state-file")

	return cmd
}

func newAdapter(ctx context.Context, provider, region string) (providers.CloudAdapter, error) {
	switch provider {
	case "aws":
		if region == "" {
			region = os.Getenv("AWS_REGION")
		}
		if region == "" {
			region = "us-east-1"
		}
		return awsprovider.NewAdapter(ctx, region)
	default:
		return nil, fmt.Errorf("unsupported provider %q", provider)
	}
}

func renderReport(report *models.DriftReport, format string) error {
	switch strings.ToLower(format) {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	case "table":
		printTable(report)
		return nil
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func printTable(report *models.DriftReport) {
	fmt.Printf("Scan ID:     %s\n", report.ScanID)
	fmt.Printf("Workspace:   %s\n", report.Workspace)
	fmt.Printf("Provider:    %s\n", report.Provider)
	fmt.Printf("Duration:    %s\n", report.Duration)
	fmt.Printf("Summary:     %d total, %d drifted, %d in sync\n\n",
		report.Summary.TotalResources, report.Summary.Drifted, report.Summary.InSync)
	fmt.Printf("%-40s %-22s %-12s %s\n", "ADDRESS", "TYPE", "DRIFT", "MESSAGE")
	fmt.Println(strings.Repeat("-", 100))
	for _, item := range report.Items {
		fmt.Printf("%-40s %-22s %-12s %s\n",
			truncate(item.Address, 40),
			truncate(item.Type, 22),
			string(item.DriftType),
			truncate(item.Message, 40),
		)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
