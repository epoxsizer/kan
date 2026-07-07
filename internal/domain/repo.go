package domain

import "context"

type Repository interface {
	CreateProject(context.Context, *Project) error
	GetProject(context.Context, string) (Project, error)
	ListProjects(context.Context) ([]Project, error)
	UpdateProject(context.Context, *Project) error
	DeleteProject(context.Context, string) error

	CreateBoard(context.Context, *Board) error
	GetBoard(context.Context, string) (Board, error)
	ListBoards(context.Context, string) ([]Board, error)
	UpdateBoard(context.Context, *Board) error
	DeleteBoard(context.Context, string) error

	CreateColumn(context.Context, *Column) error
	GetColumn(context.Context, string) (Column, error)
	ListColumns(context.Context, string) ([]Column, error)
	UpdateColumn(context.Context, *Column) error
	MoveColumn(context.Context, string, int) error
	DeleteColumn(context.Context, string) error

	CreateCard(context.Context, *Card) error
	GetCard(context.Context, string) (Card, error)
	ListCards(context.Context, string) ([]Card, error)
	ListCardsIncludingDeleted(context.Context, string) ([]Card, error)
	UpdateCard(context.Context, *Card) error
	MoveCard(context.Context, string, string, int) error
	DeleteCard(context.Context, string) error
	RestoreCard(context.Context, string) error
	ArchiveCardsInColumn(context.Context, string) (int, error)
	ArchiveExpiredCards(context.Context, string) (int, error)

	CreateCardTemplate(context.Context, *CardTemplate) error
	GetCardTemplate(context.Context, string) (CardTemplate, error)
	ListCardTemplates(context.Context, string) ([]CardTemplate, error)
	UpdateCardTemplate(context.Context, *CardTemplate) error
	DeleteCardTemplate(context.Context, string) error

	CreateFieldDef(context.Context, *FieldDef) error
	GetFieldDef(context.Context, string) (FieldDef, error)
	ListFieldDefs(context.Context, string) ([]FieldDef, error)
	UpdateFieldDef(context.Context, *FieldDef) error
	DeleteFieldDef(context.Context, string) error

	SearchCards(context.Context, string, string) ([]Card, error)
	ImportDocument(context.Context, ExportDocument, bool) error
	Close() error
}
