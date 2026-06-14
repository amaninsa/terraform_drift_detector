package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/fatih/color"
	"github.com/terraform-drift-detector/terraform_drift_detector/internal/models"
)

var (
	NoColor bool

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 2)
)

// Banner returns the CLI header.
func Banner() string {
	logo := `
 ██████╗ ██████╗ ██╗███████╗████████╗
 ██╔══██╗██╔══██╗██║██╔════╝╚══██╔══╝
 ██║  ██║██████╔╝██║█████╗     ██║
 ██║  ██║██╔══██╗██║██╔══╝     ██║
 ██████╔╝██║  ██║██║██║        ██║
 ╚═════╝ ╚═╝  ╚═╝╚═╝╚═╝        ╚═╝`
	if NoColor {
		return logo + "\nTerraform Drift Detector — infrastructure drift, zero terraform plan"
	}
	return titleStyle.Render(logo) + "\n" + subtitleStyle.Render("Terraform Drift Detector — infrastructure drift, zero terraform plan")
}

// DriftLabel returns a colorized drift type label.
func DriftLabel(dt models.DriftType) string {
	label := string(dt)
	if NoColor {
		return label
	}
	switch dt {
	case models.DriftTypeMissing:
		return color.New(color.FgRed).Sprint(label)
	case models.DriftTypeModified:
		return color.New(color.FgYellow).Sprint(label)
	case models.DriftTypeTagOnly:
		return color.New(color.FgBlue).Sprint(label)
	case models.DriftTypeUnmanaged:
		return color.New(color.FgMagenta).Sprint(label)
	case models.DriftTypeFetchErr:
		return color.New(color.FgHiBlack).Sprint(label)
	case models.DriftTypeInSync:
		return color.New(color.FgGreen).Sprint(label)
	default:
		return label
	}
}

// SummaryBox renders scan summary statistics.
func SummaryBox(report *models.DriftReport) string {
	s := report.Summary
	lines := []string{
		fmt.Sprintf("Scan ID    %s", report.ScanID),
		fmt.Sprintf("Workspace  %s", report.Workspace),
		fmt.Sprintf("Provider   %s", report.Provider),
		fmt.Sprintf("State      %s", report.StateSource),
		fmt.Sprintf("Duration   %s", report.Duration),
		"",
		fmt.Sprintf("Total      %d", s.TotalResources),
		fmt.Sprintf("In Sync    %d", s.InSync),
		fmt.Sprintf("Drifted    %d", s.Drifted),
		fmt.Sprintf("  Missing    %d", s.Missing),
		fmt.Sprintf("  Modified   %d", s.Modified),
		fmt.Sprintf("  Tag Only   %d", s.TagOnly),
		fmt.Sprintf("  Unmanaged  %d", s.Unmanaged),
		fmt.Sprintf("  Errors     %d", s.FetchErrors),
	}
	content := strings.Join(lines, "\n")
	if NoColor {
		return content
	}
	return boxStyle.Render(content)
}

// StatusIcon returns a status indicator for drift types.
func StatusIcon(dt models.DriftType) string {
	switch dt {
	case models.DriftTypeInSync:
		return "✓"
	case models.DriftTypeMissing:
		return "✗"
	case models.DriftTypeModified:
		return "!"
	case models.DriftTypeTagOnly:
		return "~"
	case models.DriftTypeUnmanaged:
		return "+"
	default:
		return "?"
	}
}

// Info prints an info message.
func Info(msg string) {
	if NoColor {
		fmt.Fprintf(os.Stderr, "info: %s\n", msg)
		return
	}
	c := color.New(color.FgCyan)
	c.Fprintf(os.Stderr, "→ %s\n", msg)
}

// Success prints a success message.
func Success(msg string) {
	if NoColor {
		fmt.Fprintf(os.Stderr, "ok: %s\n", msg)
		return
	}
	c := color.New(color.FgGreen)
	c.Fprintf(os.Stderr, "✓ %s\n", msg)
}

// Warn prints a warning message.
func Warn(msg string) {
	if NoColor {
		fmt.Fprintf(os.Stderr, "warn: %s\n", msg)
		return
	}
	c := color.New(color.FgYellow)
	c.Fprintf(os.Stderr, "⚠ %s\n", msg)
}

// Error prints an error message.
func Error(msg string) {
	if NoColor {
		fmt.Fprintf(os.Stderr, "error: %s\n", msg)
		return
	}
	c := color.New(color.FgRed)
	c.Fprintf(os.Stderr, "✗ %s\n", msg)
}
