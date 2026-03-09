package cli

import (
	"github.com/spf13/cobra"
)

func newCompletionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [command]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for modeltap.

To install completions for your shell, run the appropriate subcommand
and redirect the output to the correct location:

  # Bash
  modeltap completion bash > /etc/bash_completion.d/modeltap

  # Zsh
  modeltap completion zsh > "${fpath[1]}/_modeltap"

  # Fish
  modeltap completion fish > ~/.config/fish/completions/modeltap.fish

  # PowerShell
  modeltap completion powershell > modeltap.ps1
  # Then dot-source the file in your PowerShell profile.
`,
	}

	cmd.AddCommand(
		newCompletionBashCommand(),
		newCompletionZshCommand(),
		newCompletionFishCommand(),
		newCompletionPowershellCommand(),
	)

	return cmd
}

func newCompletionBashCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "bash",
		Short: "Generate bash completion script",
		Long: `Generate the autocompletion script for bash.

To load completions in your current shell session:

  source <(modeltap completion bash)

To install completions permanently:

  # Linux
  modeltap completion bash > /etc/bash_completion.d/modeltap

  # macOS (using Homebrew)
  modeltap completion bash > $(brew --prefix)/etc/bash_completion.d/modeltap
`,
		DisableFlagsInUseLine: true,
		Args:                  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Root().GenBashCompletion(cmd.OutOrStdout())
		},
	}
}

func newCompletionZshCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "zsh",
		Short: "Generate zsh completion script",
		Long: `Generate the autocompletion script for zsh.

To load completions in your current shell session:

  source <(modeltap completion zsh)

To install completions permanently, place the output in your fpath:

  modeltap completion zsh > "${fpath[1]}/_modeltap"

You may need to start a new shell for completions to take effect.
`,
		DisableFlagsInUseLine: true,
		Args:                  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
		},
	}
}

func newCompletionFishCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "fish",
		Short: "Generate fish completion script",
		Long: `Generate the autocompletion script for fish.

To load completions in your current shell session:

  modeltap completion fish | source

To install completions permanently:

  modeltap completion fish > ~/.config/fish/completions/modeltap.fish

You may need to start a new shell for completions to take effect.
`,
		DisableFlagsInUseLine: true,
		Args:                  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true)
		},
	}
}

func newCompletionPowershellCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "powershell",
		Short: "Generate powershell completion script",
		Long: `Generate the autocompletion script for PowerShell.

To load completions in your current shell session:

  modeltap completion powershell | Out-String | Invoke-Expression

To install completions permanently, add the output to your PowerShell profile.
`,
		DisableFlagsInUseLine: true,
		Args:                  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Root().GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
		},
	}
}
