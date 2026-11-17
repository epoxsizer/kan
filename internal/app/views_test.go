package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/epoxsizer/kan/internal/config"
	"github.com/epoxsizer/kan/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestSavedViewRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte("[theme]\nprimary = \"#123456\"\n"), 0o600))

	model := testModel(readRepository{})
	model.loading = false
	model.screen = boardScreen
	model.board = &domain.Board{ID: "board"}
	model.columns = []domain.Column{{ID: "todo", Name: "Todo"}}
	model.cards["todo"] = []domain.Card{{ID: "c", ColumnID: "todo", Title: "bug fix"}}
	model.configFile = path
	model.sortMode = sortDue
	model.groupMode = groupPriority
	model.filterText = "bug"
	model.planningFilter = planningOverdue

	model.executeCommand("view-save")
	require.NotNil(t, model.form)
	model.form.fields[0].value = "Focus"
	_, command := model.Update(key("ctrl+s"))
	runCommands(model, command)
	require.Len(t, model.views, 1)

	cfg, err := config.Load(config.Overrides{ConfigFile: path})
	require.NoError(t, err)
	require.Len(t, cfg.Views, 1)
	require.Equal(t, "Focus", cfg.Views[0].Name)
	require.Equal(t, "due date", cfg.Views[0].Sort)
	require.Equal(t, "overdue", cfg.Views[0].Planning)

	model.sortMode = sortPosition
	model.groupMode = groupNone
	model.filterText = ""
	model.planningFilter = planningNone
	model.executeCommand("view-apply:Focus")
	require.Equal(t, sortDue, model.sortMode)
	require.Equal(t, groupPriority, model.groupMode)
	require.Equal(t, "bug", model.filterText)
	require.Equal(t, planningOverdue, model.planningFilter)
}

func TestForgetViewFormRemovesAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(""), 0o600))
	model := testModel(readRepository{})
	model.configFile = path
	model.views = []config.View{{Name: "Focus"}, {Name: "Triage"}}

	model.executeCommand("view-forget")
	require.NotNil(t, model.form)
	require.Equal(t, forgetViewForm, model.form.kind)
	model.form.fields[0].value = "Focus"
	_, command := model.Update(key("ctrl+s"))
	runCommands(model, command)
	require.Len(t, model.views, 1)
	require.Equal(t, "Triage", model.views[0].Name)

	cfg, err := config.Load(config.Overrides{ConfigFile: path})
	require.NoError(t, err)
	require.Len(t, cfg.Views, 1)
	require.Equal(t, "Triage", cfg.Views[0].Name)
}

func TestViewMenuListsSavedViews(t *testing.T) {
	model := testModel(readRepository{})
	model.views = []config.View{{Name: "Focus"}}
	model.commandMode = true
	model.commandMenu = "view"
	model.command = "foc"
	found := false
	for _, match := range model.paletteMatches() {
		if match.command == "view-apply:Focus" {
			found = true
		}
	}
	require.True(t, found, "saved views appear in the view menu")
}
