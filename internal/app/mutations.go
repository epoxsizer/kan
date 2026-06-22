package app

import tea "github.com/charmbracelet/bubbletea"

func mutationCommand(scope screen, notice string, action func() error) tea.Cmd {
	return func() tea.Msg {
		return mutationDoneMsg{scope: scope, notice: notice, err: action()}
	}
}
