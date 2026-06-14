package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/store"
)

// NewReportCommand creates report list/show subcommands.
func NewReportCommand() *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "report",
		Short: "List and show persisted drift reports",
	}
	cmd.PersistentFlags().StringVar(&dbPath, "db", "drift.db", "SQLite database path")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List persisted scan reports",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store.Open(dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			scans, err := s.ListScans(50)
			if err != nil {
				return err
			}
			return renderReportList(scans)
		},
	}

	showCmd := &cobra.Command{
		Use:   "show [scan-id]",
		Short: "Show a persisted scan report as JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store.Open(dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			report, err := s.GetReport(args[0])
			if err != nil {
				return err
			}
			return renderReport(report, "json")
		},
	}

	cmd.AddCommand(listCmd, showCmd)
	return cmd
}

func renderReportList(scans []store.ScanMeta) error {
	if len(scans) == 0 {
		fmt.Println("No scans found.")
		return nil
	}
	fmt.Printf("%-38s %-12s %-8s %-8s %-8s %s\n", "SCAN ID", "WORKSPACE", "DRIFTED", "MISSING", "MODIFIED", "STARTED")
	fmt.Println("--------------------------------------------------------------------------------------------------------")
	for _, s := range scans {
		fmt.Printf("%-38s %-12s %-8d %-8d %-8d %s\n",
			s.ScanID,
			s.Workspace,
			s.Summary.Drifted,
			s.Summary.Missing,
			s.Summary.Modified,
			s.StartedAt.Format("2006-01-02 15:04:05"),
		)
	}
	return nil
}
