package harnesshost

// Format helpers for projecting runtime chrome updates into shell
// HostStatusEvent strings. They live in their own file so the
// projection layer remains a list of mappings.

import "fmt"

// formatContextUpdate renders a context-window update as a single
// status line. Per WU-099 the shell uses StatusKind for chrome
// decisions; this format is purely cosmetic.
func formatContextUpdate(pct float64, used, max int) string {
	return fmt.Sprintf("Context: %.0f%% (%d / %d)", pct, used, max)
}

// formatCostUpdate renders a session-cost update as a single status
// line.
func formatCostUpdate(total float64) string {
	return fmt.Sprintf("Cost: $%.4f", total)
}
