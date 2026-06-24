package seed

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/epoxsizer/kan/internal/domain"
)

const (
	ProjectID = "00000000-0000-4000-8000-000000000001"
	BoardID   = "00000000-0000-4000-8000-000000000002"
)

func Demo(ctx context.Context, repo domain.Repository) error {
	if _, err := repo.GetProject(ctx, ProjectID); err == nil {
		return nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	project := domain.Project{ID: ProjectID, Name: "Demo Project", Description: "A sample project for exploring kan", Position: 1024}
	if err := repo.CreateProject(ctx, &project); err != nil {
		return err
	}
	board := domain.Board{ID: BoardID, ProjectID: ProjectID, Name: "Product Board", Description: "Demo kanban workflow", Position: 1024}
	if err := repo.CreateBoard(ctx, &board); err != nil {
		return err
	}

	columns := []domain.Column{
		{ID: "00000000-0000-4000-8000-000000000010", BoardID: BoardID, Name: "Backlog", Position: 1024},
		{ID: "00000000-0000-4000-8000-000000000011", BoardID: BoardID, Name: "In Progress", Position: 2048},
		{ID: "00000000-0000-4000-8000-000000000012", BoardID: BoardID, Name: "Done", Position: 3072},
	}
	for index := range columns {
		if err := repo.CreateColumn(ctx, &columns[index]); err != nil {
			return err
		}
	}

	fieldDefs := []domain.FieldDef{
		{ID: "00000000-0000-4000-8000-000000000020", BoardID: BoardID, Key: "effort", Label: "Effort", Type: domain.FieldNumber, Options: json.RawMessage(`[]`), Position: 1024},
		{ID: "00000000-0000-4000-8000-000000000021", BoardID: BoardID, Key: "area", Label: "Area", Type: domain.FieldSelect, Options: json.RawMessage(`["CLI","TUI","Storage"]`), Position: 2048},
	}
	for index := range fieldDefs {
		if err := repo.CreateFieldDef(ctx, &fieldDefs[index]); err != nil {
			return err
		}
	}

	cards := []domain.Card{
		{ID: "00000000-0000-4000-8000-000000000030", BoardID: BoardID, ColumnID: columns[0].ID, Title: "Review keyboard shortcuts", Description: "Make every action discoverable from help.", Position: 1024, Tags: []string{"ux", "keyboard"}, Checklist: []domain.ChecklistItem{{ID: "00000000-0000-4000-8000-000000000040", Text: "Open the help overlay", Done: true, Position: 1024}, {ID: "00000000-0000-4000-8000-000000000041", Text: "Verify movement shortcuts", Position: 2048}}, Fields: map[string]domain.FieldValue{"effort": {Type: domain.FieldNumber, Value: 2}, "area": {Type: domain.FieldSelect, Value: "TUI"}}},
		{ID: "00000000-0000-4000-8000-000000000031", BoardID: BoardID, ColumnID: columns[1].ID, Title: "Implement repository", Description: "Keep SQL behind the domain interface.", Position: 1024, Tags: []string{"storage"}, Fields: map[string]domain.FieldValue{"effort": {Type: domain.FieldNumber, Value: 5}, "area": {Type: domain.FieldSelect, Value: "Storage"}}},
		{ID: "00000000-0000-4000-8000-000000000032", BoardID: BoardID, ColumnID: columns[2].ID, Title: "Create project scaffold", Description: "Static Go module and Cobra command.", Position: 1024, Tags: []string{"foundation"}, Fields: map[string]domain.FieldValue{"area": {Type: domain.FieldSelect, Value: "CLI"}}},
	}
	for index := range cards {
		if err := repo.CreateCard(ctx, &cards[index]); err != nil {
			return err
		}
	}
	return nil
}
