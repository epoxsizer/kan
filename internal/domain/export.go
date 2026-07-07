package domain

import "time"

const ExportVersion = 4

type ExportDocument struct {
	Format     string          `json:"format"`
	Version    int             `json:"version"`
	ExportedAt time.Time       `json:"exported_at"`
	Projects   []ExportProject `json:"projects"`
}

type ExportProject struct {
	Project
	Boards []ExportBoard `json:"boards"`
}

type ExportBoard struct {
	Board
	FieldDefs []FieldDef     `json:"field_defs"`
	Templates []CardTemplate `json:"templates,omitempty"`
	Columns   []ExportColumn `json:"columns"`
}

type ExportColumn struct {
	Column
	Cards []Card `json:"cards"`
}
