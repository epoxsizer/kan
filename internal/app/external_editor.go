package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type externalEditorPreparedMsg struct {
	command *exec.Cmd
	path    string
	err     error
}

type externalEditorFinishedMsg struct {
	content string
	apply   bool
	err     error
}

func prepareExternalEditor(value string) tea.Cmd {
	return func() tea.Msg {
		spec := strings.TrimSpace(os.Getenv("VISUAL"))
		if spec == "" {
			spec = strings.TrimSpace(os.Getenv("EDITOR"))
		}
		if spec == "" {
			return externalEditorPreparedMsg{err: errors.New("set VISUAL or EDITOR to use an external editor")}
		}
		parts, err := parseEditorCommand(spec)
		if err != nil || len(parts) == 0 {
			if err == nil {
				err = errors.New("editor command is empty")
			}
			return externalEditorPreparedMsg{err: fmt.Errorf("parse editor command: %w", err)}
		}
		executable, err := exec.LookPath(parts[0])
		if err != nil {
			return externalEditorPreparedMsg{err: fmt.Errorf("find editor %q: %w", parts[0], err)}
		}
		file, err := os.CreateTemp("", "kan-card-*.md")
		if err != nil {
			return externalEditorPreparedMsg{err: fmt.Errorf("create editor file: %w", err)}
		}
		path := file.Name()
		cleanup := func(cause error) externalEditorPreparedMsg {
			_ = file.Close()
			_ = os.Remove(path)
			return externalEditorPreparedMsg{err: cause}
		}
		if _, err = file.WriteString(value); err != nil {
			return cleanup(fmt.Errorf("write editor file: %w", err))
		}
		if err = file.Close(); err != nil {
			_ = os.Remove(path)
			return externalEditorPreparedMsg{err: fmt.Errorf("close editor file: %w", err)}
		}
		args := append(append([]string{}, parts[1:]...), path)
		return externalEditorPreparedMsg{
			command: exec.Command(executable, args...),
			path:    path,
		}
	}
}

func finishExternalEditor(path string, commandErr error) tea.Msg {
	if commandErr != nil {
		removeErr := os.Remove(path)
		if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			commandErr = errors.Join(commandErr, fmt.Errorf("remove editor file: %w", removeErr))
		}
		return externalEditorFinishedMsg{err: fmt.Errorf("external editor: %w", commandErr)}
	}
	content, readErr := os.ReadFile(path)
	removeErr := os.Remove(path)
	if readErr != nil {
		if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			readErr = errors.Join(readErr, fmt.Errorf("remove editor file: %w", removeErr))
		}
		return externalEditorFinishedMsg{err: fmt.Errorf("read editor file: %w", readErr)}
	}
	if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return externalEditorFinishedMsg{
			content: string(content),
			apply:   true,
			err:     fmt.Errorf("remove editor file: %w", removeErr),
		}
	}
	return externalEditorFinishedMsg{content: string(content), apply: true}
}
