package app

import (
	"strings"
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
	result := editText("title", 5, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("\r\nnext\tpart"), Paste: true}, false)
	require.Equal(t, "title next part", result.value)
}

func TestEditTextSanitizesMultilineTerminalPaste(t *testing.T) {
	paste := "\x1b[2J# Heading\r\n- item\rnext\x00\x07"
	result := editText("", 0, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(paste), Paste: true}, true)
	require.Equal(t, "# Heading\n- item\nnext", result.value)
	require.NotContains(t, result.value, "\r")
	require.NotContains(t, result.value, "\x1b")

	viewport := editorViewport("safe\x1b[2Jtext", len([]rune("safe\x1b[2Jtext")), 30, 2)
	require.NotContains(t, viewport, "\x1b")
	require.Contains(t, viewport, "safe�[2Jtext")
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

	multiline := editorViewport(strings.Repeat("界", 20), 10, 10, 10)
	for _, line := range strings.Split(multiline, "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), 10)
	}
}
