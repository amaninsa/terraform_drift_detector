package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/cli/ui"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/models"
)

func renderReport(report *models.DriftReport, format string) error {
	switch strings.ToLower(format) {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	case "table":
		printTable(report)
		return nil
	case "rich":
		printRich(report)
		return nil
	default:
		return fmt.Errorf("unsupported output format %q (use json, table, or rich)", format)
	}
}

func printRich(report *models.DriftReport) {
	fmt.Println(ui.SummaryBox(report))
	fmt.Println()
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"", "Address", "Type", "Status", "Message"})
	table.SetBorder(false)
	table.SetColumnSeparator("")
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	for _, item := range report.Items {
		table.Append([]string{
			ui.StatusIcon(item.DriftType),
			truncate(item.Address, 36),
			truncate(item.Type, 24),
			ui.DriftLabel(item.DriftType),
			truncate(item.Message, 48),
		})
	}
	table.Render()

	drifted := report.Summary.Drifted
	if drifted == 0 {
		ui.Success("No drift detected — infrastructure matches Terraform state")
	} else {
		ui.Warn(fmt.Sprintf("%d resource(s) drifted from Terraform state", drifted))
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
