CREATE TABLE card_templates (
    id TEXT PRIMARY KEY,
    board_id TEXT NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    priority TEXT,
    due_offset_days INTEGER CHECK (due_offset_days IS NULL OR due_offset_days >= 0),
    tags TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(tags) AND json_type(tags) = 'array'),
    checklist TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(checklist) AND json_type(checklist) = 'array'),
    position REAL NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (board_id, name)
);

CREATE INDEX idx_card_templates_board_position ON card_templates(board_id, position);
