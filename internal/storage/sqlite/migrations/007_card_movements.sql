CREATE TABLE card_movements (
    id TEXT PRIMARY KEY,
    card_id TEXT NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    from_column_id TEXT,
    to_column_id TEXT NOT NULL,
    actor TEXT NOT NULL DEFAULT '',
    moved_at TEXT NOT NULL
);

CREATE INDEX idx_card_movements_card ON card_movements(card_id, moved_at);
