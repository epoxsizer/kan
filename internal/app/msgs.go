package app

import "gitlab.digital-spirit.ru/solutions/common/kan/internal/domain"

type projectsLoadedMsg struct {
	projects []domain.Project
	counts   map[string]int
	err      error
}

type boardsLoadedMsg struct {
	boards []domain.Board
	counts map[string]int
	err    error
}

type boardLoadedMsg struct {
	columns []domain.Column
	cards   []domain.Card
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
