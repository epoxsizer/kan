package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const helpText = `NAVIGATION
  j/k/arrows     items; h/l columns; g/G first/last
  Enter / Esc    open / back or close
EDITING
  a/e/D          add / edit / confirmed delete
  d/m            describe / edit comments
  c/E/X          add / edit / delete a column
  H/L S-Tab/Tab   move card left / right
  J/K            move card down / up in its column
BOARD VIEW
  /              live FTS filter; Ctrl-U clears
  s/v            cycle sort / grouping
FORMS
  Tab/Shift-Tab  next / previous field
  Enter/Ctrl-S   open field / save; Esc cancels
GENERAL
  ?              toggle help
  :              fuzzy search; settings; layout table/cards
  q/Ctrl-C       quit`

func (model *Model) renderHelpText() string {
	headings := map[string]lipgloss.Color{
		"NAVIGATION": namedColors["blue"],
		"EDITING":    namedColors["green"],
		"BOARD VIEW": namedColors["yellow"],
		"FORMS":      namedColors["purple"],
		"GENERAL":    namedColors["orange"],
	}
	lines := strings.Split(helpText, "\n")
	for index, line := range lines {
		if color, ok := headings[line]; ok {
			lines[index] = lipgloss.NewStyle().Bold(true).Faint(true).Foreground(color).Render(line)
		}
	}
	return strings.Join(lines, "\n")
}
