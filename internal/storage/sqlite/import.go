package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/epoxsizer/kan/internal/domain"
)

func (repo *Repository) ImportDocument(ctx context.Context, document domain.ExportDocument, replace bool) error {
	if document.Format != "kan" {
		return fmt.Errorf("%w: unsupported import format %q", domain.ErrValidation, document.Format)
	}
	if document.Version < 1 || document.Version > domain.ExportVersion {
		return fmt.Errorf("%w: unsupported import version %d", domain.ErrValidation, document.Version)
	}
	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var existing int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects`).Scan(&existing); err != nil {
		return err
	}
	if existing > 0 && !replace {
		return fmt.Errorf("%w: database is not empty; use --replace --yes", domain.ErrConflict)
	}
	if replace {
		if _, err = tx.ExecContext(ctx, `DELETE FROM projects`); err != nil {
			return mapError(err)
		}
	}

	now := domain.UTCNow()
	links := map[string][]string{}
	for projectIndex := range document.Projects {
		exportedProject := document.Projects[projectIndex]
		project := exportedProject.Project
		if project.ID == "" {
			return importValidation("project ID is required")
		}
		if err = domain.ValidateProject(project); err != nil {
			return err
		}
		project.CreatedAt, project.UpdatedAt = importTimes(project.CreatedAt, project.UpdatedAt, now)
		if _, err = tx.ExecContext(ctx, `INSERT INTO projects(id,name,description,position,created_at,updated_at) VALUES(?,?,?,?,?,?)`, project.ID, project.Name, project.Description, project.Position, encodeTime(project.CreatedAt), encodeTime(project.UpdatedAt)); err != nil {
			return mapError(err)
		}

		for boardIndex := range exportedProject.Boards {
			exportedBoard := exportedProject.Boards[boardIndex]
			board := exportedBoard.Board
			if board.ID == "" || board.ProjectID != project.ID {
				return importValidation("board ID and project relationship must match its hierarchy")
			}
			if err = domain.ValidateBoard(board); err != nil {
				return err
			}
			board.CreatedAt, board.UpdatedAt = importTimes(board.CreatedAt, board.UpdatedAt, now)
			if _, err = tx.ExecContext(ctx, `INSERT INTO boards(id,project_id,name,description,position,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, board.ID, board.ProjectID, board.Name, board.Description, board.Position, encodeTime(board.CreatedAt), encodeTime(board.UpdatedAt)); err != nil {
				return mapError(err)
			}

			for defIndex := range exportedBoard.FieldDefs {
				def := exportedBoard.FieldDefs[defIndex]
				if def.ID == "" || def.BoardID != board.ID {
					return importValidation("field definition ID and board relationship must match its hierarchy")
				}
				if len(def.Options) == 0 {
					def.Options = json.RawMessage(`[]`)
				}
				if err = domain.ValidateFieldDef(def); err != nil {
					return err
				}
				def.CreatedAt, def.UpdatedAt = importTimes(def.CreatedAt, def.UpdatedAt, now)
				if _, err = tx.ExecContext(ctx, `INSERT INTO field_defs(id,board_id,key,label,type,options,required,position,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, def.ID, def.BoardID, def.Key, def.Label, def.Type, string(def.Options), def.Required, def.Position, encodeTime(def.CreatedAt), encodeTime(def.UpdatedAt)); err != nil {
					return mapError(err)
				}
			}

			for columnIndex := range exportedBoard.Columns {
				exportedColumn := exportedBoard.Columns[columnIndex]
				column := exportedColumn.Column
				if column.ID == "" || column.BoardID != board.ID {
					return importValidation("column ID and board relationship must match its hierarchy")
				}
				if column.ArchiveAfterDays == 0 {
					column.ArchiveAfterDays = 14
				}
				if err = domain.ValidateColumn(column); err != nil {
					return err
				}
				column.CreatedAt, column.UpdatedAt = importTimes(column.CreatedAt, column.UpdatedAt, now)
				if _, err = tx.ExecContext(ctx, `INSERT INTO board_columns(id,board_id,name,position,wip_limit,color,auto_archive,archive_after_days,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, column.ID, column.BoardID, column.Name, column.Position, column.WIPLimit, column.Color, column.AutoArchive, column.ArchiveAfterDays, encodeTime(column.CreatedAt), encodeTime(column.UpdatedAt)); err != nil {
					return mapError(err)
				}

				for cardIndex := range exportedColumn.Cards {
					card := exportedColumn.Cards[cardIndex]
					if card.ID == "" || card.BoardID != board.ID || card.ColumnID != column.ID {
						return importValidation("card ID, board, and column relationships must match its hierarchy")
					}
					if card.Tags == nil {
						card.Tags = []string{}
					}
					if card.Fields == nil {
						card.Fields = map[string]domain.FieldValue{}
					}
					if card.Checklist == nil {
						card.Checklist = []domain.ChecklistItem{}
					}
					if err = domain.ValidateCard(card); err != nil {
						return err
					}
					card.CreatedAt, card.UpdatedAt = importTimes(card.CreatedAt, card.UpdatedAt, now)
					if card.ColumnEnteredAt.IsZero() {
						card.ColumnEnteredAt = card.UpdatedAt
					}
					tags, encodeErr := jsonValue(card.Tags, `[]`)
					if encodeErr != nil {
						return encodeErr
					}
					fields, encodeErr := jsonValue(card.Fields, `{}`)
					if encodeErr != nil {
						return encodeErr
					}
					checklist, encodeErr := jsonValue(card.Checklist, `[]`)
					if encodeErr != nil {
						return encodeErr
					}
					if _, err = tx.ExecContext(ctx, `INSERT INTO cards(id,board_id,column_id,title,description,position,priority,due_date,tags,fields,checklist,created_at,updated_at,deleted_at,column_entered_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, card.ID, card.BoardID, card.ColumnID, card.Title, card.Description, card.Position, card.Priority, encodeOptionalTime(card.DueDate), tags, fields, checklist, encodeTime(card.CreatedAt), encodeTime(card.UpdatedAt), encodeOptionalTime(card.DeletedAt), encodeTime(card.ColumnEnteredAt)); err != nil {
						return mapError(err)
					}
					links[card.ID] = append([]string(nil), card.RelatedCardIDs...)
				}
			}
		}
	}

	for cardID, relatedIDs := range links {
		for _, relatedID := range relatedIDs {
			if cardID == relatedID {
				return importValidation("a card cannot relate to itself")
			}
			var cardProject, relatedProject string
			if err = tx.QueryRowContext(ctx, `SELECT b.project_id FROM cards c JOIN boards b ON b.id=c.board_id WHERE c.id=?`, cardID).Scan(&cardProject); err != nil {
				return mapError(err)
			}
			if err = tx.QueryRowContext(ctx, `SELECT b.project_id FROM cards c JOIN boards b ON b.id=c.board_id WHERE c.id=?`, relatedID).Scan(&relatedProject); err != nil {
				return mapError(err)
			}
			if cardProject != relatedProject {
				return importValidation("related cards must belong to the same project")
			}
			left, right := cardID, relatedID
			if left > right {
				left, right = right, left
			}
			if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO card_links(card_id,related_card_id,created_at) VALUES(?,?,?)`, left, right, encodeTime(now)); err != nil {
				return mapError(err)
			}
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit import: %w", err)
	}
	return nil
}

func importTimes(created, updated, fallback time.Time) (time.Time, time.Time) {
	if created.IsZero() {
		created = fallback
	}
	if updated.IsZero() {
		updated = created
	}
	return created.UTC(), updated.UTC()
}

func importValidation(message string) error {
	return fmt.Errorf("%w: %s", domain.ErrValidation, message)
}
