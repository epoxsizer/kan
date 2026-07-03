package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var namedColors = map[string]lipgloss.Color{
	"blue": "#4C8DFF", "green": "#42C77A", "yellow": "#E5C454", "orange": "#F59E42",
	"red": "#F05B62", "purple": "#A879FF", "gray": "#909090",
}

func colorForName(name string) lipgloss.Color {
	if color, ok := namedColors[strings.ToLower(name)]; ok {
		return color
	}
	return namedColors["gray"]
}

func priorityColor(priority string) lipgloss.Color {
	switch strings.ToLower(priority) {
	case "low":
		return namedColors["green"]
	case "medium":
		return namedColors["blue"]
	case "high":
		return namedColors["orange"]
	case "urgent":
		return namedColors["red"]
	default:
		return namedColors["gray"]
	}
}

type styles struct {
	body                     lipgloss.Style
	header                   lipgloss.Style
	subtle                   lipgloss.Style
	selected                 lipgloss.Style
	tableHeader              lipgloss.Style
	panel                    lipgloss.Style
	focusedPanel             lipgloss.Style
	card                     lipgloss.Style
	selectedCard             lipgloss.Style
	status                   lipgloss.Style
	statusAccent             lipgloss.Style
	shortcutKey              lipgloss.Style
	shortcutText             lipgloss.Style
	help                     lipgloss.Style
	error                    lipgloss.Style
	command                  lipgloss.Style
	selectedColumnForeground lipgloss.Color
	selectedColumnBackground lipgloss.Color
	selectedColumnBorder     lipgloss.Color
	columnDefault            lipgloss.Color
}

type Theme struct {
	Primary                  string
	Muted                    string
	Text                     string
	Background               string
	SelectedForeground       string
	SelectedBackground       string
	Danger                   string
	Border                   string
	SelectedColumnForeground string
	SelectedColumnBackground string
	SelectedColumnBorder     string
	SelectedCardForeground   string
	SelectedCardBackground   string
	PanelBorder              string
	FocusedPanelBorder       string
	StatusForeground         string
	StatusBackground         string
	StatusAccentForeground   string
	StatusAccentBackground   string
	ShortcutKeyForeground    string
	ShortcutKeyBackground    string
	ShortcutText             string
	HelpText                 string
	HelpBorder               string
	Command                  string
	ColumnDefault            string
}

func DefaultTheme() Theme {
	return Theme{
		Primary: "#7D7AFF", Muted: "#909090", Text: "#C4C4D0", Background: "#24243A", SelectedForeground: "#000000", SelectedBackground: "#4C8DFF", Danger: "#FF6B6B", Border: "rounded",
		SelectedColumnForeground: "#000000", SelectedColumnBackground: "#4C8DFF", SelectedColumnBorder: "#4C8DFF", SelectedCardForeground: "#000000", SelectedCardBackground: "#4C8DFF",
		PanelBorder: "#909090", FocusedPanelBorder: "#4C8DFF", StatusForeground: "#909090", StatusBackground: "#24243A", StatusAccentForeground: "#FFFFFF", StatusAccentBackground: "#7D7AFF",
		ShortcutKeyForeground: "#FFFFFF", ShortcutKeyBackground: "#5A56E0", ShortcutText: "#909090", HelpText: "#C4C4D0", HelpBorder: "#7D7AFF", Command: "#7D7AFF", ColumnDefault: "#4C8DFF",
	}
}

func defaultStyles() styles { return stylesForTheme(DefaultTheme()) }

func stylesForTheme(theme Theme) styles {
	primary := lipgloss.Color(theme.Primary)
	muted := lipgloss.Color(theme.Muted)
	text := lipgloss.Color(theme.Text)
	background := lipgloss.Color(theme.Background)
	selected := lipgloss.Color(theme.SelectedForeground)
	selectedBackground := lipgloss.Color(theme.SelectedBackground)
	danger := lipgloss.Color(theme.Danger)
	selectedColumnForeground := lipgloss.Color(theme.SelectedColumnForeground)
	selectedColumnBackground := lipgloss.Color(theme.SelectedColumnBackground)
	selectedColumnBorder := lipgloss.Color(theme.SelectedColumnBorder)
	selectedCardForeground := lipgloss.Color(theme.SelectedCardForeground)
	selectedCardBackground := lipgloss.Color(theme.SelectedCardBackground)
	panelBorder := lipgloss.Color(theme.PanelBorder)
	focusedPanelBorder := lipgloss.Color(theme.FocusedPanelBorder)
	statusForeground := lipgloss.Color(theme.StatusForeground)
	statusBackground := lipgloss.Color(theme.StatusBackground)
	statusAccentForeground := lipgloss.Color(theme.StatusAccentForeground)
	statusAccentBackground := lipgloss.Color(theme.StatusAccentBackground)
	shortcutKeyForeground := lipgloss.Color(theme.ShortcutKeyForeground)
	shortcutKeyBackground := lipgloss.Color(theme.ShortcutKeyBackground)
	shortcutText := lipgloss.Color(theme.ShortcutText)
	helpText := lipgloss.Color(theme.HelpText)
	helpBorder := lipgloss.Color(theme.HelpBorder)
	command := lipgloss.Color(theme.Command)
	columnDefault := lipgloss.Color(theme.ColumnDefault)
	border := borderForName(theme.Border)

	return styles{
		body:                     lipgloss.NewStyle().Foreground(text),
		header:                   lipgloss.NewStyle().Bold(true).Foreground(primary),
		subtle:                   lipgloss.NewStyle().Foreground(muted),
		selected:                 lipgloss.NewStyle().Bold(true).Foreground(selected).Background(selectedBackground).Padding(0, 1),
		tableHeader:              lipgloss.NewStyle().Bold(true).Foreground(muted),
		panel:                    lipgloss.NewStyle().Border(border).BorderForeground(panelBorder).Padding(0, 1),
		focusedPanel:             lipgloss.NewStyle().Border(border).BorderForeground(focusedPanelBorder).Padding(0, 1),
		card:                     lipgloss.NewStyle().Foreground(text).Padding(0, 1),
		selectedCard:             lipgloss.NewStyle().Bold(true).Foreground(selectedCardForeground).Background(selectedCardBackground).Padding(0, 1),
		status:                   lipgloss.NewStyle().Foreground(statusForeground).Background(statusBackground).Padding(0, 1),
		statusAccent:             lipgloss.NewStyle().Bold(true).Foreground(statusAccentForeground).Background(statusAccentBackground).Padding(0, 1),
		shortcutKey:              lipgloss.NewStyle().Bold(true).Foreground(shortcutKeyForeground).Background(shortcutKeyBackground),
		shortcutText:             lipgloss.NewStyle().Foreground(shortcutText).Background(background),
		help:                     lipgloss.NewStyle().Foreground(helpText).Border(lipgloss.DoubleBorder()).BorderForeground(helpBorder).Padding(1, 2),
		error:                    lipgloss.NewStyle().Foreground(danger).Bold(true),
		command:                  lipgloss.NewStyle().Foreground(command).Bold(true),
		selectedColumnForeground: selectedColumnForeground,
		selectedColumnBackground: selectedColumnBackground,
		selectedColumnBorder:     selectedColumnBorder,
		columnDefault:            columnDefault,
	}
}

func borderForName(name string) lipgloss.Border {
	switch name {
	case "normal":
		return lipgloss.NormalBorder()
	case "thick":
		return lipgloss.ThickBorder()
	case "double":
		return lipgloss.DoubleBorder()
	default:
		return lipgloss.RoundedBorder()
	}
}
