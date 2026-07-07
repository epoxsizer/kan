package domain

import (
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	t.Parallel()
	require.ErrorIs(t, ValidateProject(Project{Name: "", Position: 1}), ErrValidation)
	require.ErrorIs(t, ValidateCard(Card{BoardID: "b", ColumnID: "c", Title: "card", Position: math.NaN()}), ErrValidation)
	require.NoError(t, ValidateFieldDef(FieldDef{BoardID: "b", Key: "area", Label: "Area", Type: FieldSelect, Options: json.RawMessage(`["TUI"]`), Position: 1}))
	err := ValidateFieldDef(FieldDef{BoardID: "b", Key: "bad key", Label: "Bad", Type: FieldText, Position: 1})
	require.True(t, errors.Is(err, ErrValidation))
	offset := 7
	require.NoError(t, ValidateCardTemplate(CardTemplate{BoardID: "b", Name: "Bug", Title: "Fix bug", Position: 1, DueOffsetDays: &offset, Checklist: []ChecklistItem{{ID: "one", Text: "Reproduce", Position: 1}}}))
	negative := -1
	require.ErrorIs(t, ValidateCardTemplate(CardTemplate{BoardID: "b", Name: "Bug", Title: "Fix bug", Position: 1, DueOffsetDays: &negative}), ErrValidation)
	require.ErrorIs(t, ValidateCardTemplate(CardTemplate{BoardID: "b", Name: "", Title: "Fix bug", Position: 1}), ErrValidation)
}

func TestCardFieldValueTypes(t *testing.T) {
	t.Parallel()
	base := Card{BoardID: "b", ColumnID: "c", Title: "card", Position: 1}
	tests := []struct {
		name  string
		field FieldValue
		valid bool
	}{
		{name: "text", field: FieldValue{Type: FieldText, Value: "value"}, valid: true},
		{name: "number", field: FieldValue{Type: FieldNumber, Value: 2.5}, valid: true},
		{name: "date", field: FieldValue{Type: FieldDate, Value: "2026-06-22"}, valid: true},
		{name: "checkbox", field: FieldValue{Type: FieldCheckbox, Value: true}, valid: true},
		{name: "url", field: FieldValue{Type: FieldURL, Value: "https://example.com"}, valid: true},
		{name: "bad number", field: FieldValue{Type: FieldNumber, Value: "two"}, valid: false},
		{name: "bad date", field: FieldValue{Type: FieldDate, Value: "tomorrow"}, valid: false},
		{name: "bad checkbox", field: FieldValue{Type: FieldCheckbox, Value: "yes"}, valid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			card := base
			card.Fields = map[string]FieldValue{"value": test.field}
			err := ValidateCard(card)
			if test.valid {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, ErrValidation)
			}
		})
	}
}
