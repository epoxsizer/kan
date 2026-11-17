package app

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/epoxsizer/kan/internal/config"
	"github.com/stretchr/testify/require"
)

// runBatchCommands executes a command tree, unwrapping tea.BatchMsg the way
// the Bubble Tea runtime does, and feeds every message back into the model.
func runBatchCommands(model *Model, command tea.Cmd) {
	if command == nil {
		return
	}
	message := command()
	if batch, ok := message.(tea.BatchMsg); ok {
		for _, item := range batch {
			runBatchCommands(model, item)
		}
		return
	}
	_, next := model.Update(message)
	runBatchCommands(model, next)
}

func TestSettingsMenuAppliesAndSavesGeneralParameters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte("show_card_tags = true\n\n[theme]\nprimary = \"#123456\"\n"), 0o600))

	model := testModel(readRepository{})
	model.configFile = path
	model.executeCommand("settings")
	require.NotNil(t, model.form)
	require.Equal(t, settingsForm, model.form.kind)
	require.Len(t, model.form.fields, 8)

	model.form.fields[5].value = "Disabled" // wrap card titles
	model.form.fields[6].value = "Disabled" // mouse capture
	model.form.fields[7].value = "Enabled"  // legacy keymap
	_, command := model.Update(key("ctrl+s"))

	require.Nil(t, model.form)
	require.False(t, model.wrapCardTitles)
	require.False(t, model.mouseEnabled)
	require.True(t, model.legacyKeymap)
	require.NotNil(t, command, "mouse change and config save are scheduled")
	runBatchCommands(model, command)
	require.Equal(t, "Settings saved", model.notice)

	cfg, err := config.Load(config.Overrides{ConfigFile: path})
	require.NoError(t, err)
	require.False(t, cfg.WrapCardTitles)
	require.False(t, cfg.Mouse)
	require.True(t, cfg.LegacyKeymap)
	require.True(t, cfg.ShowCardTags)
	require.Equal(t, "#123456", cfg.Theme.Primary, "other settings survive the save")
}
