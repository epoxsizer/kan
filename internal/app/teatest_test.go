package app

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"gitlab.digital-spirit.ru/solutions/common/kan/internal/domain"
)

func TestTeatestMainNavigationAndHelp(t *testing.T) {
	repo := readRepository{
		projects: []domain.Project{{ID: "project", Name: "Demo Project"}},
		boards:   []domain.Board{{ID: "board", ProjectID: "project", Name: "Delivery"}},
		columns:  []domain.Column{{ID: "todo", BoardID: "board", Name: "Todo"}},
		cards:    []domain.Card{{ID: "card", BoardID: "board", ColumnID: "todo", Title: "Ship release"}},
	}
	model := testModel(repo)
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(80, 24))
	waitForText := func(value string) {
		teatest.WaitFor(t, tm.Output(), func(output []byte) bool {
			return strings.Contains(string(output), value)
		}, teatest.WithDuration(2*time.Second), teatest.WithCheckInterval(10*time.Millisecond))
	}

	waitForText("Demo Project")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForText("Delivery")
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	waitForText("Ship release")
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	waitForText("BOARD VIEW")
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}
