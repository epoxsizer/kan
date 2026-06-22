package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gitlab.digital-spirit.ru/solutions/common/kan/internal/domain"
)

const timestampFormat = time.RFC3339Nano

type scanner interface {
	Scan(...any) error
}

func prepareIdentity(id *string, createdAt, updatedAt *time.Time) {
	if *id == "" {
		*id = uuid.NewString()
	}
	now := domain.UTCNow()
	if createdAt.IsZero() {
		*createdAt = now
	} else {
		*createdAt = createdAt.UTC()
	}
	*updatedAt = now
}

func encodeTime(value time.Time) string { return value.UTC().Format(timestampFormat) }

func encodeOptionalTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return encodeTime(*value)
}

func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(timestampFormat, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse timestamp %q: %w", value, err)
	}
	return parsed.UTC(), nil
}

func parseOptionalTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	parsed, err := parseTime(value.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func jsonValue(value any, fallback string) (string, error) {
	if value == nil {
		return fallback, nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func ensureAffected(result sql.Result, err error) error {
	if err != nil {
		return mapError(err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return domain.ErrNotFound
	}
	return nil
}
