package domain

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var fieldKeyPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

func ValidateProject(value Project) error {
	return validateNamed("project", value.Name, value.Position)
}

func ValidateBoard(value Board) error {
	if value.ProjectID == "" {
		return validationError("board project ID is required")
	}
	return validateNamed("board", value.Name, value.Position)
}

func ValidateColumn(value Column) error {
	if value.BoardID == "" {
		return validationError("column board ID is required")
	}
	if value.WIPLimit != nil && *value.WIPLimit < 1 {
		return validationError("column WIP limit must be positive")
	}
	if value.AutoArchive && value.ArchiveAfterDays < 1 {
		return validationError("column archive period must be positive")
	}
	return validateNamed("column", value.Name, value.Position)
}

func ValidateCard(value Card) error {
	if value.BoardID == "" || value.ColumnID == "" {
		return validationError("card board and column IDs are required")
	}
	if strings.TrimSpace(value.Title) == "" {
		return validationError("card title is required")
	}
	if !validPosition(value.Position) {
		return validationError("card position must be finite")
	}
	checklistIDs := map[string]struct{}{}
	for _, item := range value.Checklist {
		if item.ID == "" {
			return validationError("checklist item ID is required")
		}
		if strings.TrimSpace(item.Text) == "" {
			return validationError("checklist item text is required")
		}
		if !validPosition(item.Position) {
			return validationError("checklist item position must be finite")
		}
		if _, exists := checklistIDs[item.ID]; exists {
			return validationError("checklist item IDs must be unique")
		}
		checklistIDs[item.ID] = struct{}{}
	}
	for key, field := range value.Fields {
		if strings.TrimSpace(key) == "" {
			return validationError("card field key is required")
		}
		if !validFieldType(field.Type) {
			return validationError(fmt.Sprintf("card field %q has invalid type %q", key, field.Type))
		}
		if err := validateFieldValue(key, field); err != nil {
			return err
		}
	}
	if _, err := json.Marshal(value.Fields); err != nil {
		return validationError(fmt.Sprintf("card fields are not JSON-compatible: %v", err))
	}
	return nil
}

func validateFieldValue(key string, field FieldValue) error {
	if field.Value == nil {
		return nil
	}
	switch field.Type {
	case FieldText, FieldSelect:
		if _, ok := field.Value.(string); !ok {
			return validationError(fmt.Sprintf("card field %q must be text", key))
		}
	case FieldURL:
		text, ok := field.Value.(string)
		if !ok || !validURL(text) {
			return validationError(fmt.Sprintf("card field %q must be a valid URL", key))
		}
	case FieldNumber:
		switch field.Value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, json.Number:
		default:
			return validationError(fmt.Sprintf("card field %q must be a number", key))
		}
	case FieldCheckbox:
		if _, ok := field.Value.(bool); !ok {
			return validationError(fmt.Sprintf("card field %q must be a checkbox value", key))
		}
	case FieldDate:
		text, ok := field.Value.(string)
		if !ok {
			return validationError(fmt.Sprintf("card field %q must be a date in YYYY-MM-DD format", key))
		}
		if _, err := time.Parse("2006-01-02", text); err != nil {
			return validationError(fmt.Sprintf("card field %q must be a date in YYYY-MM-DD format", key))
		}
	}
	return nil
}

func ValidateFieldDef(value FieldDef) error {
	if value.BoardID == "" {
		return validationError("field definition board ID is required")
	}
	if !fieldKeyPattern.MatchString(value.Key) {
		return validationError("field definition key must start with a letter and contain only letters, numbers, underscores, or hyphens")
	}
	if strings.TrimSpace(value.Label) == "" {
		return validationError("field definition label is required")
	}
	if !validFieldType(value.Type) {
		return validationError(fmt.Sprintf("invalid field definition type %q", value.Type))
	}
	if !validPosition(value.Position) {
		return validationError("field definition position must be finite")
	}
	if value.Type == FieldSelect {
		var options []string
		if len(value.Options) == 0 || json.Unmarshal(value.Options, &options) != nil || len(options) == 0 {
			return validationError("select field definition requires a non-empty string options array")
		}
	} else if len(value.Options) > 0 && string(value.Options) != "[]" && string(value.Options) != "null" {
		return validationError("options are only valid for select fields")
	}
	return nil
}

func validateNamed(kind, name string, position float64) error {
	if strings.TrimSpace(name) == "" {
		return validationError(kind + " name is required")
	}
	if !validPosition(position) {
		return validationError(kind + " position must be finite")
	}
	return nil
}

func validPosition(position float64) bool {
	return !math.IsNaN(position) && !math.IsInf(position, 0)
}

func validFieldType(fieldType FieldType) bool {
	switch fieldType {
	case FieldText, FieldNumber, FieldDate, FieldSelect, FieldCheckbox, FieldURL:
		return true
	default:
		return false
	}
}

func validURL(value string) bool {
	parsed, err := url.ParseRequestURI(value)
	return err == nil && parsed.Scheme != "" && parsed.Host != ""
}

func validationError(message string) error {
	return fmt.Errorf("%w: %s", ErrValidation, message)
}

func UTCNow() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}
