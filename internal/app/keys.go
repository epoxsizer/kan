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
  d/m            details / comments; Ctrl-G editor
  c/E/X          add / edit / delete a column
  H/L/Tab        move; M choose column; u undo
  J/K            move card down / up in its column
BOARD VIEW
  /              live FTS filter; Ctrl-U deletes left
  s/v            cycle sort / grouping
FORMS
  ←/→ Home/End   cursor; Ctrl-W/U/K delete text
  Tab fields · Enter open · Ctrl-S save · Esc cancel
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
