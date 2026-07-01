ALTER TABLE board_columns ADD COLUMN auto_archive INTEGER NOT NULL DEFAULT 0 CHECK (auto_archive IN (0, 1));
ALTER TABLE board_columns ADD COLUMN archive_after_days INTEGER NOT NULL DEFAULT 14 CHECK (archive_after_days > 0);
ALTER TABLE cards ADD COLUMN column_entered_at TEXT;
UPDATE cards SET column_entered_at = updated_at WHERE column_entered_at IS NULL;
CREATE INDEX idx_cards_auto_archive ON cards(column_id, deleted_at, column_entered_at);
