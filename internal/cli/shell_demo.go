package cli

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonahenderson/modeltap/internal/harnessdemo"
	"github.com/jasonahenderson/modeltap/internal/harnessshell"
	"github.com/spf13/cobra"
)

// newShellDemoCommand registers the `modeltap shell-demo` subcommand.
// It launches the reusable conversation shell wired through the
// modeltap host adapter (internal/harnesshost) against the fake/demo
// runtime in internal/harnessdemo. The command replaces the legacy
// `modeltap harness-spike` entrypoint per WU-100 Stage E.
func newShellDemoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shell-demo",
		Short: "Launch the reusable conversation shell with a fake runtime",
		Long: `Launch the post-extraction conversation shell with a synthetic backend.
This path runs internal/harnessshell wrapped in internal/harnesshost.Adapter
with internal/harnessdemo.FakeRuntime — useful for evaluating shell layout,
streaming behavior, queue follow-ups, and the permission demo without a
real BFF.

Type a message and press Enter to submit. /perm triggers the permission
request demo. /clear wipes the transcript. Esc once arms an interrupt
during streaming; Esc twice emits the InterruptRunAction.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			shell := harnessshell.New(
				harnessshell.WithLabel("fake-kimi-demo"),
				harnessshell.WithPlaceholder("Ask something. Enter sends. /perm shows the permission demo. /clear wipes the transcript."),
			)
			runtime := harnessdemo.New()
			driver := harnessdemo.NewDriver(shell, runtime)
			p := tea.NewProgram(driver, tea.WithAltScreen(), tea.WithMouseAllMotion())
			_, err := p.Run()
			return err
		},
	}
	return cmd
}
