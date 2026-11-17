package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

// navPoint is one visited location in the navigation history.
type navPoint struct {
	screen    screen
	projectID string
	boardID   string
}

func (model *Model) currentNavPoint() navPoint {
	point := navPoint{screen: model.screen}
	if model.project != nil {
		point.projectID = model.project.ID
	}
	if model.board != nil {
		point.boardID = model.board.ID
	}
	return point
}

// pushNavOrigin remembers where a transition starts, so the first recorded
// move already has a destination to return to.
func (model *Model) pushNavOrigin() {
	point := model.currentNavPoint()
	if len(model.navHistory) > 0 && model.navHistory[model.navIndex] == point {
		return
	}
	model.navHistory = append(model.navHistory, point)
	model.navIndex = len(model.navHistory) - 1
}

// recordNavDestination appends the new location, dropping any forward tail.
func (model *Model) recordNavDestination() {
	point := model.currentNavPoint()
	model.navHistory = append(model.navHistory[:model.navIndex+1], point)
	if len(model.navHistory) > 50 {
		model.navHistory = model.navHistory[len(model.navHistory)-50:]
	}
	model.navIndex = len(model.navHistory) - 1
}

func (model *Model) navigateHistory(delta int) (tea.Model, tea.Cmd) {
	target := model.navIndex + delta
	if target < 0 || target >= len(model.navHistory) {
		model.notice = "No navigation history"
		return model, nil
	}
	model.navIndex = target
	return model.navigateTo(model.navHistory[target])
}

func (model *Model) navigateTo(point navPoint) (tea.Model, tea.Cmd) {
	model.loadGeneration++
	model.detail = nil
	model.help = false
	switch point.screen {
	case projectsScreen:
		model.screen = projectsScreen
		model.loading = true
		return model, model.loadProjects()
	case boardsScreen:
		for index := range model.projects {
			if model.projects[index].ID == point.projectID {
				model.project = &model.projects[index]
				model.screen = boardsScreen
				model.loading = true
				return model, model.loadBoards(point.projectID)
			}
		}
	case boardScreen:
		for index := range model.boards {
			if model.boards[index].ID == point.boardID {
				selected := model.boards[index]
				model.board = &selected
				for projectIndex := range model.projects {
					if model.projects[projectIndex].ID == selected.ProjectID {
						model.project = &model.projects[projectIndex]
					}
				}
				model.clearBoardFilter()
				model.screen = boardScreen
				model.workspace.focus = boardFocus
				model.loading = true
				return model, model.loadBoard(point.boardID)
			}
		}
	}
	model.notice = "Navigation target is no longer available"
	return model, nil
}
