package app

import (
	"slices"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type textEditResult struct {
	value   string
	cursor  int
	handled bool
	changed bool
}

func editText(value string, cursor int, key tea.KeyMsg, multiline bool) textEditResult {
	runes := []rune(value)
	cursor = min(max(cursor, 0), len(runes))
	result := textEditResult{value: value, cursor: cursor}
	replace := func(updated []rune, updatedCursor int) textEditResult {
		return textEditResult{
			value:   string(updated),
			cursor:  min(max(updatedCursor, 0), len(updated)),
			handled: true,
			changed: string(updated) != value,
		}
	}
	if key.Paste {
		insert := []rune(normalizeEditorText(string(key.Runes), multiline))
		return insertRunes(runes, cursor, insert, value)
	}

	switch key.String() {
	case "left":
		result.cursor = max(cursor-1, 0)
		result.handled = true
	case "right":
		result.cursor = min(cursor+1, len(runes))
		result.handled = true
	case "home", "ctrl+a":
		if multiline {
			result.cursor = lineStart(runes, cursor)
		} else {
			result.cursor = 0
		}
		result.handled = true
	case "end", "ctrl+e":
		if multiline {
			result.cursor = lineEnd(runes, cursor)
		} else {
			result.cursor = len(runes)
		}
		result.handled = true
	case "up":
		if !multiline {
			return result
		}
		result.cursor = verticalCursor(runes, cursor, -1)
		result.handled = true
	case "down":
		if !multiline {
			return result
		}
		result.cursor = verticalCursor(runes, cursor, 1)
		result.handled = true
	case "backspace":
		result.handled = true
		if cursor > 0 {
			return replace(append(runes[:cursor-1], runes[cursor:]...), cursor-1)
		}
	case "delete":
		result.handled = true
		if cursor < len(runes) {
			return replace(append(runes[:cursor], runes[cursor+1:]...), cursor)
		}
	case "ctrl+w":
		result.handled = true
		start := previousWordStart(runes, cursor)
		if start < cursor {
			return replace(append(runes[:start], runes[cursor:]...), start)
		}
	case "ctrl+u":
		result.handled = true
		if cursor > 0 {
			return replace(append([]rune{}, runes[cursor:]...), 0)
		}
	case "ctrl+k":
		result.handled = true
		if cursor < len(runes) {
			return replace(append([]rune{}, runes[:cursor]...), cursor)
		}
	case "enter":
		if !multiline {
			return result
		}
		return insertRunes(runes, cursor, []rune{'\n'}, value)
	case "tab":
		if !multiline {
			return result
		}
		return insertRunes(runes, cursor, []rune{'\t'}, value)
	case " ":
		return insertRunes(runes, cursor, []rune{' '}, value)
	default:
		if key.Type == tea.KeySpace {
			return insertRunes(runes, cursor, []rune{' '}, value)
		}
		if key.Type != tea.KeyRunes {
			return result
		}
		insert := append([]rune{}, key.Runes...)
		if !multiline {
			for index, valueRune := range insert {
				if valueRune == '\n' || valueRune == '\r' || valueRune == '\t' {
					insert[index] = ' '
				}
			}
		}
		return insertRunes(runes, cursor, insert, value)
	}
	return result
}

func normalizeEditorText(value string, multiline bool) string {
	value = ansi.Strip(value)
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	var normalized strings.Builder
	for _, valueRune := range value {
		switch {
		case valueRune == '\n' || valueRune == '\t':
			if multiline {
				normalized.WriteRune(valueRune)
			} else {
				normalized.WriteRune(' ')
			}
		case valueRune < ' ' || valueRune == 0x7f:
			continue
		default:
			normalized.WriteRune(valueRune)
		}
	}
	return normalized.String()
}

func insertRunes(value []rune, cursor int, insert []rune, original string) textEditResult {
	updated := make([]rune, 0, len(value)+len(insert))
	updated = append(updated, value[:cursor]...)
	updated = append(updated, insert...)
	updated = append(updated, value[cursor:]...)
	return textEditResult{
		value:   string(updated),
		cursor:  cursor + len(insert),
		handled: true,
		changed: string(updated) != original,
	}
}

func previousWordStart(value []rune, cursor int) int {
	for cursor > 0 && unicode.IsSpace(value[cursor-1]) {
		cursor--
	}
	for cursor > 0 && !unicode.IsSpace(value[cursor-1]) {
		cursor--
	}
	return cursor
}

func textViewport(value string, cursor, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(value)
	cursor = min(max(cursor, 0), len(runes))
	if width == 1 {
		return "█"
	}

	left := runes[:cursor]
	right := runes[cursor:]
	rightBudget := 0
	if len(right) > 0 {
		rightBudget = max((width-1)/3, 1)
	}
	rightVisible := runePrefixByWidth(right, rightBudget)
	leftBudget := width - 1 - lipgloss.Width(string(rightVisible))
	leftVisible := runeSuffixByWidth(left, leftBudget)

	remaining := width - 1 - lipgloss.Width(string(leftVisible)) - lipgloss.Width(string(rightVisible))
	if remaining > 0 && len(leftVisible) < len(left) {
		leftVisible = runeSuffixByWidth(left, lipgloss.Width(string(leftVisible))+remaining)
	}
	remaining = width - 1 - lipgloss.Width(string(leftVisible)) - lipgloss.Width(string(rightVisible))
	if remaining > 0 && len(rightVisible) < len(right) {
		rightVisible = runePrefixByWidth(right, lipgloss.Width(string(rightVisible))+remaining)
	}

	leftClipped := len(leftVisible) < len(left)
	rightClipped := len(rightVisible) < len(right)
	if leftClipped {
		leftVisible = runeSuffixByWidth(leftVisible, max(lipgloss.Width(string(leftVisible))-1, 0))
	}
	if rightClipped {
		rightVisible = runePrefixByWidth(rightVisible, max(lipgloss.Width(string(rightVisible))-1, 0))
	}
	result := ""
	if leftClipped {
		result += "…"
	}
	result += string(leftVisible) + "█" + string(rightVisible)
	if rightClipped {
		result += "…"
	}
	return result
}

func runePrefixByWidth(value []rune, width int) []rune {
	if width <= 0 {
		return nil
	}
	end := 0
	for end < len(value) {
		if lipgloss.Width(string(value[:end+1])) > width {
			break
		}
		end++
	}
	return value[:end]
}

func runeSuffixByWidth(value []rune, width int) []rune {
	if width <= 0 {
		return nil
	}
	start := len(value)
	for start > 0 {
		if lipgloss.Width(string(value[start-1:])) > width {
			break
		}
		start--
	}
	return value[start:]
}

func textValues(fields []formField) []string {
	values := make([]string, len(fields))
	for index := range fields {
		values[index] = fields[index].value
	}
	return values
}

func equalTextValues(left, right []string) bool {
	return slices.Equal(left, right)
}
