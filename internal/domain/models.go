package domain

import (
	"encoding/json"
	"time"
)

type FieldType string

const (
	FieldText     FieldType = "text"
	FieldNumber   FieldType = "number"
	FieldDate     FieldType = "date"
	FieldSelect   FieldType = "select"
	FieldCheckbox FieldType = "checkbox"
	FieldURL      FieldType = "url"
)

type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Position    float64   `json:"position"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Board struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Position    float64   `json:"position"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Column struct {
	ID               string    `json:"id"`
	BoardID          string    `json:"board_id"`
	Name             string    `json:"name"`
	Position         float64   `json:"position"`
	WIPLimit         *int      `json:"wip_limit,omitempty"`
	Color            *string   `json:"color,omitempty"`
	AutoArchive      bool      `json:"auto_archive,omitempty"`
	ArchiveAfterDays int       `json:"archive_after_days,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type FieldValue struct {
	Type  FieldType `json:"type"`
	Value any       `json:"value"`
}

type ChecklistItem struct {
	ID       string  `json:"id"`
	Text     string  `json:"text"`
	Done     bool    `json:"done"`
	Position float64 `json:"position"`
}

type Card struct {
	ID              string                `json:"id"`
	BoardID         string                `json:"board_id"`
	ColumnID        string                `json:"column_id"`
	Title           string                `json:"title"`
	Description     string                `json:"description"`
	Position        float64               `json:"position"`
	Priority        *string               `json:"priority,omitempty"`
	DueDate         *time.Time            `json:"due_date,omitempty"`
	Tags            []string              `json:"tags"`
	RelatedCardIDs  []string              `json:"related_card_ids"`
	Checklist       []ChecklistItem       `json:"checklist"`
	Fields          map[string]FieldValue `json:"fields"`
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
	DeletedAt       *time.Time            `json:"deleted_at,omitempty"`
	ColumnEnteredAt time.Time             `json:"column_entered_at,omitempty"`
}

type CardTemplate struct {
	ID            string          `json:"id"`
	BoardID       string          `json:"board_id"`
	Name          string          `json:"name"`
	Title         string          `json:"title"`
	Description   string          `json:"description"`
	Priority      *string         `json:"priority,omitempty"`
	DueOffsetDays *int            `json:"due_offset_days,omitempty"`
	Tags          []string        `json:"tags"`
	Checklist     []ChecklistItem `json:"checklist"`
	Position      float64         `json:"position"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type FieldDef struct {
	ID        string          `json:"id"`
	BoardID   string          `json:"board_id"`
	Key       string          `json:"key"`
	Label     string          `json:"label"`
	Type      FieldType       `json:"type"`
	Options   json.RawMessage `json:"options"`
	Required  bool            `json:"required"`
	Position  float64         `json:"position"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}
