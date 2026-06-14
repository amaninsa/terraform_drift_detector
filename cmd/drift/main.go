package main

import (
	"os"

	"github.com/terraform-drift-detector/terraform_drift_detector/internal/cli"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
