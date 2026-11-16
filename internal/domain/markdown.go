package domain

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// CardMarkdown renders one card as a shareable Markdown document: title,
// metadata line, description, checklist, custom fields, and the card ID.
func CardMarkdown(card Card, columnName string) string {
	var builder strings.Builder
	builder.WriteString("# " + card.Title + "\n")
	metadata := []string{}
	if columnName != "" {
		metadata = append(metadata, "Status: "+columnName)
	}
	if card.Priority != nil && *card.Priority != "" {
		metadata = append(metadata, "Priority: "+*card.Priority)
	}
	if card.DueDate != nil {
		metadata = append(metadata, "Due: "+card.DueDate.Format("2006-01-02"))
	}
	if len(card.Tags) > 0 {
		metadata = append(metadata, "Tags: "+strings.Join(card.Tags, ", "))
	}
	if len(metadata) > 0 {
		builder.WriteString("\n" + strings.Join(metadata, " · ") + "\n")
	}
	if strings.TrimSpace(card.Description) != "" {
		builder.WriteString("\n" + strings.TrimRight(card.Description, "\n") + "\n")
	}
	if len(card.Checklist) > 0 {
		builder.WriteString("\n## Checklist\n\n")
		for _, item := range card.Checklist {
			mark := "[ ]"
			if item.Done {
				mark = "[x]"
			}
			builder.WriteString("- " + mark + " " + item.Text + "\n")
		}
	}
	if len(card.Fields) > 0 {
		builder.WriteString("\n## Fields\n\n")
		keys := make([]string, 0, len(card.Fields))
		for key := range card.Fields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			field := card.Fields[key]
			value, err := json.Marshal(field.Value)
			if err != nil {
				value = []byte("<invalid>")
			}
			builder.WriteString(fmt.Sprintf("- %s [%s]: %s\n", key, field.Type, value))
		}
	}
	builder.WriteString("\n---\nCard ID: " + card.ID + "\n")
	return builder.String()
}
