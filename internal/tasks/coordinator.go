package tasks

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/epoxsizer/kan/internal/domain"
)

const positionSpacing = 1024.0

type Coordinator struct {
	domain.Repository
	mu sync.Mutex
}

type CardPatch struct {
	Title          *string
	Description    *string
	Priority       *string
	ClearPriority  bool
	DueDate        *time.Time
	ClearDueDate   bool
	Tags           *[]string
	RelatedCardIDs *[]string
	Checklist      *[]domain.ChecklistItem
	Fields         *map[string]domain.FieldValue
}

func New(repo domain.Repository) *Coordinator {
	return &Coordinator{Repository: repo}
}

func (coordinator *Coordinator) CreateProject(ctx context.Context, value *domain.Project) error {
	return coordinator.mutate(func() error { return coordinator.Repository.CreateProject(ctx, value) })
}

func (coordinator *Coordinator) UpdateProject(ctx context.Context, value *domain.Project) error {
	return coordinator.mutate(func() error { return coordinator.Repository.UpdateProject(ctx, value) })
}

func (coordinator *Coordinator) DeleteProject(ctx context.Context, id string) error {
	return coordinator.mutate(func() error { return coordinator.Repository.DeleteProject(ctx, id) })
}

func (coordinator *Coordinator) CreateBoard(ctx context.Context, value *domain.Board) error {
	return coordinator.mutate(func() error { return coordinator.Repository.CreateBoard(ctx, value) })
}

func (coordinator *Coordinator) UpdateBoard(ctx context.Context, value *domain.Board) error {
	return coordinator.mutate(func() error { return coordinator.Repository.UpdateBoard(ctx, value) })
}

func (coordinator *Coordinator) DeleteBoard(ctx context.Context, id string) error {
	return coordinator.mutate(func() error { return coordinator.Repository.DeleteBoard(ctx, id) })
}

func (coordinator *Coordinator) CreateColumn(ctx context.Context, value *domain.Column) error {
	return coordinator.mutate(func() error { return coordinator.Repository.CreateColumn(ctx, value) })
}

func (coordinator *Coordinator) UpdateColumn(ctx context.Context, value *domain.Column) error {
	return coordinator.mutate(func() error { return coordinator.Repository.UpdateColumn(ctx, value) })
}

func (coordinator *Coordinator) MoveColumn(ctx context.Context, id string, target int) error {
	return coordinator.mutate(func() error { return coordinator.Repository.MoveColumn(ctx, id, target) })
}

func (coordinator *Coordinator) DeleteColumn(ctx context.Context, id string) error {
	return coordinator.mutate(func() error { return coordinator.Repository.DeleteColumn(ctx, id) })
}

func (coordinator *Coordinator) CreateCard(ctx context.Context, value *domain.Card) error {
	return coordinator.mutate(func() error { return coordinator.Repository.CreateCard(ctx, value) })
}

func (coordinator *Coordinator) UpdateCard(ctx context.Context, value *domain.Card) error {
	return coordinator.mutate(func() error { return coordinator.Repository.UpdateCard(ctx, value) })
}

func (coordinator *Coordinator) MoveCard(ctx context.Context, id, columnID string, target int) error {
	return coordinator.mutate(func() error { return coordinator.Repository.MoveCard(ctx, id, columnID, target) })
}

func (coordinator *Coordinator) DeleteCard(ctx context.Context, id string) error {
	return coordinator.mutate(func() error { return coordinator.Repository.DeleteCard(ctx, id) })
}

func (coordinator *Coordinator) RestoreCard(ctx context.Context, id string) error {
	return coordinator.mutate(func() error { return coordinator.Repository.RestoreCard(ctx, id) })
}

func (coordinator *Coordinator) ArchiveCardsInColumn(ctx context.Context, id string) (int, error) {
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	return coordinator.Repository.ArchiveCardsInColumn(ctx, id)
}

func (coordinator *Coordinator) ArchiveExpiredCards(ctx context.Context, boardID string) (int, error) {
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	return coordinator.Repository.ArchiveExpiredCards(ctx, boardID)
}

func (coordinator *Coordinator) CreateCardTemplate(ctx context.Context, value *domain.CardTemplate) error {
	return coordinator.mutate(func() error { return coordinator.Repository.CreateCardTemplate(ctx, value) })
}

func (coordinator *Coordinator) UpdateCardTemplate(ctx context.Context, value *domain.CardTemplate) error {
	return coordinator.mutate(func() error { return coordinator.Repository.UpdateCardTemplate(ctx, value) })
}

func (coordinator *Coordinator) DeleteCardTemplate(ctx context.Context, id string) error {
	return coordinator.mutate(func() error { return coordinator.Repository.DeleteCardTemplate(ctx, id) })
}

func (coordinator *Coordinator) CreateFieldDef(ctx context.Context, value *domain.FieldDef) error {
	return coordinator.mutate(func() error { return coordinator.Repository.CreateFieldDef(ctx, value) })
}

func (coordinator *Coordinator) UpdateFieldDef(ctx context.Context, value *domain.FieldDef) error {
	return coordinator.mutate(func() error { return coordinator.Repository.UpdateFieldDef(ctx, value) })
}

func (coordinator *Coordinator) DeleteFieldDef(ctx context.Context, id string) error {
	return coordinator.mutate(func() error { return coordinator.Repository.DeleteFieldDef(ctx, id) })
}

func (coordinator *Coordinator) ImportDocument(ctx context.Context, document domain.ExportDocument, replace bool) error {
	return coordinator.mutate(func() error { return coordinator.Repository.ImportDocument(ctx, document, replace) })
}

func (coordinator *Coordinator) Backup(ctx context.Context, destination string) error {
	return coordinator.mutate(func() error {
		backupper, ok := coordinator.Repository.(interface {
			Backup(context.Context, string) error
		})
		if !ok {
			return fmt.Errorf("repository does not support backups")
		}
		return backupper.Backup(ctx, destination)
	})
}

func (coordinator *Coordinator) CreateCardAtEnd(ctx context.Context, value *domain.Card) error {
	return coordinator.mutate(func() error {
		column, err := coordinator.Repository.GetColumn(ctx, value.ColumnID)
		if err != nil {
			return err
		}
		if column.BoardID != value.BoardID {
			return fmt.Errorf("%w: column belongs to another board", domain.ErrConflict)
		}
		cards, err := coordinator.Repository.ListCards(ctx, value.BoardID)
		if err != nil {
			return err
		}
		count := 0
		maximum := 0.0
		for _, card := range cards {
			if card.ColumnID != value.ColumnID {
				continue
			}
			count++
			maximum = math.Max(maximum, card.Position)
		}
		if column.WIPLimit != nil && count >= *column.WIPLimit {
			return fmt.Errorf("%w: target column WIP limit %d reached", domain.ErrConflict, *column.WIPLimit)
		}
		value.Position = maximum + positionSpacing
		return coordinator.Repository.CreateCard(ctx, value)
	})
}

func (coordinator *Coordinator) PatchCard(ctx context.Context, id string, patch CardPatch) (domain.Card, error) {
	var result domain.Card
	err := coordinator.mutate(func() error {
		card, err := coordinator.Repository.GetCard(ctx, id)
		if err != nil {
			return err
		}
		if patch.Title != nil {
			card.Title = *patch.Title
		}
		if patch.Description != nil {
			card.Description = *patch.Description
		}
		if patch.Priority != nil {
			card.Priority = patch.Priority
		}
		if patch.ClearPriority {
			card.Priority = nil
		}
		if patch.DueDate != nil {
			due := patch.DueDate.UTC()
			card.DueDate = &due
		}
		if patch.ClearDueDate {
			card.DueDate = nil
		}
		if patch.Tags != nil {
			card.Tags = append([]string(nil), (*patch.Tags)...)
		}
		if patch.RelatedCardIDs != nil {
			card.RelatedCardIDs = append([]string(nil), (*patch.RelatedCardIDs)...)
		}
		if patch.Checklist != nil {
			card.Checklist = append([]domain.ChecklistItem(nil), (*patch.Checklist)...)
		}
		if patch.Fields != nil {
			card.Fields = cloneFields(*patch.Fields)
		}
		if err = coordinator.Repository.UpdateCard(ctx, &card); err != nil {
			return err
		}
		result = card
		return nil
	})
	return result, err
}

func (coordinator *Coordinator) MoveCardTo(ctx context.Context, id, columnID string, targetIndex *int) (domain.Card, error) {
	var result domain.Card
	err := coordinator.mutate(func() error {
		index := 0
		if targetIndex == nil {
			card, err := coordinator.Repository.GetCard(ctx, id)
			if err != nil {
				return err
			}
			cards, err := coordinator.Repository.ListCards(ctx, card.BoardID)
			if err != nil {
				return err
			}
			for _, candidate := range cards {
				if candidate.ColumnID == columnID && candidate.ID != id {
					index++
				}
			}
		} else {
			index = *targetIndex
		}
		if err := coordinator.Repository.MoveCard(ctx, id, columnID, index); err != nil {
			return err
		}
		card, err := coordinator.Repository.GetCard(ctx, id)
		if err != nil {
			return err
		}
		result = card
		return nil
	})
	return result, err
}

func (coordinator *Coordinator) GetCardIncludingArchived(ctx context.Context, id string) (domain.Card, error) {
	return coordinator.getCardIncludingArchived(ctx, id)
}

func (coordinator *Coordinator) ArchiveCard(ctx context.Context, id string) (domain.Card, error) {
	var result domain.Card
	err := coordinator.mutate(func() error {
		_, err := coordinator.Repository.GetCard(ctx, id)
		if err != nil {
			return err
		}
		if err = coordinator.Repository.DeleteCard(ctx, id); err != nil {
			return err
		}
		result, err = coordinator.getCardIncludingArchived(ctx, id)
		return err
	})
	return result, err
}

func (coordinator *Coordinator) RestoreArchivedCard(ctx context.Context, id string) (domain.Card, error) {
	var result domain.Card
	err := coordinator.mutate(func() error {
		card, err := coordinator.getCardIncludingArchived(ctx, id)
		if err != nil {
			return err
		}
		if card.DeletedAt == nil {
			return fmt.Errorf("%w: card is not archived", domain.ErrConflict)
		}
		if err = coordinator.Repository.RestoreCard(ctx, id); err != nil {
			return err
		}
		result, err = coordinator.Repository.GetCard(ctx, id)
		return err
	})
	return result, err
}

func (coordinator *Coordinator) getCardIncludingArchived(ctx context.Context, id string) (domain.Card, error) {
	projects, err := coordinator.Repository.ListProjects(ctx)
	if err != nil {
		return domain.Card{}, err
	}
	for _, project := range projects {
		boards, listErr := coordinator.Repository.ListBoards(ctx, project.ID)
		if listErr != nil {
			return domain.Card{}, listErr
		}
		for _, board := range boards {
			cards, cardsErr := coordinator.Repository.ListCardsIncludingDeleted(ctx, board.ID)
			if cardsErr != nil {
				return domain.Card{}, cardsErr
			}
			for _, card := range cards {
				if card.ID == id {
					return card, nil
				}
			}
		}
	}
	return domain.Card{}, domain.ErrNotFound
}

func (coordinator *Coordinator) mutate(action func() error) error {
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	return action()
}

func cloneFields(source map[string]domain.FieldValue) map[string]domain.FieldValue {
	result := make(map[string]domain.FieldValue, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
