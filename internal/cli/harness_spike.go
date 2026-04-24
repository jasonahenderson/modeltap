package cli

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/harnessspike"
	"github.com/spf13/cobra"
)

func newHarnessSpikeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "harness-spike",
		Short: "Launch the replacement-harness spike with a fake backend",
		Long: `Launch an isolated Bubble Tea spike shell for evaluating the replacement
terminal UI. This path uses fake streaming replies and no real BFF
integration, so layout and interaction can be judged independently of
the current harness architecture.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app := harnessspike.New()
			p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseAllMotion())
			_, err := p.Run()
			return err
		},
	}
	return cmd
}
