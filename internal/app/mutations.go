package app

import tea "github.com/charmbracelet/bubbletea"

func mutationCommand(scope screen, notice string, action func() error) tea.Cmd {
	return func() tea.Msg {
		return mutationDoneMsg{scope: scope, notice: notice, err: action()}
	}
}

func (model *Model) saveFormCommand(scope screen, notice string, action func() error) tea.Cmd {
	form := model.form
	form.saving = true
	form.err = ""
	return func() tea.Msg {
		return mutationDoneMsg{scope: scope, notice: notice, err: action(), form: form}
	}
}
