package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/epoxsizer/kan/internal/domain"
)

type boardHealth struct {
	overdueCount int
	nextDue      *time.Time
}

func summarizeBoardHealth(cards []domain.Card, now time.Time) boardHealth {
	health := boardHealth{}
	for _, card := range cards {
		if card.DueDate == nil {
			continue
		}
		if calendarDayDelta(*card.DueDate, now) < 0 {
			health.overdueCount++
			continue
		}
		if health.nextDue == nil || card.DueDate.Before(*health.nextDue) {
			value := *card.DueDate
			health.nextDue = &value
		}
	}
	return health
}

func boardHealthLabel(health boardHealth, now time.Time) string {
	if health.overdueCount == 1 {
		return "1 overdue"
	}
	if health.overdueCount > 1 {
		return fmt.Sprintf("%d overdue", health.overdueCount)
	}
	if health.nextDue == nil {
		return "no due dates"
	}
	return relativeDueLabel(health.nextDue, now)
}

func relativeDueLabel(due *time.Time, now time.Time) string {
	if due == nil {
		return "no due"
	}
	delta := calendarDayDelta(*due, now)
	switch {
	case delta < 0:
		return fmt.Sprintf("overdue %dd", -delta)
	case delta == 0:
		return "due today"
	case delta == 1:
		return "due tomorrow"
	case delta <= 7:
		return fmt.Sprintf("due in %dd", delta)
	default:
		return "due " + due.Format("2006-01-02")
	}
}

func calendarDayDelta(value, now time.Time) int {
	now = now.In(time.Local)
	valueDay := time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return int(valueDay.Sub(today) / (24 * time.Hour))
}

func (model *Model) cardMetadata(card domain.Card, now time.Time) string {
	parts := []string{}
	if card.Priority != nil && *card.Priority != "" {
		parts = append(parts, strings.ToUpper(*card.Priority))
	}
	parts = append(parts, relativeDueLabel(card.DueDate, now))
	if len(card.Checklist) > 0 {
		done := 0
		for _, item := range card.Checklist {
			if item.Done {
				done++
			}
		}
		parts = append(parts, fmt.Sprintf("✓%d/%d", done, len(card.Checklist)))
	}
	if model.showCardTags && len(card.Tags) > 0 {
		tag := "#" + card.Tags[0]
		if len(card.Tags) > 1 {
			tag += fmt.Sprintf(" +%d", len(card.Tags)-1)
		}
		parts = append(parts, tag)
	}
	if len(card.RelatedCardIDs) > 0 {
		parts = append(parts, fmt.Sprintf("↗%d", len(card.RelatedCardIDs)))
	}
	return strings.Join(parts, " · ")
}

func (model *Model) selectedCardBlock(card domain.Card, width, maxLines int) string {
	width = max(width, 1)
	maxLines = max(maxLines, 1)
	contentWidth := max(width-2, 1)
	lines := []string{truncate(card.Title, width)}
	if len(lines) < maxLines {
		lines = append(lines, "  "+truncate(model.cardMetadata(card, time.Now()), contentWidth))
	}
	if strings.TrimSpace(card.Description) != "" && len(lines) < maxLines {
		description := wrappedDetailLines([]string{markdownPlainText(card.Description)}, contentWidth)
		for _, line := range description {
			if len(lines) >= min(maxLines, 4) {
				break
			}
			lines = append(lines, "  "+truncate(line, contentWidth))
		}
	}
	return strings.Join(lines, "\n")
}
