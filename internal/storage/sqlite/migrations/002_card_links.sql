CREATE TABLE card_links (
    card_id TEXT NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    related_card_id TEXT NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,
    PRIMARY KEY (card_id, related_card_id),
    CHECK (card_id < related_card_id)
);

CREATE INDEX idx_card_links_related ON card_links(related_card_id);
