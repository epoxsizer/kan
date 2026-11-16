package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const helpText = `NAVIGATION
  j/k/arrows     items; h/l columns; g/G first/last
  Enter / Esc    open / back or close
EDITING
  a/e/D          add / edit / confirmed delete
  d/m            details / description; Shift+E detail size
  c/E/X columns · ⇧←/→ reorder
  H/L/J/K    move/reorder cards; M choose; u undo
BOARD VIEW
  /              fuzzy filter; :view opens planning and layout actions
  s/v            cycle sort / grouping
  Tab/Shift-Tab  focus panes; H/L/M move cards
FORMS
  Ctrl-S save · Ctrl-R reload card · Tab fields · Ctrl-P preview · Ctrl-F find · Ctrl-T tasks · Ctrl-G editor
GENERAL
  Mouse          click/open · wheel · right-click back
  ? / :          help / hierarchical action menus
  q/Ctrl-C       quit`

func (model *Model) renderHelpText() string {
	headings := map[string]lipgloss.Color{
		"NAVIGATION": namedColors["blue"],
		"EDITING":    namedColors["green"],
		"BOARD VIEW": namedColors["yellow"],
		"FORMS":      namedColors["purple"],
		"GENERAL":    namedColors["orange"],
	}
	text := helpText
	for _, action := range workspaceActions {
		text += "\n  " + action.key + "              " + action.description
	}
	if model.legacyKeymap {
		text += "\n  Legacy keys: Tab/Shift-Tab move cards"
	}
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		if color, ok := headings[line]; ok {
			lines[index] = lipgloss.NewStyle().Bold(true).Faint(true).Foreground(color).Render(line)
		}
	}
	return strings.Join(lines, "\n")
}

func (model *Model) helpLines(width int) []string {
	return strings.Split(ansi.Hardwrap(model.renderHelpText(), max(min(66, width-4)-4, 1), true), "\n")
}

func (model *Model) renderHelp(width, height int) string {
	lines := model.helpLines(width)
	rows := max(height-7, 1)
	start := min(model.helpOffset, max(len(lines)-rows, 0))
	visible := append([]string{}, lines[start:min(start+rows, len(lines))]...)
	visible = append(visible, model.styles.subtle.Render(truncate("j/k scroll · PgUp/PgDn page · Esc close", max(min(66, width-4)-4, 1))))
	popup := model.styles.help.Width(min(66, width-4)).Render(strings.Join(visible, "\n"))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, popup)
}

func (model *Model) handleHelpKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	width, height := model.dimensions()
	switch key.String() {
	case "j", "down":
		model.helpOffset++
	case "k", "up":
		model.helpOffset--
	case "pgdown":
		model.helpOffset += max(height-7, 1)
	case "pgup":
		model.helpOffset -= max(height-7, 1)
	case "g", "home":
		model.helpOffset = 0
	case "G", "end":
		model.helpOffset = len(model.helpLines(width))
	}
	model.helpOffset = min(max(model.helpOffset, 0), max(len(model.helpLines(width))-max(height-7, 1), 0))
	return model, nil
}
