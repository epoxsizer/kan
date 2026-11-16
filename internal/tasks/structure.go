package tasks

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/google/uuid"
)

type BoardPatch struct {
	ExpectedUpdatedAt *time.Time
	Name              *string
	Description       *string
}

type ColumnPatch struct {
	ExpectedUpdatedAt *time.Time
	Name              *string
	WIPLimit          *int
	ClearWIPLimit     bool
	Color             *string
	ClearColor        bool
	AutoArchive       *bool
	ArchiveAfterDays  *int
}

type ChecklistItemPatch struct {
	ExpectedUpdatedAt *time.Time
	Text              *string
	Done              *bool
}

func (coordinator *Coordinator) CreateBoardAtEnd(ctx context.Context, value *domain.Board) error {
	return coordinator.mutate(func() error {
		if _, err := coordinator.Repository.GetProject(ctx, value.ProjectID); err != nil {
			return err
		}
		boards, err := coordinator.Repository.ListBoards(ctx, value.ProjectID)
		if err != nil {
			return err
		}
		maximum := 0.0
		for _, board := range boards {
			maximum = math.Max(maximum, board.Position)
		}
		value.Position = maximum + positionSpacing
		return coordinator.Repository.CreateBoard(ctx, value)
	})
}

func (coordinator *Coordinator) PatchBoard(ctx context.Context, id string, patch BoardPatch) (domain.Board, error) {
	var result domain.Board
	err := coordinator.mutate(func() error {
		board, err := coordinator.Repository.GetBoard(ctx, id)
		if err != nil {
			return err
		}
		if err = checkRevision("board", board.UpdatedAt, patch.ExpectedUpdatedAt); err != nil {
			return err
		}
		if patch.Name != nil {
			board.Name = strings.TrimSpace(*patch.Name)
		}
		if patch.Description != nil {
			board.Description = *patch.Description
		}
		if err = coordinator.Repository.UpdateBoard(ctx, &board); err != nil {
			return err
		}
		result, err = coordinator.Repository.GetBoard(ctx, id)
		return err
	})
	return result, err
}

func (coordinator *Coordinator) CreateColumnAtEnd(ctx context.Context, value *domain.Column) error {
	return coordinator.mutate(func() error {
		if _, err := coordinator.Repository.GetBoard(ctx, value.BoardID); err != nil {
			return err
		}
		columns, err := coordinator.Repository.ListColumns(ctx, value.BoardID)
		if err != nil {
			return err
		}
		maximum := 0.0
		for _, column := range columns {
			maximum = math.Max(maximum, column.Position)
		}
		value.Position = maximum + positionSpacing
		return coordinator.Repository.CreateColumn(ctx, value)
	})
}

func (coordinator *Coordinator) PatchColumn(ctx context.Context, id string, patch ColumnPatch) (domain.Column, error) {
	var result domain.Column
	err := coordinator.mutate(func() error {
		column, err := coordinator.Repository.GetColumn(ctx, id)
		if err != nil {
			return err
		}
		if err = checkRevision("column", column.UpdatedAt, patch.ExpectedUpdatedAt); err != nil {
			return err
		}
		if patch.Name != nil {
			column.Name = strings.TrimSpace(*patch.Name)
		}
		if patch.WIPLimit != nil {
			limit := *patch.WIPLimit
			column.WIPLimit = &limit
		}
		if patch.ClearWIPLimit {
			column.WIPLimit = nil
		}
		if patch.Color != nil {
			color := strings.TrimSpace(*patch.Color)
			column.Color = &color
		}
		if patch.ClearColor {
			column.Color = nil
		}
		if patch.AutoArchive != nil {
			column.AutoArchive = *patch.AutoArchive
		}
		if patch.ArchiveAfterDays != nil {
			column.ArchiveAfterDays = *patch.ArchiveAfterDays
		}
		if err = coordinator.Repository.UpdateColumn(ctx, &column); err != nil {
			return err
		}
		result, err = coordinator.Repository.GetColumn(ctx, id)
		return err
	})
	return result, err
}

func (coordinator *Coordinator) MoveColumnTo(ctx context.Context, id string, targetIndex int, expected ...*time.Time) (domain.Column, error) {
	var result domain.Column
	err := coordinator.mutate(func() error {
		column, err := coordinator.Repository.GetColumn(ctx, id)
		if err != nil {
			return err
		}
		if err = checkRevision("column", column.UpdatedAt, expected...); err != nil {
			return err
		}
		if err := coordinator.Repository.MoveColumn(ctx, id, targetIndex); err != nil {
			return err
		}
		result, err = coordinator.Repository.GetColumn(ctx, id)
		return err
	})
	return result, err
}

func (coordinator *Coordinator) AddChecklistItem(ctx context.Context, cardID, text string, done bool, expected ...*time.Time) (domain.Card, domain.ChecklistItem, error) {
	var result domain.Card
	var item domain.ChecklistItem
	err := coordinator.mutate(func() error {
		card, err := coordinator.Repository.GetCard(ctx, cardID)
		if err != nil {
			return err
		}
		if err = checkRevision("card", card.UpdatedAt, expected...); err != nil {
			return err
		}
		maximum := 0.0
		for _, existing := range card.Checklist {
			maximum = math.Max(maximum, existing.Position)
		}
		item = domain.ChecklistItem{ID: uuid.NewString(), Text: strings.TrimSpace(text), Done: done, Position: maximum + positionSpacing}
		card.Checklist = append(append([]domain.ChecklistItem(nil), card.Checklist...), item)
		if err = coordinator.Repository.UpdateCard(ctx, &card); err != nil {
			return err
		}
		result = card
		return nil
	})
	return result, item, err
}

func (coordinator *Coordinator) PatchChecklistItem(ctx context.Context, cardID, itemID string, patch ChecklistItemPatch) (domain.Card, error) {
	var result domain.Card
	err := coordinator.mutate(func() error {
		card, err := coordinator.Repository.GetCard(ctx, cardID)
		if err != nil {
			return err
		}
		if err = checkRevision("card", card.UpdatedAt, patch.ExpectedUpdatedAt); err != nil {
			return err
		}
		checklist := append([]domain.ChecklistItem(nil), card.Checklist...)
		found := false
		for index := range checklist {
			if checklist[index].ID != itemID {
				continue
			}
			found = true
			if patch.Text != nil {
				checklist[index].Text = strings.TrimSpace(*patch.Text)
			}
			if patch.Done != nil {
				checklist[index].Done = *patch.Done
			}
		}
		if !found {
			return fmt.Errorf("%w: checklist item %q", domain.ErrNotFound, itemID)
		}
		card.Checklist = checklist
		if err = coordinator.Repository.UpdateCard(ctx, &card); err != nil {
			return err
		}
		result = card
		return nil
	})
	return result, err
}

func (coordinator *Coordinator) CreateCardFromTemplate(ctx context.Context, templateID, columnID string) (domain.Card, error) {
	template, err := coordinator.Repository.GetCardTemplate(ctx, templateID)
	if err != nil {
		return domain.Card{}, err
	}
	card := CardFromTemplate(template, template.BoardID, columnID, 0, time.Now())
	if err = coordinator.CreateCardAtEnd(ctx, &card); err != nil {
		return domain.Card{}, err
	}
	return card, nil
}

// CardFromTemplate builds an unsaved card. A template due offset counts whole
// days from the local date of now.
func CardFromTemplate(template domain.CardTemplate, boardID, columnID string, position float64, now time.Time) domain.Card {
	card := domain.Card{
		BoardID: boardID, ColumnID: columnID, Title: template.Title, Description: template.Description, Position: position,
		Tags: append([]string(nil), template.Tags...), Checklist: append([]domain.ChecklistItem(nil), template.Checklist...), Fields: map[string]domain.FieldValue{},
	}
	if template.Priority != nil {
		priority := *template.Priority
		card.Priority = &priority
	}
	if template.DueOffsetDays != nil {
		local := now.In(time.Local)
		due := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, *template.DueOffsetDays).UTC()
		card.DueDate = &due
	}
	return card
}
