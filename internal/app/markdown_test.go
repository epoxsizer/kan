package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestRenderMarkdownAndPlainText(t *testing.T) {
	source := "# Release\n\n- **Build** artifacts\n- [ ] Publish\n\n```go\nfmt.Println(\"ok\")\n```"
	lines, err := renderMarkdown(source, 42)
	require.NoError(t, err)
	rendered := ansi.Strip(strings.Join(lines, "\n"))
	for _, expected := range []string{"Release", "Build", "Publish", `fmt.Println("ok")`} {
		require.Contains(t, rendered, expected)
	}
	for _, line := range lines {
		require.LessOrEqual(t, lipgloss.Width(line), 42)
	}
}

func TestMarkdownListEditing(t *testing.T) {
	result := continueMarkdownList("- [x] shipped", len([]rune("- [x] shipped")))
	require.Equal(t, "- [x] shipped\n- [ ] ", result.value)

	result = continueMarkdownList("3. third", len([]rune("3. third")))
	require.Equal(t, "3. third\n4. ", result.value)

	result = continueMarkdownList("before\n- ", len([]rune("before\n- ")))
	require.Equal(t, "before\n\n", result.value)

	result, handled := markdownEdit("- item", 0, "tab")
	require.True(t, handled)
	require.Equal(t, "  - item", result.value)

	_, handled = markdownEdit("plain", 0, "tab")
	require.False(t, handled)
}

func TestMarkdownEditorUndoRedoSearchAndAdaptivePreview(t *testing.T) {
	model := testModel(readRepository{})
	model.form = &formModal{
		kind:   editCardForm,
		fields: []formField{{label: "Description", kind: commentField, markdown: true, value: "# Heading"}},
	}
	model.form.openControl()

	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" text")})
	require.Equal(t, "# Heading text", model.form.control.value)
	model.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})
	require.Equal(t, "# Heading", model.form.control.value)
	model.Update(tea.KeyMsg{Type: tea.KeyCtrlY})
	require.Equal(t, "# Heading text", model.form.control.value)

	model.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("heading")})
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.Equal(t, 2, model.form.control.cursor)
	model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.Less(t, moveEditorCursorLines("one\ntwo\nthree", len([]rune("one\ntwo\nthree")), -2), len([]rune("one\ntwo\nthree")))

	model.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	view := model.View()
	require.Contains(t, view, "EDIT *")
	require.Contains(t, view, "PREVIEW")
	require.Contains(t, ansi.Strip(view), "Heading text")

	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	require.Contains(t, ansi.Strip(model.View()), "Heading text")
	require.NotContains(t, model.View(), "█")
}

func TestMarkdownEditorPastesMultilineContentAsOneSafeEdit(t *testing.T) {
	model := testModel(readRepository{})
	model.form = &formModal{
		kind:   editCardForm,
		fields: []formField{{label: "Description", kind: commentField, markdown: true}},
	}
	model.form.openControl()
	paste := "\x1b[2J## Pasted\r\n\r\n- **first**\r- second\x00"

	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(paste), Paste: true})
	require.Equal(t, "## Pasted\n\n- **first**\n- second", model.form.control.value)
	view := model.View()
	require.NotContains(t, view, "\x1b[2J")
	require.Contains(t, ansi.Strip(view), "Pasted")
	require.LessOrEqual(t, lipgloss.Height(view), 24)
	for _, line := range strings.Split(view, "\n") {
		require.LessOrEqual(t, lipgloss.Width(line), 80)
	}

	model.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})
	require.Empty(t, model.form.control.value)
}
