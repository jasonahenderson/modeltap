package harnessshell

// Styles for the reusable conversation shell. Per WU-098 §"Theme/style import
// boundary" and the HIL/Kimi #7 disposition, this file must not import
// internal/harness/theme or any other modeltap-specific style constants.
// Defaults live as plain lipgloss styles so the package can promote out of
// the repository without rewiring theme integration. Hosts that want to
// override visuals should do so via shell-local options rather than reaching
// into modeltap theme code.

import "github.com/charmbracelet/lipgloss"

var (
	splashBoxStyle = lipgloss.NewStyle().
			Padding(1, 1).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#2A3648"))
	splashMarkStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Bold(true).
			Background(lipgloss.Color("#79C0FF")).
			Foreground(lipgloss.Color("#111821"))
	splashTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#F5F7FB"))
	splashSubtitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8EA2BC"))

	composerBoxStyle = lipgloss.NewStyle().
				Padding(1, 1).
				Border(lipgloss.NormalBorder(), true, false, true, false).
				BorderForeground(lipgloss.Color("#2A3648"))
	footerStatusStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#D9E1EC"))
	footerHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#79C0FF")).
			Bold(true)

	dialogBoxStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Width(48).
			Background(lipgloss.Color("#18212E")).
			Foreground(lipgloss.Color("#F5F7FB")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#79C0FF"))
	dialogTitleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F5F7FB"))
	dialogHintStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#8EA2BC"))
	dialogDividerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#2A3648"))
	keycapStyle        = lipgloss.NewStyle().Padding(0, 1).Background(lipgloss.Color("#1F2A38")).Foreground(lipgloss.Color("#D9E1EC"))
	previewBodyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#E6EDF3"))

	systemStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(lipgloss.Color("#8EA2BC"))

	hostInfoStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(lipgloss.Color("#C8D1DC"))

	chromeStatusReadyStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Foreground(lipgloss.Color("#8EA2BC"))
	chromeStatusStreamingStyle = lipgloss.NewStyle().
					Padding(0, 1).
					Foreground(lipgloss.Color("#79C0FF")).
					Bold(true)
	chromeStatusErrorStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Foreground(lipgloss.Color("#F85149")).
				Bold(true)
	chromeStatusPermissionStyle = lipgloss.NewStyle().
					Padding(0, 1).
					Foreground(lipgloss.Color("#F2CC60")).
					Bold(true)
	chromeStatusInterruptStyle = lipgloss.NewStyle().
					Padding(0, 1).
					Foreground(lipgloss.Color("#F85149"))

	userBodyStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(lipgloss.Color("#E6EDF3"))
	queuedLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#F2CC60"))
	queuedBodyStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(lipgloss.Color("#F6E7B0"))
	eventInfoStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(lipgloss.Color("#C8D1DC"))
	eventRequestedStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Foreground(lipgloss.Color("#79C0FF"))
	eventPermissionStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Foreground(lipgloss.Color("#F2CC60"))
	eventRunningStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Foreground(lipgloss.Color("#E6EDF3"))
	eventDoneStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(lipgloss.Color("#7EE787"))
	eventGrantedStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Foreground(lipgloss.Color("#7EE787")).
				Bold(true)
	eventDeniedStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Foreground(lipgloss.Color("#F85149")).
				Bold(true)
	permissionActionsStyle = lipgloss.NewStyle().
				Padding(0, 1)
	permissionPromptStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#F2CC60"))
	permissionDetailsStyle = lipgloss.NewStyle().
				Padding(0, 1)
	permissionLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#F5F7FB"))
	permissionMetaStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8EA2BC"))
	permissionGrantedMetaStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#7EE787")).
					Bold(true)
	permissionActionStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Background(lipgloss.Color("#1D2631")).
				Foreground(lipgloss.Color("#F2CC60"))
	permissionActionActiveStyle = lipgloss.NewStyle().
					Padding(0, 1).
					Background(lipgloss.Color("#F2CC60")).
					Foreground(lipgloss.Color("#111821")).
					Bold(true)

	assistantLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#79C0FF"))
	assistantBodyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#F5F7FB"))

	tokenStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Background(lipgloss.Color("#1A2330")).
			Foreground(lipgloss.Color("#D9E1EC")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#324154"))
	tokenActiveStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Background(lipgloss.Color("#23405F")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#79C0FF"))
	tokenHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8EA2BC"))
	transcriptTokenStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Background(lipgloss.Color("#1D2631")).
				Foreground(lipgloss.Color("#D9E1EC"))
	transcriptTokenActiveStyle = lipgloss.NewStyle().
					Padding(0, 1).
					Background(lipgloss.Color("#23405F")).
					Foreground(lipgloss.Color("#FFFFFF")).
					Bold(true)
	transcriptTokenBlockStyle = lipgloss.NewStyle().
					Margin(0, 0, 1, 0)
	transcriptMetaStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#AAB7C6"))
)
