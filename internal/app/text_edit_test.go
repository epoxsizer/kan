package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/require"
)

func TestEditTextSupportsCursorAndDeletionCommands(t *testing.T) {
	result := editText("alpha gamma", 6, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("beta ")}, false)
	require.Equal(t, "alpha beta gamma", result.value)
	require.Equal(t, 11, result.cursor)

	result = editText(result.value, result.cursor, key("ctrl+w"), false)
	require.Equal(t, "alpha gamma", result.value)
	require.Equal(t, 6, result.cursor)

	result = editText("Привет мир", 6, key("backspace"), false)
	require.Equal(t, "Приве мир", result.value)
	require.Equal(t, 5, result.cursor)

	result = editText("left right", 4, key("ctrl+k"), false)
	require.Equal(t, "left", result.value)
	result = editText("left right", 5, key("ctrl+u"), false)
	require.Equal(t, "right", result.value)
	require.Zero(t, result.cursor)
}

func TestEditTextNormalizesMultilinePasteInSingleLineInput(t *testing.T) {
	result := editText("title", 5, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("\nnext\tpart")}, false)
	require.Equal(t, "title next part", result.value)
}

func TestTextViewportFollowsCursorAndRespectsDisplayWidth(t *testing.T) {
	value := "beginning with 世界 and a long visible ending"
	atEnd := textViewport(value, len([]rune(value)), 18)
	require.Contains(t, atEnd, "ending█")
	require.Contains(t, atEnd, "…")
	require.LessOrEqual(t, lipgloss.Width(atEnd), 18)

	inMiddle := textViewport(value, 16, 18)
	require.Contains(t, inMiddle, "█")
	require.LessOrEqual(t, lipgloss.Width(inMiddle), 18)
}
