package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

const version = "1.1.0"

// NewVersionCommand prints version information.
func NewVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("drift %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
		},
	}
}
