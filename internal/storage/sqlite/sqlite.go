package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"

	"gitlab.digital-spirit.ru/solutions/common/kan/internal/config"
	"gitlab.digital-spirit.ru/solutions/common/kan/internal/domain"
)

type Repository struct {
	db *sql.DB
}

func Open(ctx context.Context, path string) (*Repository, error) {
	if !strings.Contains(path, "mode=memory") && path != ":memory:" {
		if err := config.EnsureParent(path); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	cleanup := func(openErr error) (*Repository, error) {
		db.Close()
		return nil, openErr
	}
	if _, err = db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return cleanup(fmt.Errorf("enable foreign keys: %w", err))
	}
	if _, err = db.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
		return cleanup(fmt.Errorf("set busy timeout: %w", err))
	}
	if !strings.Contains(path, "mode=memory") && path != ":memory:" {
		if _, err = db.ExecContext(ctx, "PRAGMA journal_mode = WAL"); err != nil {
			return cleanup(fmt.Errorf("enable WAL: %w", err))
		}
	}
	if err = migrate(ctx, db); err != nil {
		return cleanup(err)
	}
	return &Repository{db: db}, nil
}

func Migrate(ctx context.Context, path string) error {
	repo, err := Open(ctx, path)
	if err != nil {
		return err
	}
	return repo.Close()
}

func (repo *Repository) Close() error {
	return repo.db.Close()
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "constraint failed") || strings.Contains(message, "unique constraint") || strings.Contains(message, "foreign key constraint") {
		return fmt.Errorf("%w: %v", domain.ErrConflict, err)
	}
	if strings.Contains(message, "database is locked") || strings.Contains(message, "database is busy") {
		return fmt.Errorf("%w: %v", domain.ErrLocked, err)
	}
	return err
}
