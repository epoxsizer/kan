package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/epoxsizer/kan/internal/seed"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func openTestRepository(t *testing.T) *Repository {
	t.Helper()
	path := "file:" + uuid.NewString() + "?mode=memory&cache=shared"
	repo, err := Open(context.Background(), path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, repo.Close()) })
	return repo
}

func createHierarchy(t *testing.T, repo *Repository) (domain.Project, domain.Board, domain.Column) {
	t.Helper()
	ctx := context.Background()
	project := domain.Project{Name: "Project", Position: 1024}
	require.NoError(t, repo.CreateProject(ctx, &project))
	board := domain.Board{ProjectID: project.ID, Name: "Board", Position: 1024}
	require.NoError(t, repo.CreateBoard(ctx, &board))
	column := domain.Column{BoardID: board.ID, Name: "Backlog", Position: 1024}
	require.NoError(t, repo.CreateColumn(ctx, &column))
	return project, board, column
}

func TestMigrationsAreRepeatableAndEnableFeatures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kan.db")
	ctx := context.Background()
	repo, err := Open(ctx, path)
	require.NoError(t, err)
	var foreignKeys int
	require.NoError(t, repo.db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys))
	require.Equal(t, 1, foreignKeys)
	var journalMode string
	require.NoError(t, repo.db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode))
	require.Equal(t, "wal", journalMode)
	require.NoError(t, repo.Close())

	repo, err = Open(ctx, path)
	require.NoError(t, err)
	var migrationCount int
	require.NoError(t, repo.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&migrationCount))
	require.Equal(t, 4, migrationCount)
	require.NoError(t, repo.Close())
}

func TestColumnCardArchiving(t *testing.T) {
	repo := openTestRepository(t)
	ctx := context.Background()
	_, board, column := createHierarchy(t, repo)
	column.AutoArchive = true
	column.ArchiveAfterDays = 14
	require.NoError(t, repo.UpdateColumn(ctx, &column))

	expired := domain.Card{BoardID: board.ID, ColumnID: column.ID, Title: "Expired", Position: 1024}
	current := domain.Card{BoardID: board.ID, ColumnID: column.ID, Title: "Current", Position: 2048}
	require.NoError(t, repo.CreateCard(ctx, &expired))
	require.NoError(t, repo.CreateCard(ctx, &current))
	past := encodeTime(domain.UTCNow().AddDate(0, 0, -15))
	require.NoError(t, ensureAffected(repo.db.ExecContext(ctx, `UPDATE cards SET column_entered_at=? WHERE id=?`, past, expired.ID)))

	count, err := repo.ArchiveExpiredCards(ctx, board.ID)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	cards, err := repo.ListCards(ctx, board.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"Current"}, []string{cards[0].Title})

	count, err = repo.ArchiveCardsInColumn(ctx, column.ID)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	cards, err = repo.ListCards(ctx, board.ID)
	require.NoError(t, err)
	require.Empty(t, cards)
}

func TestRepositoryCRUDSearchSoftDeleteAndCascade(t *testing.T) {
	repo := openTestRepository(t)
	ctx := context.Background()
	project, board, column := createHierarchy(t, repo)

	project.Name = "Renamed"
	require.NoError(t, repo.UpdateProject(ctx, &project))
	loadedProject, err := repo.GetProject(ctx, project.ID)
	require.NoError(t, err)
	require.Equal(t, "Renamed", loadedProject.Name)

	wip := 3
	color := "blue"
	column.WIPLimit, column.Color = &wip, &color
	require.NoError(t, repo.UpdateColumn(ctx, &column))
	columns, err := repo.ListColumns(ctx, board.ID)
	require.NoError(t, err)
	require.Equal(t, 3, *columns[0].WIPLimit)

	fieldDef := domain.FieldDef{BoardID: board.ID, Key: "area", Label: "Area", Type: domain.FieldSelect, Options: json.RawMessage(`["Storage","TUI"]`), Required: true, Position: 1024}
	require.NoError(t, repo.CreateFieldDef(ctx, &fieldDef))
	fieldDef.Label = "Component"
	require.NoError(t, repo.UpdateFieldDef(ctx, &fieldDef))
	defs, err := repo.ListFieldDefs(ctx, board.ID)
	require.NoError(t, err)
	require.Equal(t, "Component", defs[0].Label)

	due := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Microsecond)
	priority := "high"
	card := domain.Card{BoardID: board.ID, ColumnID: column.ID, Title: "Build repository", Description: "SQLite persistence layer", Position: 1024, Priority: &priority, DueDate: &due, Tags: []string{"backend", "urgent"}, Checklist: []domain.ChecklistItem{{ID: "check-1", Text: "Verify deployment", Done: false, Position: 1024}}, Fields: map[string]domain.FieldValue{"area": {Type: domain.FieldSelect, Value: "Storage"}, "owner": {Type: domain.FieldText, Value: "Ada"}}}
	require.NoError(t, repo.CreateCard(ctx, &card))
	loaded, err := repo.GetCard(ctx, card.ID)
	require.NoError(t, err)
	require.Equal(t, card.Tags, loaded.Tags)
	require.Equal(t, "Storage", loaded.Fields["area"].Value)
	require.Equal(t, due, *loaded.DueDate)
	require.Equal(t, card.Checklist, loaded.Checklist)

	for _, query := range []string{"repository", "persistence", "backend", "Storage", "Ada", "deployment"} {
		results, searchErr := repo.SearchCards(ctx, board.ID, query)
		require.NoError(t, searchErr, query)
		require.Len(t, results, 1, query)
	}
	results, err := repo.SearchCards(ctx, board.ID, `"depl"*`)
	require.NoError(t, err)
	require.Len(t, results, 1)
	card.Title = "Updated title"
	require.NoError(t, repo.UpdateCard(ctx, &card))
	results, err = repo.SearchCards(ctx, board.ID, "Updated")
	require.NoError(t, err)
	require.Len(t, results, 1)

	require.NoError(t, repo.DeleteCard(ctx, card.ID))
	_, err = repo.GetCard(ctx, card.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
	results, err = repo.SearchCards(ctx, board.ID, "Updated")
	require.NoError(t, err)
	require.Empty(t, results)
	require.NoError(t, repo.RestoreCard(ctx, card.ID))
	restored, err := repo.GetCard(ctx, card.ID)
	require.NoError(t, err)
	require.Nil(t, restored.DeletedAt)
	results, err = repo.SearchCards(ctx, board.ID, "Updated")
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NoError(t, repo.DeleteCard(ctx, card.ID))
	require.ErrorIs(t, repo.DeleteCard(ctx, card.ID), domain.ErrNotFound)

	require.NoError(t, repo.DeleteProject(ctx, project.ID))
	_, err = repo.GetBoard(ctx, board.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestRelationshipConstraintsAndErrors(t *testing.T) {
	repo := openTestRepository(t)
	ctx := context.Background()
	_, board, column := createHierarchy(t, repo)
	otherProject := domain.Project{Name: "Other", Position: 2048}
	require.NoError(t, repo.CreateProject(ctx, &otherProject))
	otherBoard := domain.Board{ProjectID: otherProject.ID, Name: "Other", Position: 1024}
	require.NoError(t, repo.CreateBoard(ctx, &otherBoard))

	card := domain.Card{BoardID: otherBoard.ID, ColumnID: column.ID, Title: "Invalid", Position: 1}
	err := repo.CreateCard(ctx, &card)
	require.True(t, errors.Is(err, domain.ErrConflict), err)
	_, err = repo.GetBoard(ctx, "missing")
	require.ErrorIs(t, err, domain.ErrNotFound)

	duplicate := domain.FieldDef{BoardID: board.ID, Key: "owner", Label: "Owner", Type: domain.FieldText, Options: json.RawMessage(`[]`), Position: 1}
	require.NoError(t, repo.CreateFieldDef(ctx, &duplicate))
	duplicate.ID = ""
	err = repo.CreateFieldDef(ctx, &duplicate)
	require.ErrorIs(t, err, domain.ErrConflict)
}

func TestMoveCardPersistsOrderAndColumn(t *testing.T) {
	repo := openTestRepository(t)
	ctx := context.Background()
	_, board, backlog := createHierarchy(t, repo)
	done := domain.Column{BoardID: board.ID, Name: "Done", Position: 2048}
	require.NoError(t, repo.CreateColumn(ctx, &done))

	first := domain.Card{BoardID: board.ID, ColumnID: backlog.ID, Title: "First", Position: 1024}
	second := domain.Card{BoardID: board.ID, ColumnID: backlog.ID, Title: "Second", Position: 2048}
	third := domain.Card{BoardID: board.ID, ColumnID: backlog.ID, Title: "Third", Position: 3072}
	for _, card := range []*domain.Card{&first, &second, &third} {
		require.NoError(t, repo.CreateCard(ctx, card))
	}

	require.NoError(t, repo.MoveCard(ctx, third.ID, backlog.ID, 0))
	require.NoError(t, repo.MoveCard(ctx, first.ID, done.ID, 0))
	cards, err := repo.ListCards(ctx, board.ID)
	require.NoError(t, err)
	byColumn := map[string][]string{}
	for _, card := range cards {
		byColumn[card.ColumnID] = append(byColumn[card.ColumnID], card.Title)
	}
	require.Equal(t, []string{"Third", "Second"}, byColumn[backlog.ID])
	require.Equal(t, []string{"First"}, byColumn[done.ID])
}

func TestMoveColumnPersistsOrderAndRenormalizesPositions(t *testing.T) {
	repo := openTestRepository(t)
	ctx := context.Background()
	_, board, _ := createHierarchy(t, repo)
	second := domain.Column{BoardID: board.ID, Name: "Doing", Position: 1024 + 1e-8}
	third := domain.Column{BoardID: board.ID, Name: "Done", Position: 3072}
	require.NoError(t, repo.CreateColumn(ctx, &second))
	require.NoError(t, repo.CreateColumn(ctx, &third))

	require.NoError(t, repo.MoveColumn(ctx, third.ID, 0))
	columns, err := repo.ListColumns(ctx, board.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"Done", "Backlog", "Doing"}, []string{columns[0].Name, columns[1].Name, columns[2].Name})
	require.Equal(t, []float64{1024, 2048, 3072}, []float64{columns[0].Position, columns[1].Position, columns[2].Position})

	require.NoError(t, repo.MoveColumn(ctx, third.ID, 99))
	columns, err = repo.ListColumns(ctx, board.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"Backlog", "Doing", "Done"}, []string{columns[0].Name, columns[1].Name, columns[2].Name})
	require.ErrorIs(t, repo.MoveColumn(ctx, "missing", 0), domain.ErrNotFound)
}

func TestMoveCardEnforcesWIPAndRenormalizesPositions(t *testing.T) {
	repo := openTestRepository(t)
	ctx := context.Background()
	_, board, source := createHierarchy(t, repo)
	limit := 1
	target := domain.Column{BoardID: board.ID, Name: "Limited", Position: 2048, WIPLimit: &limit}
	require.NoError(t, repo.CreateColumn(ctx, &target))

	moving := domain.Card{BoardID: board.ID, ColumnID: source.ID, Title: "Moving", Position: 3072}
	left := domain.Card{BoardID: board.ID, ColumnID: target.ID, Title: "Left", Position: 1024}
	require.NoError(t, repo.CreateCard(ctx, &moving))
	require.NoError(t, repo.CreateCard(ctx, &left))
	require.ErrorIs(t, repo.MoveCard(ctx, moving.ID, target.ID, 1), domain.ErrConflict)

	limit = 3
	target.WIPLimit = &limit
	require.NoError(t, repo.UpdateColumn(ctx, &target))
	right := domain.Card{BoardID: board.ID, ColumnID: target.ID, Title: "Right", Position: 1024 + 1e-8}
	require.NoError(t, repo.CreateCard(ctx, &right))
	require.NoError(t, repo.MoveCard(ctx, moving.ID, target.ID, 1))

	cards, err := repo.ListCards(ctx, board.ID)
	require.NoError(t, err)
	titles := []string{}
	positions := []float64{}
	for _, card := range cards {
		if card.ColumnID == target.ID {
			titles = append(titles, card.Title)
			positions = append(positions, card.Position)
		}
	}
	require.Equal(t, []string{"Left", "Moving", "Right"}, titles)
	require.Greater(t, positions[1]-positions[0], 1.0)
	require.Greater(t, positions[2]-positions[1], 1.0)
}

func TestRelatedCardsAreSymmetricAndLimitedToProject(t *testing.T) {
	repo := openTestRepository(t)
	ctx := context.Background()
	project, board, firstColumn := createHierarchy(t, repo)
	secondBoard := domain.Board{ProjectID: project.ID, Name: "Second board", Position: 2048}
	require.NoError(t, repo.CreateBoard(ctx, &secondBoard))
	secondColumn := domain.Column{BoardID: secondBoard.ID, Name: "Queue", Position: 1024}
	require.NoError(t, repo.CreateColumn(ctx, &secondColumn))

	first := domain.Card{BoardID: board.ID, ColumnID: firstColumn.ID, Title: "First", Position: 1024}
	second := domain.Card{BoardID: secondBoard.ID, ColumnID: secondColumn.ID, Title: "Second", Position: 1024}
	require.NoError(t, repo.CreateCard(ctx, &first))
	require.NoError(t, repo.CreateCard(ctx, &second))
	first.RelatedCardIDs = []string{second.ID, second.ID}
	require.NoError(t, repo.UpdateCard(ctx, &first))

	loadedFirst, err := repo.GetCard(ctx, first.ID)
	require.NoError(t, err)
	require.Equal(t, []string{second.ID}, loadedFirst.RelatedCardIDs)
	loadedSecond, err := repo.GetCard(ctx, second.ID)
	require.NoError(t, err)
	require.Equal(t, []string{first.ID}, loadedSecond.RelatedCardIDs)

	otherProject := domain.Project{Name: "Other project", Position: 2048}
	require.NoError(t, repo.CreateProject(ctx, &otherProject))
	otherBoard := domain.Board{ProjectID: otherProject.ID, Name: "Other board", Position: 1024}
	require.NoError(t, repo.CreateBoard(ctx, &otherBoard))
	otherColumn := domain.Column{BoardID: otherBoard.ID, Name: "Other", Position: 1024}
	require.NoError(t, repo.CreateColumn(ctx, &otherColumn))
	other := domain.Card{BoardID: otherBoard.ID, ColumnID: otherColumn.ID, Title: "Other", Position: 1024}
	require.NoError(t, repo.CreateCard(ctx, &other))
	loadedFirst.RelatedCardIDs = []string{other.ID}
	require.ErrorIs(t, repo.UpdateCard(ctx, &loadedFirst), domain.ErrConflict)
	loadedFirst.RelatedCardIDs = []string{first.ID}
	require.ErrorIs(t, repo.UpdateCard(ctx, &loadedFirst), domain.ErrValidation)

	require.NoError(t, repo.DeleteCard(ctx, second.ID))
	loadedFirst, err = repo.GetCard(ctx, first.ID)
	require.NoError(t, err)
	require.Empty(t, loadedFirst.RelatedCardIDs)
}

func TestUpdateCardRejectsStaleCopy(t *testing.T) {
	repo := openTestRepository(t)
	ctx := context.Background()
	_, board, column := createHierarchy(t, repo)
	card := domain.Card{BoardID: board.ID, ColumnID: column.ID, Title: "Original", Position: 1024}
	require.NoError(t, repo.CreateCard(ctx, &card))

	first, err := repo.GetCard(ctx, card.ID)
	require.NoError(t, err)
	stale, err := repo.GetCard(ctx, card.ID)
	require.NoError(t, err)
	first.Title = "First update"
	require.NoError(t, repo.UpdateCard(ctx, &first))
	stale.Title = "Stale update"
	require.ErrorIs(t, repo.UpdateCard(ctx, &stale), domain.ErrConflict)

	current, err := repo.GetCard(ctx, card.ID)
	require.NoError(t, err)
	require.Equal(t, "First update", current.Title)
}

func TestSeedIsIdempotent(t *testing.T) {
	repo := openTestRepository(t)
	ctx := context.Background()
	require.NoError(t, seed.Demo(ctx, repo))
	require.NoError(t, seed.Demo(ctx, repo))
	projects, err := repo.ListProjects(ctx)
	require.NoError(t, err)
	require.Len(t, projects, 3)
	cards, err := repo.ListCards(ctx, seed.BoardID)
	require.NoError(t, err)
	require.Len(t, cards, 3)
}

func TestAdvisoryLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kan.db")
	first, err := AcquireLock(path)
	require.NoError(t, err)
	defer first.Close()
	_, err = AcquireLock(path)
	require.ErrorIs(t, err, domain.ErrLocked)
	require.NoError(t, first.Close())
	second, err := AcquireLock(path)
	require.NoError(t, err)
	require.NoError(t, second.Close())
}

func TestAdvisoryLockWaitsForShortLivedCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kan.db")
	first, err := AcquireLock(path)
	require.NoError(t, err)
	released := make(chan struct{})
	go func() {
		time.Sleep(75 * time.Millisecond)
		_ = first.Close()
		close(released)
	}()
	second, err := AcquireLockTimeout(path, time.Second)
	require.NoError(t, err)
	require.NoError(t, second.Close())
	<-released
}

func TestBackupContainsAllData(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	source := filepath.Join(directory, "source.db")
	destination := filepath.Join(directory, "backup", "nightly.db")
	repo, err := Open(ctx, source)
	require.NoError(t, err)
	require.NoError(t, seed.Demo(ctx, repo))
	require.NoError(t, repo.Backup(ctx, destination))
	require.ErrorIs(t, repo.Backup(ctx, destination), domain.ErrConflict)
	require.NoError(t, repo.Close())

	info, err := os.Stat(destination)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	backup, err := Open(ctx, destination)
	require.NoError(t, err)
	defer backup.Close()
	projects, err := backup.ListProjects(ctx)
	require.NoError(t, err)
	require.Len(t, projects, 3)
	cards, err := backup.ListCards(ctx, seed.BoardID)
	require.NoError(t, err)
	require.Len(t, cards, 3)
	results, err := backup.SearchCards(ctx, seed.BoardID, "repository")
	require.NoError(t, err)
	require.Len(t, results, 1)
	var integrity string
	require.NoError(t, backup.db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity))
	require.Equal(t, "ok", integrity)
}
