package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/spf13/cobra"
)

const positionSpacing = 1024.0

func writeJSON(cmd *cobra.Command, value any) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func withRepository(cmd *cobra.Command, opts *options, action func(context.Context, domain.Repository) error) error {
	res, err := open(cmd.Context(), *opts)
	if err != nil {
		return err
	}
	defer res.Close()
	return action(cmd.Context(), res.repo)
}

func requireDeleteConfirmation(yes bool) error {
	if !yes {
		return fmt.Errorf("destructive operation requires --yes")
	}
	return nil
}

func nextPosition[T any](values []T, position func(T) float64) float64 {
	maximum := 0.0
	for _, value := range values {
		maximum = math.Max(maximum, position(value))
	}
	return maximum + positionSpacing
}

func parseTags(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag == "" {
			continue
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		result = append(result, tag)
	}
	return result
}

func parseFields(value string) (map[string]domain.FieldValue, error) {
	if strings.TrimSpace(value) == "" {
		return map[string]domain.FieldValue{}, nil
	}
	var fields map[string]domain.FieldValue
	if err := json.Unmarshal([]byte(value), &fields); err != nil {
		return nil, fmt.Errorf("parse --fields JSON: %w", err)
	}
	if fields == nil {
		fields = map[string]domain.FieldValue{}
	}
	return fields, nil
}

func parseChecklist(value string) ([]domain.ChecklistItem, error) {
	if strings.TrimSpace(value) == "" {
		return []domain.ChecklistItem{}, nil
	}
	var items []domain.ChecklistItem
	if err := json.Unmarshal([]byte(value), &items); err != nil {
		return nil, fmt.Errorf("parse --checklist JSON: %w", err)
	}
	if items == nil {
		items = []domain.ChecklistItem{}
	}
	return items, nil
}

func parseDueDate(value string) (*time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, fmt.Errorf("due date must use YYYY-MM-DD: %w", err)
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func deletedResult(id string) map[string]string {
	return map[string]string{"deleted": id}
}
