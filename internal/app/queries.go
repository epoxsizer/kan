package app

import tea "github.com/charmbracelet/bubbletea"

type formLoadedMsg struct {
	form    *formModal
	message tea.Msg
}

func forForm(form *formModal, command tea.Cmd) tea.Cmd {
	return func() tea.Msg { return formLoadedMsg{form: form, message: command()} }
}

type searchLoadedMsg struct {
	generation uint64
	boardID    string
	message    tea.Msg
}

func (model *Model) searchBoardCards(query string) tea.Cmd {
	model.searchGeneration++
	generation, boardID := model.searchGeneration, model.board.ID
	command := searchBoardCards(model.ctx, model.repo, boardID, query)
	return func() tea.Msg { return searchLoadedMsg{generation: generation, boardID: boardID, message: command()} }
}

// Every workspace load carries its navigation generation. Results from older
// requests are ignored, including requests for a board that was left and reopened.
type workspaceLoadedMsg struct {
	generation uint64
	screen     screen
	message    tea.Msg
}

func (model *Model) workspaceQuery(scope screen, command tea.Cmd) tea.Cmd {
	model.loadGeneration++
	generation := model.loadGeneration
	return func() tea.Msg {
		return workspaceLoadedMsg{generation: generation, screen: scope, message: command()}
	}
}

func (model *Model) loadProjects() tea.Cmd {
	return model.workspaceQuery(projectsScreen, loadProjects(model.ctx, model.repo))
}

func (model *Model) loadBoards(projectID string) tea.Cmd {
	return model.workspaceQuery(boardsScreen, loadBoards(model.ctx, model.repo, projectID))
}

func (model *Model) loadBoard(boardID string) tea.Cmd {
	return model.workspaceQuery(boardScreen, loadBoard(model.ctx, model.repo, boardID))
}
