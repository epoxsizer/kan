ALTER TABLE cards ADD COLUMN checklist TEXT NOT NULL DEFAULT '[]'
    CHECK (json_valid(checklist) AND json_type(checklist) = 'array');

DROP TRIGGER cards_fts_insert;
DROP TRIGGER cards_fts_update;

CREATE TRIGGER cards_fts_insert AFTER INSERT ON cards
WHEN NEW.deleted_at IS NULL
BEGIN
    INSERT INTO card_fts(card_id, board_id, title, description, metadata)
    VALUES (
        NEW.id,
        NEW.board_id,
        NEW.title,
        NEW.description,
        COALESCE((SELECT group_concat(CAST(atom AS TEXT), ' ') FROM json_tree(NEW.fields) WHERE atom IS NOT NULL), '') || ' ' ||
        COALESCE((SELECT group_concat(CAST(value AS TEXT), ' ') FROM json_each(NEW.tags)), '') || ' ' ||
        COALESCE((SELECT group_concat(json_extract(value, '$.text'), ' ') FROM json_each(NEW.checklist)), '')
    );
END;

CREATE TRIGGER cards_fts_update AFTER UPDATE ON cards
BEGIN
    DELETE FROM card_fts WHERE card_id = OLD.id;
    INSERT INTO card_fts(card_id, board_id, title, description, metadata)
    SELECT
        NEW.id,
        NEW.board_id,
        NEW.title,
        NEW.description,
        COALESCE((SELECT group_concat(CAST(atom AS TEXT), ' ') FROM json_tree(NEW.fields) WHERE atom IS NOT NULL), '') || ' ' ||
        COALESCE((SELECT group_concat(CAST(value AS TEXT), ' ') FROM json_each(NEW.tags)), '') || ' ' ||
        COALESCE((SELECT group_concat(json_extract(value, '$.text'), ' ') FROM json_each(NEW.checklist)), '')
    WHERE NEW.deleted_at IS NULL;
END;

UPDATE cards SET checklist = checklist;
