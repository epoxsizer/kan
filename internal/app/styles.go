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
	body         lipgloss.Style
	header       lipgloss.Style
	subtle       lipgloss.Style
	selected     lipgloss.Style
	tableHeader  lipgloss.Style
	panel        lipgloss.Style
	focusedPanel lipgloss.Style
	card         lipgloss.Style
	selectedCard lipgloss.Style
	status       lipgloss.Style
	statusAccent lipgloss.Style
	shortcutKey  lipgloss.Style
	shortcutText lipgloss.Style
	help         lipgloss.Style
	error        lipgloss.Style
	command      lipgloss.Style
}

type Theme struct {
	Primary            string
	Muted              string
	Text               string
	Background         string
	SelectedForeground string
	SelectedBackground string
	Danger             string
	Border             string
}

func DefaultTheme() Theme {
	return Theme{Primary: "#7D7AFF", Muted: "#909090", Text: "#C4C4D0", Background: "#24243A", SelectedForeground: "#FFFFFF", SelectedBackground: "#5A56E0", Danger: "#FF6B6B", Border: "rounded"}
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
	border := borderForName(theme.Border)

	return styles{
		body:         lipgloss.NewStyle().Foreground(text),
		header:       lipgloss.NewStyle().Bold(true).Foreground(primary),
		subtle:       lipgloss.NewStyle().Foreground(muted),
		selected:     lipgloss.NewStyle().Bold(true).Foreground(selected).Background(selectedBackground).Padding(0, 1),
		tableHeader:  lipgloss.NewStyle().Bold(true).Foreground(muted),
		panel:        lipgloss.NewStyle().Border(border).BorderForeground(muted).Padding(0, 1),
		focusedPanel: lipgloss.NewStyle().Border(border).BorderForeground(primary).Padding(0, 1),
		card:         lipgloss.NewStyle().Foreground(text).Padding(0, 1),
		selectedCard: lipgloss.NewStyle().Bold(true).Foreground(selected).Background(selectedBackground).Padding(0, 1),
		status:       lipgloss.NewStyle().Foreground(muted).Background(background).Padding(0, 1),
		statusAccent: lipgloss.NewStyle().Bold(true).Foreground(selected).Background(primary).Padding(0, 1),
		shortcutKey:  lipgloss.NewStyle().Bold(true).Foreground(selected).Background(selectedBackground),
		shortcutText: lipgloss.NewStyle().Foreground(muted).Background(background),
		help:         lipgloss.NewStyle().Foreground(text).Border(lipgloss.DoubleBorder()).BorderForeground(primary).Padding(1, 2),
		error:        lipgloss.NewStyle().Foreground(danger).Bold(true),
		command:      lipgloss.NewStyle().Foreground(primary).Bold(true),
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
