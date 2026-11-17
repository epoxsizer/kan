package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/epoxsizer/kan/internal/config"
)

// viewsSavedMsg reports the result of persisting saved views.
type viewsSavedMsg struct{ err error }

func (model *Model) startSaveViewForm() {
	model.activateForm(&formModal{kind: saveViewForm, title: "Save view", fields: []formField{
		{label: "View name"},
	}})
}

func (model *Model) startForgetViewForm() {
	names := []string{}
	for _, view := range model.views {
		names = append(names, view.Name)
	}
	model.activateForm(&formModal{kind: forgetViewForm, title: "Remove view", fields: []formField{
		{label: "View", kind: dropdownField, options: names},
	}})
}

// persistViews writes the saved views to the configuration file off the UI
// goroutine.
func (model *Model) persistViews() tea.Cmd {
	if model.configFile == "" {
		return nil
	}
	views := append([]config.View{}, model.views...)
	path := model.configFile
	return func() tea.Msg { return viewsSavedMsg{err: config.SaveViews(path, views)} }
}

func (model *Model) applySavedView(name string) (tea.Model, tea.Cmd) {
	var view *config.View
	for index := range model.views {
		if model.views[index].Name == name {
			view = &model.views[index]
		}
	}
	if view == nil {
		model.err = fmt.Errorf("unknown view %q", name)
		return model, nil
	}
	if model.screen != boardScreen || model.board == nil {
		model.err = fmt.Errorf("open a board first")
		return model, nil
	}
	sortMode, err := parseCardSort(view.Sort)
	if err != nil {
		model.err = err
		return model, nil
	}
	groupMode, err := parseCardGroup(view.Group)
	if err != nil {
		model.err = err
		return model, nil
	}
	model.sortMode = sortMode
	model.groupMode = groupMode
	model.cardIndexes = make(map[string]int, len(model.columns))
	model.applyPlanningFilter(planningKind(view.Planning))
	model.filterText = view.Filter
	if strings.TrimSpace(view.Filter) != "" {
		model.filterLoading = true
		return model, model.searchBoardCards(view.Filter)
	}
	model.filteredCards = nil
	model.filterScores = nil
	model.filterErr = nil
	return model, nil
}
