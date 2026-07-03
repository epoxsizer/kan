package app

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/x/ansi"
)

var markdownListPattern = regexp.MustCompile(`^(\s*)([-+*]|\d+[.)])(\s+)(\[[ xX]\]\s+)?(.*)$`)

func renderMarkdown(source string, width int) ([]string, error) {
	source = normalizeEditorText(source, true)
	if strings.TrimSpace(source) == "" {
		return nil, nil
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(max(width-4, 10)),
		glamour.WithPreservedNewLines(),
	)
	if err != nil {
		return nil, fmt.Errorf("create markdown renderer: %w", err)
	}
	rendered, err := renderer.Render(source)
	if err != nil {
		return nil, fmt.Errorf("render markdown: %w", err)
	}
	rendered = strings.Trim(rendered, "\n")
	if rendered == "" {
		return nil, nil
	}
	lines := strings.Split(rendered, "\n")
	for index := range lines {
		lines[index] = ansi.Truncate(lines[index], width, "")
	}
	return lines, nil
}

func (control *formControl) markdownPreview(width int) ([]string, error) {
	if control.previewSource == control.value && control.previewWidth == width {
		return control.previewLines, control.previewErr
	}
	control.previewSource = control.value
	control.previewWidth = width
	control.previewLines, control.previewErr = renderMarkdown(control.value, width)
	return control.previewLines, control.previewErr
}

func (control *formControl) pushUndo() {
	state := editorState{value: control.value, cursor: control.cursor}
	if len(control.undo) == 0 || control.undo[len(control.undo)-1] != state {
		control.undo = append(control.undo, state)
		if len(control.undo) > 100 {
			control.undo = control.undo[len(control.undo)-100:]
		}
	}
	control.redo = nil
}

func (control *formControl) undoEdit() {
	if len(control.undo) == 0 {
		return
	}
	control.redo = append(control.redo, editorState{value: control.value, cursor: control.cursor})
	state := control.undo[len(control.undo)-1]
	control.undo = control.undo[:len(control.undo)-1]
	control.value, control.cursor = state.value, state.cursor
}

func (control *formControl) redoEdit() {
	if len(control.redo) == 0 {
		return
	}
	control.undo = append(control.undo, editorState{value: control.value, cursor: control.cursor})
	state := control.redo[len(control.redo)-1]
	control.redo = control.redo[:len(control.redo)-1]
	control.value, control.cursor = state.value, state.cursor
}

func markdownEdit(value string, cursor int, key string) (textEditResult, bool) {
	switch key {
	case "enter":
		return continueMarkdownList(value, cursor), true
	case "tab":
		if markdownLineCanIndent(value, cursor) {
			return indentMarkdownLine(value, cursor, false), true
		}
		return textEditResult{}, false
	case "shift+tab":
		if markdownLineCanIndent(value, cursor) {
			return indentMarkdownLine(value, cursor, true), true
		}
		return textEditResult{}, false
	default:
		return textEditResult{}, false
	}
}

func markdownLineCanIndent(value string, cursor int) bool {
	runes := []rune(value)
	line := string(runes[lineStart(runes, cursor):lineEnd(runes, cursor)])
	return markdownListPattern.MatchString(line)
}

func continueMarkdownList(value string, cursor int) textEditResult {
	runes := []rune(value)
	start, end := lineStart(runes, cursor), lineEnd(runes, cursor)
	line := string(runes[start:end])
	match := markdownListPattern.FindStringSubmatch(line)
	if match == nil {
		return insertRunes(runes, cursor, []rune{'\n'}, value)
	}
	if strings.TrimSpace(match[5]) == "" {
		updated := append(append([]rune{}, runes[:start]...), runes[end:]...)
		return insertRunes(updated, start, []rune{'\n'}, value)
	}
	marker := match[2]
	if marker[0] >= '0' && marker[0] <= '9' {
		number, _ := strconv.Atoi(strings.TrimRight(marker, ".)"))
		suffix := marker[len(marker)-1:]
		marker = strconv.Itoa(number+1) + suffix
	}
	prefix := match[1] + marker + match[3]
	if match[4] != "" {
		prefix += "[ ] "
	}
	return insertRunes(runes, cursor, []rune("\n"+prefix), value)
}

func indentMarkdownLine(value string, cursor int, outdent bool) textEditResult {
	runes := []rune(value)
	start := lineStart(runes, cursor)
	if outdent {
		remove := 0
		for remove < 2 && start+remove < len(runes) && runes[start+remove] == ' ' {
			remove++
		}
		if remove == 0 {
			return textEditResult{value: value, cursor: cursor, handled: true}
		}
		updated := append(append([]rune{}, runes[:start]...), runes[start+remove:]...)
		return textEditResult{value: string(updated), cursor: max(cursor-remove, start), handled: true, changed: true}
	}
	return insertRunes(runes, start, []rune("  "), value)
}

func editorLineColumn(value string, cursor int) (int, int) {
	runes := []rune(value)
	cursor = min(max(cursor, 0), len(runes))
	line := 1
	column := 1
	for _, value := range runes[:cursor] {
		if value == '\n' {
			line++
			column = 1
		} else {
			column++
		}
	}
	return line, column
}

func moveEditorCursorLines(value string, cursor, delta int) int {
	runes := []rune(value)
	step := 1
	if delta < 0 {
		step = -1
		delta = -delta
	}
	for range delta {
		next := verticalCursor(runes, cursor, step)
		if next == cursor {
			break
		}
		cursor = next
	}
	return cursor
}

func moveToSearchMatch(value, query string, cursor int, reverse bool) int {
	valueRunes, queryRunes := []rune(value), []rune(query)
	if len(queryRunes) == 0 || len(queryRunes) > len(valueRunes) {
		return cursor
	}
	lowerValue := []rune(strings.ToLower(value))
	lowerQuery := string([]rune(strings.ToLower(query)))
	if reverse {
		for index := min(cursor-1, len(lowerValue)-len(queryRunes)); index >= 0; index-- {
			if string(lowerValue[index:index+len(queryRunes)]) == lowerQuery {
				return index
			}
		}
		return cursor
	}
	for index := min(cursor+1, len(lowerValue)); index+len(queryRunes) <= len(lowerValue); index++ {
		if string(lowerValue[index:index+len(queryRunes)]) == lowerQuery {
			return index
		}
	}
	for index := 0; index+len(queryRunes) <= min(cursor+1, len(lowerValue)); index++ {
		if string(lowerValue[index:index+len(queryRunes)]) == lowerQuery {
			return index
		}
	}
	return cursor
}
