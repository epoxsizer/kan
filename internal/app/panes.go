package app

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type paneFocus uint8

const (
	boardFocus paneFocus = iota
	inspectorFocus
	navigationFocus
)

type activityEntry struct {
	at             time.Time
	action, cardID string
}

type workspaceState struct {
	focus                            paneFocus
	navigation, inspector            bool
	navigationIndex, inspectorOffset int
	activity                         []activityEntry
}

func selectionText(value string, selected bool) string {
	if os.Getenv("NO_COLOR") == "" {
		return value
	}
	if selected {
		return "> " + value
	}
	return "  " + value
}

func (model *Model) cyclePaneFocus(reverse bool) {
	width, height := model.dimensions()
	layout := model.workspaceLayout(width, height)
	panes := []paneFocus{boardFocus}
	if layout.inspectorWidth > 0 {
		panes = append(panes, inspectorFocus)
	}
	if layout.navigationWidth > 0 {
		panes = append(panes, navigationFocus)
	}
	index := 0
	for i, pane := range panes {
		if pane == model.workspace.focus {
			index = i
		}
	}
	delta := 1
	if reverse {
		delta = -1
	}
	model.workspace.focus = panes[(index+delta+len(panes))%len(panes)]
}

func (model *Model) normalizePaneFocus() {
	width, height := model.dimensions()
	layout := model.workspaceLayout(width, height)
	if model.workspace.focus == inspectorFocus && layout.inspectorWidth == 0 || model.workspace.focus == navigationFocus && layout.navigationWidth == 0 {
		model.workspace.focus = boardFocus
	}
}

func (model *Model) renderPane(title string, lines []string, width, height int, focused bool) string {
	innerWidth := max(width-4, 1)
	rows := max(height-3, 1)
	style := model.styles.panel
	if focused {
		style = model.styles.focusedPanel.Copy().Border(lipgloss.DoubleBorder())
	}
	content := []string{model.styles.header.Render(truncate(title, innerWidth))}
	for _, line := range lines[:min(len(lines), rows-1)] {
		content = append(content, truncate(line, innerWidth))
	}
	return style.Width(max(width-2, 1)).Height(max(height-2, 1)).Render(strings.Join(content, "\n"))
}

func (model *Model) renderNavigation(width, height int) string {
	lines := []string{model.styles.subtle.Render("Projects  [p]"), ""}
	if model.project != nil {
		lines = append(lines, truncate(model.project.Name, max(width-4, 1)))
	}
	visible := max(height-12, 1)
	start := min(max(model.workspace.navigationIndex-visible+1, 0), max(len(model.boards)-visible, 0))
	for index := start; index < min(start+visible, len(model.boards)); index++ {
		board := model.boards[index]
		prefix := "  "
		if model.board != nil && board.ID == model.board.ID {
			prefix = "› "
		}
		line := prefix + truncate(board.Name, max(width-6, 1))
		if model.workspace.focus == navigationFocus && index == model.workspace.navigationIndex {
			line = model.styles.selectedCard.Copy().Padding(0).Render(line)
		}
		lines = append(lines, line)
	}
	lines = append(lines, "", "Views")
	for index, label := range []string{"Today", "Overdue", "Blocked"} {
		selected := model.workspace.focus == navigationFocus && model.workspace.navigationIndex == len(model.boards)+index
		line := selectionText(label, selected)
		if selected {
			line = model.styles.selectedCard.Copy().Padding(0).Render(line)
		}
		lines = append(lines, line)
	}
	return model.renderPane("Boards", lines, width, height, model.workspace.focus == navigationFocus)
}

func (model *Model) clickNavigation(event tea.MouseEvent) (tea.Model, tea.Cmd) {
	model.workspace.focus = navigationFocus
	if event.Y == 3 {
		model.screen = projectsScreen
		model.workspace.focus = boardFocus
		return model, nil
	}
	_, height := model.dimensions()
	visible := max((height-4)-12, 1)
	start := min(max(model.workspace.navigationIndex-visible+1, 0), max(len(model.boards)-visible, 0))
	index := event.Y - 6 + start
	if index >= start && index < min(start+visible, len(model.boards)) {
		if model.workspace.navigationIndex == index {
			model.boardIndex = index
			return model.openSelectedBoard()
		}
		model.workspace.navigationIndex = index
	}
	shown := min(start+visible, len(model.boards)) - start
	viewIndex := event.Y - (8 + shown)
	if viewIndex >= 0 && viewIndex < 3 {
		target := len(model.boards) + viewIndex
		if target == model.workspace.navigationIndex {
			return model.openNavigationSelection()
		}
		model.workspace.navigationIndex = target
	}
	return model, nil
}

func (model *Model) openNavigationSelection() (tea.Model, tea.Cmd) {
	index := model.workspace.navigationIndex
	model.workspace.focus = boardFocus
	if index < len(model.boards) {
		model.boardIndex = index
		return model.openSelectedBoard()
	}
	commands := []string{"today", "overdue", "blocked"}
	if index-len(model.boards) < len(commands) {
		return model.executeCommand(commands[index-len(model.boards)])
	}
	return model, nil
}

func (model *Model) inspectorLines(width int) []string {
	if len(model.columns) == 0 {
		return []string{"No column selected"}
	}
	columnID, cardID := model.selectedIDs()
	if cardID == "" {
		return []string{"No card selected", "a  Add a card"}
	}
	for _, card := range model.visibleCards(columnID) {
		if card.ID == cardID {
			column := model.columns[model.columnIndex]
			priority := "none"
			if card.Priority != nil {
				priority = *card.Priority
			}
			due := "No due date"
			if card.DueDate != nil {
				due = card.DueDate.Format("2006-01-02")
			}
			summary := []string{card.Title, "", "Status: " + column.Name, "Priority: " + priority, "Due: " + due}
			if label := timeInColumnLabel(card, time.Now()); label != "" {
				summary = append(summary, "In column: "+label)
			}
			if card.AgentStatus != "" {
				summary = append(summary, "Agent: "+agentStatusLabel(card))
				if strings.TrimSpace(card.AgentResult) != "" {
					summary = append(summary, wrappedDetailLines(strings.Split(card.AgentResult, "\n"), width)...)
				}
			}
			summary = append(summary, "Tags: "+strings.Join(card.Tags, ", "), "")
			lines := wrappedDetailLines(summary, width)
			if card.Description != "" {
				lines = append(lines, "Description")
				rendered, err := renderMarkdown(card.Description, width)
				if err == nil {
					lines = append(lines, rendered...)
				} else {
					lines = append(lines, wrappedDetailLines(strings.Split(card.Description, "\n"), width)...)
				}
			}
			if len(card.Checklist) > 0 {
				lines = append(lines, "", "Checklist")
				for _, item := range card.Checklist {
					mark := "[ ]"
					if item.Done {
						mark = "[x]"
					}
					lines = append(lines, wrappedDetailLines([]string{mark + " " + item.Text}, width)...)
				}
			}
			keys := []string{}
			for key := range card.Fields {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			if len(keys) > 0 {
				lines = append(lines, "", "Fields")
			}
			for _, key := range keys {
				lines = append(lines, wrappedDetailLines([]string{fmt.Sprintf("%s: %v", key, card.Fields[key].Value)}, width)...)
			}
			if len(card.RelatedCardIDs) > 0 {
				lines = append(lines, "", fmt.Sprintf("Related cards: %d", len(card.RelatedCardIDs)))
			}
			lines = append(lines, "", "d  Full details and IDs")
			return lines
		}
	}
	return nil
}

func (model *Model) renderInspector(width, height int) string {
	lines := model.inspectorLines(max(width-4, 1))
	rows := inspectorRows(height)
	offset := min(model.workspace.inspectorOffset, max(len(lines)-rows, 0))
	visible := append([]string{}, lines[offset:min(offset+rows, len(lines))]...)
	visible = append(visible, "", fmt.Sprintf("%d–%d/%d · e edit · i hide", min(offset+1, len(lines)), min(offset+rows, len(lines)), len(lines)))
	return model.renderPane("Inspector", visible, width, height, model.workspace.focus == inspectorFocus)
}

func inspectorRows(bodyHeight int) int { return max(bodyHeight-6, 1) }

func (model *Model) scrollInspector(delta int) {
	width, height := model.dimensions()
	layout := model.workspaceLayout(width, height)
	lines := model.inspectorLines(max(layout.inspectorWidth-4, 1))
	model.workspace.inspectorOffset = min(max(model.workspace.inspectorOffset+delta, 0), max(len(lines)-inspectorRows(height-4), 0))
}

func (model *Model) handlePaneKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if model.workspace.focus == inspectorFocus {
		switch key.String() {
		case "j", "down":
			model.scrollInspector(1)
		case "k", "up":
			model.scrollInspector(-1)
		case "pgdown":
			model.scrollInspector(max(model.height-9, 1))
		case "pgup":
			model.scrollInspector(-max(model.height-9, 1))
		case "home", "g":
			model.workspace.inspectorOffset = 0
		case "end", "G":
			model.scrollInspector(int(^uint(0) >> 2))
		case "e":
			return model.openSelectedCardEdit()
		case "enter", "esc":
			model.workspace.focus = boardFocus
		}
		return model, nil
	}
	switch key.String() {
	case "j", "down":
		model.workspace.navigationIndex = clampIndex(model.workspace.navigationIndex+1, len(model.boards)+3)
	case "k", "up":
		model.workspace.navigationIndex = clampIndex(model.workspace.navigationIndex-1, len(model.boards)+3)
	case "home", "g":
		model.workspace.navigationIndex = 0
	case "end", "G":
		model.workspace.navigationIndex = len(model.boards) + 2
	case "p":
		model.screen = projectsScreen
		model.workspace.focus = boardFocus
	case "enter":
		return model.openNavigationSelection()
	case "esc":
		model.workspace.focus = boardFocus
	}
	return model, nil
}

func (model *Model) recordActivity(change ExternalChangeMsg) {
	model.workspace.activity = append(model.workspace.activity, activityEntry{at: time.Now(), action: change.Action, cardID: change.CardID})
	if len(model.workspace.activity) > 50 {
		model.workspace.activity = model.workspace.activity[len(model.workspace.activity)-50:]
	}
}

func (model *Model) openActivity() {
	lines := []string{}
	for index := len(model.workspace.activity) - 1; index >= 0; index-- {
		entry := model.workspace.activity[index]
		line := entry.at.Format("15:04:05") + "  MCP: " + entry.action
		if entry.cardID != "" {
			line += "  " + entry.cardID
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		lines = append(lines, "No agent changes in this session.")
	}
	model.detail = &detailPopup{kind: "activity", title: "Agent activity", lines: lines}
}
