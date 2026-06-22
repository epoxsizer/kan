package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"gitlab.digital-spirit.ru/solutions/common/kan/internal/domain"
)

type paletteItemKind string

const (
	projectItem paletteItemKind = "project"
	boardItem   paletteItemKind = "board"
	columnItem  paletteItemKind = "column"
	cardItem    paletteItemKind = "card"
)

type paletteItem struct {
	kind        paletteItemKind
	label       string
	description string
	searchText  string
	project     domain.Project
	board       domain.Board
	column      domain.Column
	card        domain.Card
}

type paletteLoadedMsg struct {
	items []paletteItem
	err   error
}

func loadPaletteIndex(ctx context.Context, repo domain.Repository) tea.Cmd {
	return func() tea.Msg {
		projects, err := repo.ListProjects(ctx)
		if err != nil {
			return paletteLoadedMsg{err: err}
		}
		items := make([]paletteItem, 0, len(projects))
		for _, project := range projects {
			items = append(items, paletteItem{
				kind: projectItem, label: project.Name,
				description: fallbackDescription(project.Description, "project"),
				searchText:  project.Name + " " + project.Description,
				project:     project,
			})
			boards, listErr := repo.ListBoards(ctx, project.ID)
			if listErr != nil {
				return paletteLoadedMsg{err: listErr}
			}
			for _, board := range boards {
				items = append(items, paletteItem{
					kind: boardItem, label: board.Name,
					description: fmt.Sprintf("%s · %s", project.Name, fallbackDescription(board.Description, "board")),
					searchText:  board.Name + " " + board.Description + " " + project.Name,
					project:     project, board: board,
				})
				columns, columnErr := repo.ListColumns(ctx, board.ID)
				if columnErr != nil {
					return paletteLoadedMsg{err: columnErr}
				}
				for _, column := range columns {
					items = append(items, paletteItem{
						kind: columnItem, label: column.Name,
						description: fmt.Sprintf("%s / %s", project.Name, board.Name),
						searchText:  column.Name + " " + board.Name + " " + project.Name,
						project:     project, board: board, column: column,
					})
				}
				cards, cardErr := repo.ListCards(ctx, board.ID)
				if cardErr != nil {
					return paletteLoadedMsg{err: cardErr}
				}
				for _, card := range cards {
					metadata, _ := json.Marshal(card.Fields)
					items = append(items, paletteItem{
						kind: cardItem, label: card.Title,
						description: fmt.Sprintf("%s / %s · %s", project.Name, board.Name, fallbackDescription(card.Description, "card")),
						searchText:  card.Title + " " + card.Description + " " + strings.Join(card.Tags, " ") + " " + checklistSearchText(card.Checklist) + " " + string(metadata) + " " + board.Name + " " + project.Name,
						project:     project, board: board, card: card,
					})
				}
			}
		}
		return paletteLoadedMsg{items: items}
	}
}

func checklistSearchText(items []domain.ChecklistItem) string {
	values := make([]string, len(items))
	for index, item := range items {
		values[index] = item.Text
	}
	return strings.Join(values, " ")
}

func fallbackDescription(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
