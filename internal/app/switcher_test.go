package app

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/epoxsizer/kan/internal/domain"
	storagesqlite "github.com/epoxsizer/kan/internal/storage/sqlite"
	"github.com/stretchr/testify/require"
)

func TestSwitcherJumpsToCardOnAnotherBoard(t *testing.T) {
	ctx := context.Background()
	repo, err := storagesqlite.Open(ctx, filepath.Join(t.TempDir(), "switcher.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, repo.Close()) })
	project := domain.Project{Name: "Project", Position: 1024}
	require.NoError(t, repo.CreateProject(ctx, &project))
	first := domain.Board{ProjectID: project.ID, Name: "First", Position: 1024}
	require.NoError(t, repo.CreateBoard(ctx, &first))
	second := domain.Board{ProjectID: project.ID, Name: "Second", Position: 2048}
	require.NoError(t, repo.CreateBoard(ctx, &second))
	column := domain.Column{BoardID: second.ID, Name: "Todo", Position: 1024}
	require.NoError(t, repo.CreateColumn(ctx, &column))
	card := domain.Card{BoardID: second.ID, ColumnID: column.ID, Title: "Needle", Position: 1024}
	require.NoError(t, repo.CreateCard(ctx, &card))

	model := testModel(repo)
	model.loading = false
	model.projects = []domain.Project{project}
	model.screen = projectsScreen

	_, command := model.Update(key("ctrl+p"))
	require.NotNil(t, model.switcher)
	runCommands(model, command)
	require.False(t, model.switcher.loading)

	for _, character := range "needle" {
		model.Update(key(string(character)))
	}
	visible := model.switcher.visible()
	require.Len(t, visible, 1)
	require.Equal(t, "card", visible[0].kind)

	_, command = model.Update(key("enter"))
	runCommands(model, command)
	require.Nil(t, model.switcher)
	require.Equal(t, boardScreen, model.screen)
	require.Equal(t, second.ID, model.board.ID)
	require.Equal(t, "Needle", model.selectedCard().Title)
}

func TestSwitcherRendersOverlayAndLoading(t *testing.T) {
	model := testModel(readRepository{})
	model.switcher = &switcherState{loading: true}
	require.Contains(t, model.View(), "Loading…")
	model.switcher = &switcherState{matches: []switcherMatch{{kind: "board", label: "Roadmap", description: "Project"}}}
	view := model.View()
	require.Contains(t, view, "Jump to project, board, or card")
	require.Contains(t, view, "Roadmap")
}

func TestSwitcherJumpsToProject(t *testing.T) {
	projects := []domain.Project{{ID: "p1", Name: "One"}, {ID: "p2", Name: "Two"}}
	model := testModel(readRepository{projects: projects})
	model.loading = false
	model.screen = projectsScreen
	model.projects = projects
	model.switcher = &switcherState{matches: []switcherMatch{{kind: "project", label: "Two", projectID: "p2"}}}
	_, command := model.Update(key("enter"))
	runCommands(model, command)
	require.Nil(t, model.switcher)
	require.Equal(t, boardsScreen, model.screen)
	require.Equal(t, "p2", model.project.ID)
}

func TestSwitcherEscapeAndNoMatches(t *testing.T) {
	model := testModel(readRepository{projects: []domain.Project{{ID: "p", Name: "One"}}})
	model.loading = false
	_, command := model.Update(key("ctrl+p"))
	runCommands(model, command)
	require.NotNil(t, model.switcher)

	for _, character := range "zzzz" {
		model.Update(key(string(character)))
	}
	require.Empty(t, model.switcher.visible())

	model.Update(key("esc"))
	require.Nil(t, model.switcher)
}
