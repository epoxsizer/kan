package seed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

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
	if err := createProductBoard(ctx, repo); err != nil {
		return err
	}
	if err := createRoadmapBoard(ctx, repo, ProjectID, "00000000-0000-4000-8000-000000000050", 2048); err != nil {
		return err
	}
	if err := createDemoProject(ctx, repo, "00000000-0000-4000-8000-000000000101", "Operations", "Runbooks, releases, and maintenance work.", 2048, "00000000-0000-4000-8000-000000000110"); err != nil {
		return err
	}
	return createDemoProject(ctx, repo, "00000000-0000-4000-8000-000000000201", "Personal", "A lightweight board set for personal planning.", 3072, "00000000-0000-4000-8000-000000000210")
}

func createProductBoard(ctx context.Context, repo domain.Repository) error {
	board := domain.Board{ID: BoardID, ProjectID: ProjectID, Name: "Product Board", Description: "Demo kanban workflow", Position: 1024}
	columns := []domain.Column{
		{ID: "00000000-0000-4000-8000-000000000010", BoardID: BoardID, Name: "Backlog", Position: 1024, Color: stringPtr("Blue"), WIPLimit: intPtr(10)},
		{ID: "00000000-0000-4000-8000-000000000011", BoardID: BoardID, Name: "In Progress", Position: 2048, Color: stringPtr("Orange"), WIPLimit: intPtr(4)},
		{ID: "00000000-0000-4000-8000-000000000012", BoardID: BoardID, Name: "Done", Position: 3072, Color: stringPtr("Green"), WIPLimit: intPtr(20)},
	}
	fieldDefs := []domain.FieldDef{
		{ID: "00000000-0000-4000-8000-000000000020", BoardID: BoardID, Key: "effort", Label: "Effort", Type: domain.FieldNumber, Options: json.RawMessage(`[]`), Position: 1024},
		{ID: "00000000-0000-4000-8000-000000000021", BoardID: BoardID, Key: "area", Label: "Area", Type: domain.FieldSelect, Options: json.RawMessage(`["CLI","TUI","Storage"]`), Position: 2048},
	}
	cards := []domain.Card{
		{ID: "00000000-0000-4000-8000-000000000030", BoardID: BoardID, ColumnID: columns[0].ID, Title: "Review keyboard shortcuts", Description: "Make every action discoverable from help.", Position: 1024, Tags: []string{"ux", "keyboard"}, Checklist: []domain.ChecklistItem{{ID: "00000000-0000-4000-8000-000000000040", Text: "Open the help overlay", Done: true, Position: 1024}, {ID: "00000000-0000-4000-8000-000000000041", Text: "Verify movement shortcuts", Position: 2048}}, Fields: map[string]domain.FieldValue{"effort": {Type: domain.FieldNumber, Value: 2}, "area": {Type: domain.FieldSelect, Value: "TUI"}}},
		{ID: "00000000-0000-4000-8000-000000000031", BoardID: BoardID, ColumnID: columns[1].ID, Title: "Implement repository", Description: "Keep SQL behind the domain interface.", Position: 1024, Tags: []string{"storage"}, Fields: map[string]domain.FieldValue{"effort": {Type: domain.FieldNumber, Value: 5}, "area": {Type: domain.FieldSelect, Value: "Storage"}}},
		{ID: "00000000-0000-4000-8000-000000000032", BoardID: BoardID, ColumnID: columns[2].ID, Title: "Create project scaffold", Description: "Static Go module and Cobra command.", Position: 1024, Tags: []string{"foundation"}, Fields: map[string]domain.FieldValue{"area": {Type: domain.FieldSelect, Value: "CLI"}}},
	}
	return createBoard(ctx, repo, board, columns, fieldDefs, cards)
}

func createRoadmapBoard(ctx context.Context, repo domain.Repository, projectID, boardID string, position float64) error {
	board := domain.Board{ID: boardID, ProjectID: projectID, Name: "Roadmap", Description: "Upcoming product milestones.", Position: position}
	columns := []domain.Column{
		{ID: demoID(boardID, 10), BoardID: boardID, Name: "Ideas", Position: 1024, Color: stringPtr("Purple"), WIPLimit: intPtr(12)},
		{ID: demoID(boardID, 11), BoardID: boardID, Name: "Next", Position: 2048, Color: stringPtr("Blue"), WIPLimit: intPtr(6)},
		{ID: demoID(boardID, 12), BoardID: boardID, Name: "Shipped", Position: 3072, Color: stringPtr("Green"), WIPLimit: intPtr(20)},
	}
	cards := []domain.Card{
		{ID: demoID(boardID, 20), BoardID: boardID, ColumnID: columns[0].ID, Title: "S3 backup target", Description: "Upload local SQLite backups to an S3-compatible bucket.", Position: 1024, Tags: []string{"backup", "s3"}, Fields: map[string]domain.FieldValue{}},
		{ID: demoID(boardID, 21), BoardID: boardID, ColumnID: columns[1].ID, Title: "Theme presets", Description: "Expose more colors through config.toml.", Position: 1024, Tags: []string{"theme"}, Fields: map[string]domain.FieldValue{}},
		{ID: demoID(boardID, 22), BoardID: boardID, ColumnID: columns[2].ID, Title: "Release v0.1.3", Description: "Publish binaries and checksums.", Position: 1024, Tags: []string{"release"}, Fields: map[string]domain.FieldValue{}},
	}
	return createBoard(ctx, repo, board, columns, nil, cards)
}

func createDemoProject(ctx context.Context, repo domain.Repository, projectID, name, description string, position float64, boardPrefix string) error {
	project := domain.Project{ID: projectID, Name: name, Description: description, Position: position}
	if err := repo.CreateProject(ctx, &project); err != nil {
		return err
	}
	if err := createRoadmapBoard(ctx, repo, projectID, boardPrefix, 1024); err != nil {
		return err
	}
	return createRoadmapBoard(ctx, repo, projectID, demoID(boardPrefix, 40), 2048)
}

func createBoard(ctx context.Context, repo domain.Repository, board domain.Board, columns []domain.Column, fieldDefs []domain.FieldDef, cards []domain.Card) error {
	if err := repo.CreateBoard(ctx, &board); err != nil {
		return err
	}
	for index := range columns {
		if err := repo.CreateColumn(ctx, &columns[index]); err != nil {
			return err
		}
	}
	for index := range fieldDefs {
		if err := repo.CreateFieldDef(ctx, &fieldDefs[index]); err != nil {
			return err
		}
	}
	for index := range cards {
		if cards[index].Fields == nil {
			cards[index].Fields = map[string]domain.FieldValue{}
		}
		if err := repo.CreateCard(ctx, &cards[index]); err != nil {
			return err
		}
	}
	return nil
}

func stringPtr(value string) *string { return &value }

func intPtr(value int) *int { return &value }

func demoID(base string, offset int) string {
	prefix := base[:len(base)-3]
	value, err := strconv.Atoi(base[len(base)-3:])
	if err != nil {
		return base
	}
	return prefix + fmt.Sprintf("%03d", value+offset)
}
