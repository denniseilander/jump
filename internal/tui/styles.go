package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorGold   = lipgloss.Color("#FFB700")
	colorCyan   = lipgloss.Color("#00B4D8")
	colorMint   = lipgloss.Color("#80FFDB")
	colorYellow = lipgloss.Color("#FFD166")
	colorGray   = lipgloss.Color("#888888")
	colorRed    = lipgloss.Color("#FF6B6B")

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(colorGold)
	dimStyle   = lipgloss.NewStyle().Foreground(colorGray)
	labelStyle = lipgloss.NewStyle().Foreground(colorYellow)

	selectedAliasStyle = lipgloss.NewStyle().Bold(true).Foreground(colorMint)
	selectedRowBg      = lipgloss.NewStyle().Background(lipgloss.Color("236"))
	cursorGlyph        = lipgloss.NewStyle().Foreground(colorGold).Bold(true).Render("❯")

	envProdStyle = lipgloss.NewStyle().Background(colorRed).Foreground(lipgloss.Color("#1a0000")).Padding(0, 1).Bold(true)
	envAccStyle  = lipgloss.NewStyle().Background(colorYellow).Foreground(lipgloss.Color("#1a0a00")).Padding(0, 1)
	envDevStyle  = lipgloss.NewStyle().Background(lipgloss.Color("#0d2d2d")).Foreground(colorCyan).Padding(0, 1)
	envTestStyle = lipgloss.NewStyle().Background(colorCyan).Foreground(lipgloss.Color("#001a1a")).Padding(0, 1)

	previewCmdStyle = lipgloss.NewStyle().Foreground(colorMint)
	statusOkStyle   = lipgloss.NewStyle().Foreground(colorMint)
	statusErrStyle  = lipgloss.NewStyle().Foreground(colorRed)

	cmdAddStyle    = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	cmdEditStyle   = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
	cmdDeleteStyle = lipgloss.NewStyle().Foreground(colorRed).Bold(true)

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))
)
