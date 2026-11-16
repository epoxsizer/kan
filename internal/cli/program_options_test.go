package cli

import (
	"context"
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/epoxsizer/kan/internal/config"
	"github.com/stretchr/testify/require"
)

type nopModel struct{}

func (nopModel) Init() tea.Cmd                       { return nil }
func (nopModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return nopModel{}, nil }
func (nopModel) View() string                        { return "" }

// startupFlags reads the program's startup option bitmask so the test can
// check which mouse mode the options enable without running the program.
func startupFlags(t *testing.T, program *tea.Program) int64 {
	t.Helper()
	return reflect.ValueOf(program).Elem().FieldByName("startupOptions").Int()
}

func TestProgramOptionsToggleMouseCapture(t *testing.T) {
	ctx := context.Background()
	mouseFlag := startupFlags(t, tea.NewProgram(nopModel{}, tea.WithMouseCellMotion()))
	require.NotZero(t, mouseFlag)

	enabled := tea.NewProgram(nopModel{}, programOptions(config.Config{Mouse: true}, ctx)...)
	require.Equal(t, mouseFlag, startupFlags(t, enabled)&mouseFlag, "mouse capture is on by default")

	disabled := tea.NewProgram(nopModel{}, programOptions(config.Config{Mouse: false}, ctx)...)
	require.Zero(t, startupFlags(t, disabled)&mouseFlag, "mouse = false must not capture the mouse")
}
