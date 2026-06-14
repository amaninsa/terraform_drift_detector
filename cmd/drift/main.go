package main

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/cli"
)

func main() {
	root := &cobra.Command{
		Use:   "drift",
		Short: "Terraform drift detection — compare state against live cloud infrastructure",
	}
	root.AddCommand(cli.NewScanCommand())
	root.AddCommand(cli.NewReportCommand())
	root.AddCommand(cli.NewServeCommand())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
