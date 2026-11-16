package tasks

import (
	"context"
	"math"
	"strings"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/google/uuid"
)

// BoardSetup describes a new board together with its columns and card
// templates, created in one atomic mutation.
type BoardSetup struct {
	Name        string
	Description string
	Columns     []ColumnSetup
	Templates   []TemplateSetup
}

type ColumnSetup struct {
	Name             string
	WIPLimit         *int
	Color            *string
	AutoArchive      bool
	ArchiveAfterDays int
}

type TemplateSetup struct {
	Name          string
	Title         string
	Description   string
	Priority      *string
	DueOffsetDays *int
	Tags          []string
	Checklist     []string
}

type BoardSetupResult struct {
	Board     domain.Board
	Columns   []domain.Column
	Templates []domain.CardTemplate
}

// SetupBoardInProject appends a fully configured board to an existing
// project. Columns and templates keep the given order; any invalid part
// fails the whole setup without writing anything.
func (coordinator *Coordinator) SetupBoardInProject(ctx context.Context, projectID string, setup BoardSetup) (BoardSetupResult, error) {
	var result BoardSetupResult
	err := coordinator.mutate(func() error {
		if _, err := coordinator.Repository.GetProject(ctx, projectID); err != nil {
			return err
		}
		boards, err := coordinator.Repository.ListBoards(ctx, projectID)
		if err != nil {
			return err
		}
		maximum := 0.0
		for _, board := range boards {
			maximum = math.Max(maximum, board.Position)
		}
		board := domain.Board{ProjectID: projectID, Name: strings.TrimSpace(setup.Name), Description: setup.Description, Position: maximum + positionSpacing}
		columns := make([]domain.Column, 0, len(setup.Columns))
		for index, column := range setup.Columns {
			columns = append(columns, domain.Column{
				Name: strings.TrimSpace(column.Name), Position: float64(index+1) * positionSpacing,
				WIPLimit: column.WIPLimit, Color: column.Color, AutoArchive: column.AutoArchive, ArchiveAfterDays: column.ArchiveAfterDays,
			})
		}
		templates := make([]domain.CardTemplate, 0, len(setup.Templates))
		for index, template := range setup.Templates {
			checklist := make([]domain.ChecklistItem, 0, len(template.Checklist))
			for itemIndex, text := range template.Checklist {
				checklist = append(checklist, domain.ChecklistItem{ID: uuid.NewString(), Text: strings.TrimSpace(text), Position: float64(itemIndex+1) * positionSpacing})
			}
			templates = append(templates, domain.CardTemplate{
				Name: strings.TrimSpace(template.Name), Title: strings.TrimSpace(template.Title), Description: template.Description,
				Priority: template.Priority, DueOffsetDays: template.DueOffsetDays, Tags: template.Tags, Checklist: checklist,
				Position: float64(index+1) * positionSpacing,
			})
		}
		if err := coordinator.Repository.SetupBoard(ctx, &board, columns, templates); err != nil {
			return err
		}
		result = BoardSetupResult{Board: board, Columns: columns, Templates: templates}
		return nil
	})
	return result, err
}
