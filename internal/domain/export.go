package domain

import "time"

const ExportVersion = 3

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
	Columns   []ExportColumn `json:"columns"`
}

type ExportColumn struct {
	Column
	Cards []Card `json:"cards"`
}
