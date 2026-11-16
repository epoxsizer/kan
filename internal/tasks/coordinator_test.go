package tasks

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/epoxsizer/kan/internal/domain"
	storage "github.com/epoxsizer/kan/internal/storage/sqlite"
	"github.com/stretchr/testify/require"
)

func openTestCoordinator(t *testing.T) *Coordinator {
	t.Helper()
	repo, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "kan.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, repo.Close()) })
	return New(repo)
}

func createHierarchy(t *testing.T, coordinator *Coordinator) (domain.Board, domain.Column, domain.Column) {
	t.Helper()
	ctx := context.Background()
	project := domain.Project{Name: "Project", Position: 1024}
	require.NoError(t, coordinator.CreateProject(ctx, &project))
	board := domain.Board{ProjectID: project.ID, Name: "Board", Position: 1024}
	require.NoError(t, coordinator.CreateBoard(ctx, &board))
	first := domain.Column{BoardID: board.ID, Name: "Backlog", Position: 1024}
	second := domain.Column{BoardID: board.ID, Name: "Doing", Position: 2048}
	require.NoError(t, coordinator.CreateColumn(ctx, &first))
	require.NoError(t, coordinator.CreateColumn(ctx, &second))
	return board, first, second
}

func createCard(t *testing.T, coordinator *Coordinator, board domain.Board, column domain.Column, title string) domain.Card {
	t.Helper()
	card := domain.Card{BoardID: board.ID, ColumnID: column.ID, Title: title}
	require.NoError(t, coordinator.CreateCardAtEnd(context.Background(), &card))
	return card
}

func TestDirectCardWritesEnforceTheSameWIPRulesAsAgentWrites(t *testing.T) {
	ctx := context.Background()
	coordinator := openTestCoordinator(t)
	board, first, target := createHierarchy(t, coordinator)
	limit := 1
	target.WIPLimit = &limit
	require.NoError(t, coordinator.UpdateColumn(ctx, &target))
	createCard(t, coordinator, board, target, "Existing")
	card := domain.Card{BoardID: board.ID, ColumnID: target.ID, Title: "TUI create", Position: 2048}
	require.ErrorIs(t, coordinator.CreateCard(ctx, &card), domain.ErrConflict)
	card = createCard(t, coordinator, board, first, "TUI move by editing")
	card.ColumnID = target.ID
	require.ErrorIs(t, coordinator.UpdateCard(ctx, &card), domain.ErrConflict)
	stored, err := coordinator.GetCard(ctx, card.ID)
	require.NoError(t, err)
	require.Equal(t, first.ID, stored.ColumnID)
}

func TestTUIStructureEditsRejectStaleSnapshots(t *testing.T) {
	ctx := context.Background()
	coordinator := openTestCoordinator(t)
	board, column, _ := createHierarchy(t, coordinator)
	name := "Changed by agent"
	_, err := coordinator.PatchBoard(ctx, board.ID, BoardPatch{Name: &name})
	require.NoError(t, err)
	board.Name = "Stale human edit"
	require.ErrorIs(t, coordinator.UpdateBoard(ctx, &board), domain.ErrConflict)
	_, err = coordinator.PatchColumn(ctx, column.ID, ColumnPatch{Name: &name})
	require.NoError(t, err)
	column.Name = "Stale human edit"
	require.ErrorIs(t, coordinator.UpdateColumn(ctx, &column), domain.ErrConflict)
}

func TestCreateCardAtEndAppendsAndEnforcesConstraints(t *testing.T) {
	ctx := context.Background()
	coordinator := openTestCoordinator(t)
	board, first, second := createHierarchy(t, coordinator)

	one := createCard(t, coordinator, board, first, "One")
	two := createCard(t, coordinator, board, first, "Two")
	other := createCard(t, coordinator, board, second, "Other")
	require.Equal(t, positionSpacing, one.Position)
	require.Equal(t, 2*positionSpacing, two.Position)
	require.Equal(t, positionSpacing, other.Position)

	limit := 2
	first.WIPLimit = &limit
	require.NoError(t, coordinator.UpdateColumn(ctx, &first))
	blocked := domain.Card{BoardID: board.ID, ColumnID: first.ID, Title: "Blocked"}
	require.ErrorIs(t, coordinator.CreateCardAtEnd(ctx, &blocked), domain.ErrConflict)

	foreignProject := domain.Project{Name: "Foreign", Position: 2048}
	require.NoError(t, coordinator.CreateProject(ctx, &foreignProject))
	foreignBoard := domain.Board{ProjectID: foreignProject.ID, Name: "Foreign", Position: 1024}
	require.NoError(t, coordinator.CreateBoard(ctx, &foreignBoard))
	mismatched := domain.Card{BoardID: foreignBoard.ID, ColumnID: second.ID, Title: "Mismatched"}
	require.ErrorIs(t, coordinator.CreateCardAtEnd(ctx, &mismatched), domain.ErrConflict)

	missing := domain.Card{BoardID: board.ID, ColumnID: "missing", Title: "Missing"}
	require.ErrorIs(t, coordinator.CreateCardAtEnd(ctx, &missing), domain.ErrNotFound)
}

func TestConcurrentCreateCardAtEndKeepsUniquePositions(t *testing.T) {
	ctx := context.Background()
	coordinator := openTestCoordinator(t)
	board, column, _ := createHierarchy(t, coordinator)

	const workers = 16
	var group sync.WaitGroup
	errs := make(chan error, workers)
	for index := range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			card := domain.Card{BoardID: board.ID, ColumnID: column.ID, Title: "Card " + string(rune('A'+index))}
			errs <- coordinator.CreateCardAtEnd(ctx, &card)
		}()
	}
	group.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	cards, err := coordinator.ListCards(ctx, board.ID)
	require.NoError(t, err)
	require.Len(t, cards, workers)
	positions := map[float64]struct{}{}
	for _, card := range cards {
		positions[card.Position] = struct{}{}
	}
	require.Len(t, positions, workers)
}

func TestPatchCardChangesOnlyProvidedFields(t *testing.T) {
	ctx := context.Background()
	coordinator := openTestCoordinator(t)
	board, column, _ := createHierarchy(t, coordinator)
	priority := "High"
	due := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	card := domain.Card{
		BoardID: board.ID, ColumnID: column.ID, Title: "Original", Description: "Keep me",
		Priority: &priority, DueDate: &due, Tags: []string{"keep"},
	}
	require.NoError(t, coordinator.CreateCardAtEnd(ctx, &card))

	title := "Renamed"
	tags := []string{"new"}
	patched, err := coordinator.PatchCard(ctx, card.ID, CardPatch{Title: &title, Tags: &tags})
	require.NoError(t, err)
	require.Equal(t, "Renamed", patched.Title)
	require.Equal(t, "Keep me", patched.Description)
	require.Equal(t, []string{"new"}, patched.Tags)
	require.NotNil(t, patched.Priority)
	require.Equal(t, "High", *patched.Priority)
	require.NotNil(t, patched.DueDate)

	tags[0] = "mutated after patch"
	stored, err := coordinator.GetCard(ctx, card.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"new"}, stored.Tags)

	patched, err = coordinator.PatchCard(ctx, card.ID, CardPatch{ClearPriority: true, ClearDueDate: true})
	require.NoError(t, err)
	require.Nil(t, patched.Priority)
	require.Nil(t, patched.DueDate)

	_, err = coordinator.PatchCard(ctx, "missing", CardPatch{Title: &title})
	require.ErrorIs(t, err, domain.ErrNotFound)

	empty := " "
	_, err = coordinator.PatchCard(ctx, card.ID, CardPatch{Title: &empty})
	require.ErrorIs(t, err, domain.ErrValidation)
}

func TestMoveCardToAppendsOrInsertsAtIndex(t *testing.T) {
	ctx := context.Background()
	coordinator := openTestCoordinator(t)
	board, first, second := createHierarchy(t, coordinator)
	moving := createCard(t, coordinator, board, first, "Moving")
	a := createCard(t, coordinator, board, second, "A")
	b := createCard(t, coordinator, board, second, "B")

	moved, err := coordinator.MoveCardTo(ctx, moving.ID, second.ID, nil)
	require.NoError(t, err)
	require.Equal(t, second.ID, moved.ColumnID)
	require.Equal(t, []string{a.ID, b.ID, moving.ID}, columnCardIDs(t, coordinator, board.ID, second.ID))

	index := 0
	_, err = coordinator.MoveCardTo(ctx, moving.ID, second.ID, &index)
	require.NoError(t, err)
	require.Equal(t, []string{moving.ID, a.ID, b.ID}, columnCardIDs(t, coordinator, board.ID, second.ID))

	_, err = coordinator.MoveCardTo(ctx, "missing", second.ID, nil)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestArchiveAndRestoreCard(t *testing.T) {
	ctx := context.Background()
	coordinator := openTestCoordinator(t)
	board, column, _ := createHierarchy(t, coordinator)
	card := createCard(t, coordinator, board, column, "Archive me")

	_, err := coordinator.RestoreArchivedCard(ctx, card.ID)
	require.ErrorIs(t, err, domain.ErrConflict)

	archived, err := coordinator.ArchiveCard(ctx, card.ID)
	require.NoError(t, err)
	require.NotNil(t, archived.DeletedAt)
	_, err = coordinator.GetCard(ctx, card.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	found, err := coordinator.GetCardIncludingArchived(ctx, card.ID)
	require.NoError(t, err)
	require.Equal(t, card.ID, found.ID)

	_, err = coordinator.ArchiveCard(ctx, card.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)

	restored, err := coordinator.RestoreArchivedCard(ctx, card.ID)
	require.NoError(t, err)
	require.Nil(t, restored.DeletedAt)

	_, err = coordinator.GetCardIncludingArchived(ctx, "missing")
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func columnCardIDs(t *testing.T, coordinator *Coordinator, boardID, columnID string) []string {
	t.Helper()
	cards, err := coordinator.ListCards(context.Background(), boardID)
	require.NoError(t, err)
	ids := []string{}
	for _, card := range cards {
		if card.ColumnID == columnID {
			ids = append(ids, card.ID)
		}
	}
	return ids
}

func TestBoardAndColumnStructureMutations(t *testing.T) {
	ctx := context.Background()
	coordinator := openTestCoordinator(t)
	board, first, second := createHierarchy(t, coordinator)

	next := domain.Board{ProjectID: board.ProjectID, Name: "Next"}
	require.NoError(t, coordinator.CreateBoardAtEnd(ctx, &next))
	require.Equal(t, board.Position+positionSpacing, next.Position)
	missingProject := domain.Board{ProjectID: "missing", Name: "Orphan"}
	require.ErrorIs(t, coordinator.CreateBoardAtEnd(ctx, &missingProject), domain.ErrNotFound)

	name, description := "  Renamed  ", "Notes"
	patchedBoard, err := coordinator.PatchBoard(ctx, next.ID, BoardPatch{Name: &name, Description: &description})
	require.NoError(t, err)
	require.Equal(t, "Renamed", patchedBoard.Name)
	require.Equal(t, "Notes", patchedBoard.Description)
	blank := " "
	_, err = coordinator.PatchBoard(ctx, next.ID, BoardPatch{Name: &blank})
	require.ErrorIs(t, err, domain.ErrValidation)

	review := domain.Column{BoardID: board.ID, Name: "Review"}
	require.NoError(t, coordinator.CreateColumnAtEnd(ctx, &review))
	require.Equal(t, second.Position+positionSpacing, review.Position)
	require.Equal(t, 14, review.ArchiveAfterDays)
	require.False(t, review.AutoArchive)

	limit, color, enabled, days := 3, "Green", true, 30
	patchedColumn, err := coordinator.PatchColumn(ctx, review.ID, ColumnPatch{WIPLimit: &limit, Color: &color, AutoArchive: &enabled, ArchiveAfterDays: &days})
	require.NoError(t, err)
	require.Equal(t, 3, *patchedColumn.WIPLimit)
	require.Equal(t, "Green", *patchedColumn.Color)
	require.True(t, patchedColumn.AutoArchive)
	require.Equal(t, 30, patchedColumn.ArchiveAfterDays)
	patchedColumn, err = coordinator.PatchColumn(ctx, review.ID, ColumnPatch{ClearWIPLimit: true, ClearColor: true})
	require.NoError(t, err)
	require.Nil(t, patchedColumn.WIPLimit)
	require.Nil(t, patchedColumn.Color)
	zero := 0
	_, err = coordinator.PatchColumn(ctx, review.ID, ColumnPatch{WIPLimit: &zero})
	require.ErrorIs(t, err, domain.ErrValidation)

	_, err = coordinator.MoveColumnTo(ctx, review.ID, 0)
	require.NoError(t, err)
	columns, err := coordinator.ListColumns(ctx, board.ID)
	require.NoError(t, err)
	require.Equal(t, []string{review.ID, first.ID, second.ID}, []string{columns[0].ID, columns[1].ID, columns[2].ID})
}

func TestChecklistItemMutations(t *testing.T) {
	ctx := context.Background()
	coordinator := openTestCoordinator(t)
	board, column, _ := createHierarchy(t, coordinator)
	card := createCard(t, coordinator, board, column, "Checklist")

	updated, first, err := coordinator.AddChecklistItem(ctx, card.ID, " Write test ", false)
	require.NoError(t, err)
	require.NotEmpty(t, first.ID)
	require.Equal(t, "Write test", first.Text)
	updated, second, err := coordinator.AddChecklistItem(ctx, card.ID, "Make it pass", false)
	require.NoError(t, err)
	require.Greater(t, second.Position, first.Position)
	require.Len(t, updated.Checklist, 2)
	_, _, err = coordinator.AddChecklistItem(ctx, card.ID, " ", false)
	require.ErrorIs(t, err, domain.ErrValidation)

	done := true
	updated, err = coordinator.PatchChecklistItem(ctx, card.ID, first.ID, ChecklistItemPatch{Done: &done})
	require.NoError(t, err)
	require.True(t, updated.Checklist[0].Done)
	require.False(t, updated.Checklist[1].Done)
	text := "Refactor"
	updated, err = coordinator.PatchChecklistItem(ctx, card.ID, second.ID, ChecklistItemPatch{Text: &text})
	require.NoError(t, err)
	require.Equal(t, "Refactor", updated.Checklist[1].Text)
	_, err = coordinator.PatchChecklistItem(ctx, card.ID, "missing", ChecklistItemPatch{Done: &done})
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestCreateCardFromTemplate(t *testing.T) {
	ctx := context.Background()
	coordinator := openTestCoordinator(t)
	board, first, second := createHierarchy(t, coordinator)
	createCard(t, coordinator, board, second, "Existing")
	priority, offset := "High", 3
	template := domain.CardTemplate{
		BoardID: board.ID, Name: "Bug", Title: "Fix bug", Description: "Steps", Priority: &priority, DueOffsetDays: &offset,
		Tags: []string{"bug"}, Checklist: []domain.ChecklistItem{{ID: "reproduce", Text: "Reproduce", Position: 1024}}, Position: 1024,
	}
	require.NoError(t, coordinator.CreateCardTemplate(ctx, &template))

	card, err := coordinator.CreateCardFromTemplate(ctx, template.ID, second.ID)
	require.NoError(t, err)
	require.Equal(t, "Fix bug", card.Title)
	require.Equal(t, []string{"bug"}, card.Tags)
	require.Len(t, card.Checklist, 1)
	require.Equal(t, 2*positionSpacing, card.Position)
	require.NotNil(t, card.DueDate)

	_, err = coordinator.CreateCardFromTemplate(ctx, "missing", first.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestCardFromTemplateUsesLocalDateOffset(t *testing.T) {
	offset := 2
	now := time.Date(2026, 3, 10, 18, 30, 0, 0, time.Local)
	card := CardFromTemplate(domain.CardTemplate{Title: "Task", DueOffsetDays: &offset}, "board", "column", 5, now)
	expected := time.Date(2026, 3, 12, 0, 0, 0, 0, time.Local).UTC()
	require.Equal(t, expected, *card.DueDate)
	require.Equal(t, 5.0, card.Position)
	require.NotNil(t, card.Fields)
}

func TestPatchCardRejectsStaleExpectedUpdatedAt(t *testing.T) {
	ctx := context.Background()
	coordinator := openTestCoordinator(t)
	board, column, _ := createHierarchy(t, coordinator)
	card := createCard(t, coordinator, board, column, "Shared")
	loaded := card.UpdatedAt

	title := "First writer"
	_, err := coordinator.PatchCard(ctx, card.ID, CardPatch{Title: &title, ExpectedUpdatedAt: &loaded})
	require.NoError(t, err)

	title = "Stale writer"
	_, err = coordinator.PatchCard(ctx, card.ID, CardPatch{Title: &title, ExpectedUpdatedAt: &loaded})
	require.ErrorIs(t, err, domain.ErrConflict)
	stored, err := coordinator.GetCard(ctx, card.ID)
	require.NoError(t, err)
	require.Equal(t, "First writer", stored.Title)
}
