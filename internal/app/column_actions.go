package app

import tea "github.com/charmbracelet/bubbletea"

type columnMoveDoneMsg struct {
	columnID string
	err      error
}

func (model *Model) moveSelectedColumn(delta int) (tea.Model, tea.Cmd) {
	if len(model.columns) < 2 {
		return model, nil
	}
	target := model.columnIndex + delta
	if target < 0 {
		model.notice = "Column is already first"
		return model, nil
	}
	if target >= len(model.columns) {
		model.notice = "Column is already last"
		return model, nil
	}
	columnID := model.columns[model.columnIndex].ID
	model.loading = true
	model.err = nil
	return model, func() tea.Msg {
		return columnMoveDoneMsg{
			columnID: columnID,
			err:      model.repo.MoveColumn(model.ctx, columnID, target),
		}
	}
}
