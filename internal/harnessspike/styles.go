package harnessspike

import "github.com/charmbracelet/lipgloss"

var (
	pageBg = lipgloss.Color("#101318")

	sidebarBoxStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Background(lipgloss.Color("#121923")).
			Foreground(lipgloss.Color("#D9E1EC")).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("#2A3648"))

	sidebarTitleStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F5F7FB"))
	sidebarSectionStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#79C0FF"))
	sidebarItemStyle        = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("#C8D1DC"))
	sidebarItemActiveStyle  = lipgloss.NewStyle().Padding(0, 1).Background(lipgloss.Color("#1C2635")).Foreground(lipgloss.Color("#FFFFFF"))
	sidebarItemFocusedStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Background(lipgloss.Color("#1F4B75")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true)
	sidebarValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F5F7FB"))
	sidebarMutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#8EA2BC"))
	sidebarMetaStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#6F86A3")).Bold(true)

	headerBoxStyle = lipgloss.NewStyle().
			Padding(1, 2, 0, 2).
			Background(pageBg)
	headerTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F5F7FB"))
	headerSubtitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#8EA2BC"))

	transcriptBoxStyle = lipgloss.NewStyle().
				Padding(1, 2).
				Background(pageBg)

	inputBoxStyle = lipgloss.NewStyle().
			Padding(0, 2, 1, 2).
			Background(pageBg)
	footerBoxStyle = lipgloss.NewStyle().
			Padding(0, 2, 1, 2).
			Background(pageBg)
	footerStatusStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#D9E1EC"))
	footerHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#79C0FF")).
			Bold(true)
	overlayPanelStyle = lipgloss.NewStyle().
				Padding(1, 2).
				Background(lipgloss.Color("#111821")).
				Foreground(lipgloss.Color("#F5F7FB"))
	overlayHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#F5F7FB"))
	overlaySubheadStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8EA2BC"))
	overlaySectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#79C0FF"))
	overlayItemStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Foreground(lipgloss.Color("#D9E1EC"))
	overlayItemActiveStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Background(lipgloss.Color("#23405F")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true)
	overlayFooterStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8EA2BC"))

	dialogBoxStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Width(48).
			Background(lipgloss.Color("#18212E")).
			Foreground(lipgloss.Color("#F5F7FB")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#79C0FF"))
	dialogTitleStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F5F7FB"))
	dialogPromptStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#C8D1DC"))
	dialogOptionStyle       = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("#D9E1EC"))
	dialogOptionActiveStyle = lipgloss.NewStyle().Padding(0, 1).Background(lipgloss.Color("#23405F")).Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
	dialogHintStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#8EA2BC"))
	modalScrimStyle         = lipgloss.NewStyle().Faint(true)
	dialogDividerStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#2A3648"))
	keycapStyle             = lipgloss.NewStyle().Padding(0, 1).Background(lipgloss.Color("#1F2A38")).Foreground(lipgloss.Color("#D9E1EC"))
	previewBodyStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#E6EDF3"))
	paletteQueryStyle       = lipgloss.NewStyle().Padding(0, 1).Background(lipgloss.Color("#151C25")).Foreground(lipgloss.Color("#F5F7FB"))

	systemStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(lipgloss.Color("#8EA2BC")).
			Background(lipgloss.Color("#171E28"))

	userLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#AAB7C6"))
	userBodyStyle  = lipgloss.NewStyle().
			Padding(0, 1).
			Background(lipgloss.Color("#232B36")).
			Foreground(lipgloss.Color("#E6EDF3"))
	queuedLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#F2CC60"))
	queuedBodyStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Background(lipgloss.Color("#2C2414")).
			Foreground(lipgloss.Color("#F6E7B0"))
	eventInfoStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Background(lipgloss.Color("#1B2230")).
			Foreground(lipgloss.Color("#C8D1DC"))
	eventRequestedStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Background(lipgloss.Color("#1B2230")).
				Foreground(lipgloss.Color("#79C0FF"))
	eventPermissionStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Background(lipgloss.Color("#2C2414")).
				Foreground(lipgloss.Color("#F2CC60"))
	eventRunningStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Background(lipgloss.Color("#1D2631")).
				Foreground(lipgloss.Color("#E6EDF3"))
	eventDoneStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Background(lipgloss.Color("#16241C")).
			Foreground(lipgloss.Color("#7EE787"))

	assistantLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#79C0FF"))
	assistantBodyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#F5F7FB"))
	agentStreamStyle    = lipgloss.NewStyle().
				Padding(0, 1).
				Background(lipgloss.Color("#18212E")).
				Foreground(lipgloss.Color("#E6EDF3"))

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
