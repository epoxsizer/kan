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
  d/m            details / description; Shift+E detail size
  c/E/X          add / configure / delete a column
  H/L/Tab        move; M choose column; u undo
  J/K            move card down / up in its column
BOARD VIEW
  /              fuzzy filter; Ctrl-U clear
  s/v            cycle sort / grouping
FORMS
  Tab fields · Ctrl-P preview · Ctrl-F find · Ctrl-Z/Y undo · Ctrl-G editor
GENERAL
  Mouse          click/open · wheel · right-click back
  ? / :          help / search and settings
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
