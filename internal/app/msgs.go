package app

import "github.com/epoxsizer/kan/internal/domain"

// NoticeMsg lets background services display a non-blocking status message.
type NoticeMsg struct {
	Text string
}

type projectsLoadedMsg struct {
	projects []domain.Project
	counts   map[string]int
	err      error
}

type boardsLoadedMsg struct {
	boards []domain.Board
	counts map[string]int
	health map[string]boardHealth
	err    error
}

type boardLoadedMsg struct {
	columns []domain.Column
	cards   []domain.Card
	err     error
}

type archivedCardsLoadedMsg struct {
	cards   []domain.Card
	columns []domain.Column
	err     error
}

type mutationDoneMsg struct {
	scope  screen
	notice string
	err    error
}

type linkCandidatesLoadedMsg struct {
	candidates []linkCandidate
	err        error
}

type boardFilterMsg struct {
	query string
	cards []domain.Card
	err   error
}
