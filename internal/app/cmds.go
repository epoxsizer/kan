package app

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/epoxsizer/kan/internal/domain"
)

func loadProjects(ctx context.Context, repo domain.Repository) tea.Cmd {
	return func() tea.Msg {
		projects, err := repo.ListProjects(ctx)
		if err != nil {
			return projectsLoadedMsg{err: err}
		}
		counts := make(map[string]int, len(projects))
		for _, project := range projects {
			boards, listErr := repo.ListBoards(ctx, project.ID)
			if listErr != nil {
				return projectsLoadedMsg{err: listErr}
			}
			counts[project.ID] = len(boards)
		}
		return projectsLoadedMsg{projects: projects, counts: counts}
	}
}

func loadBoards(ctx context.Context, repo domain.Repository, projectID string) tea.Cmd {
	return func() tea.Msg {
		boards, err := repo.ListBoards(ctx, projectID)
		if err != nil {
			return boardsLoadedMsg{err: err}
		}
		counts := make(map[string]int, len(boards))
		health := make(map[string]boardHealth, len(boards))
		now := time.Now()
		for _, board := range boards {
			cards, listErr := repo.ListCards(ctx, board.ID)
			if listErr != nil {
				return boardsLoadedMsg{err: listErr}
			}
			counts[board.ID] = len(cards)
			health[board.ID] = summarizeBoardHealth(cards, now)
		}
		return boardsLoadedMsg{boards: boards, counts: counts, health: health}
	}
}

func loadBoard(ctx context.Context, repo domain.Repository, boardID string) tea.Cmd {
	return func() tea.Msg {
		columns, err := repo.ListColumns(ctx, boardID)
		if err != nil {
			return boardLoadedMsg{err: err}
		}
		cards, err := repo.ListCards(ctx, boardID)
		return boardLoadedMsg{columns: columns, cards: cards, err: err}
	}
}
