CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    position REAL NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE boards (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    position REAL NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE board_columns (
    id TEXT PRIMARY KEY,
    board_id TEXT NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    position REAL NOT NULL,
    wip_limit INTEGER CHECK (wip_limit IS NULL OR wip_limit > 0),
    color TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (id, board_id)
);

CREATE TABLE cards (
    id TEXT PRIMARY KEY,
    board_id TEXT NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    column_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    position REAL NOT NULL,
    priority TEXT,
    due_date TEXT,
    tags TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(tags) AND json_type(tags) = 'array'),
    fields TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(fields) AND json_type(fields) = 'object'),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    deleted_at TEXT,
    FOREIGN KEY (column_id, board_id) REFERENCES board_columns(id, board_id) ON DELETE CASCADE
);

CREATE TABLE field_defs (
    id TEXT PRIMARY KEY,
    board_id TEXT NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    label TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('text', 'number', 'date', 'select', 'checkbox', 'url')),
    options TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(options)),
    required INTEGER NOT NULL DEFAULT 0 CHECK (required IN (0, 1)),
    position REAL NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (board_id, key)
);

CREATE INDEX idx_projects_position ON projects(position);
CREATE INDEX idx_boards_project_position ON boards(project_id, position);
CREATE INDEX idx_columns_board_position ON board_columns(board_id, position);
CREATE INDEX idx_cards_board ON cards(board_id);
CREATE INDEX idx_cards_column_position ON cards(column_id, position);
CREATE INDEX idx_field_defs_board_position ON field_defs(board_id, position);

CREATE VIRTUAL TABLE card_fts USING fts5(
    card_id UNINDEXED,
    board_id UNINDEXED,
    title,
    description,
    metadata,
    tokenize = 'unicode61'
);

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
        COALESCE((SELECT group_concat(CAST(value AS TEXT), ' ') FROM json_each(NEW.tags)), '')
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
        COALESCE((SELECT group_concat(CAST(value AS TEXT), ' ') FROM json_each(NEW.tags)), '')
    WHERE NEW.deleted_at IS NULL;
END;

CREATE TRIGGER cards_fts_delete AFTER DELETE ON cards
BEGIN
    DELETE FROM card_fts WHERE card_id = OLD.id;
END;
