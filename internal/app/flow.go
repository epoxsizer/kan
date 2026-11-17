package app

import (
	"context"
	"fmt"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/epoxsizer/kan/internal/domain"
	"github.com/epoxsizer/kan/internal/tasks"
)

// flowLoadedMsg carries every card on the board, archived included, for the
// flow summary.
type flowLoadedMsg struct {
	cards []domain.Card
	err   error
}

type agingEntry struct {
	title    string
	days     int
	columnID string
}

type flowSummary struct {
	throughput     []int
	archivedTotal  int
	avgArchiveDays float64
	aging          []agingEntry
}

// summarizeFlow computes weekly throughput for the last eight weeks, the
// average days from creation to archive, and the oldest active cards.
func summarizeFlow(cards []domain.Card, now time.Time) flowSummary {
	summary := flowSummary{throughput: make([]int, 8)}
	archiveDays := []float64{}
	for _, card := range cards {
		if card.DeletedAt != nil {
			summary.archivedTotal++
			weeksAgo := int(now.Sub(*card.DeletedAt).Hours() / 24 / 7)
			if weeksAgo >= 0 && weeksAgo < 8 {
				summary.throughput[7-weeksAgo]++
			}
			if !card.CreatedAt.IsZero() {
				archiveDays = append(archiveDays, card.DeletedAt.Sub(card.CreatedAt).Hours()/24)
			}
			continue
		}
		if !card.ColumnEnteredAt.IsZero() {
			days := int(now.Sub(card.ColumnEnteredAt).Hours() / 24)
			summary.aging = append(summary.aging, agingEntry{title: card.Title, days: days, columnID: card.ColumnID})
		}
	}
	if len(archiveDays) > 0 {
		total := 0.0
		for _, days := range archiveDays {
			total += days
		}
		summary.avgArchiveDays = total / float64(len(archiveDays))
	}
	sort.SliceStable(summary.aging, func(left, right int) bool {
		return summary.aging[left].days > summary.aging[right].days
	})
	if len(summary.aging) > 5 {
		summary.aging = summary.aging[:5]
	}
	return summary
}

func flowLines(summary flowSummary, columnNames map[string]string) []string {
	lines := []string{
		"Throughput (8 weeks): " + sparkline(summary.throughput),
		fmt.Sprintf("Archived total: %d", summary.archivedTotal),
		fmt.Sprintf("Average days to archive: %.1f", summary.avgArchiveDays),
		"",
		"Aging WIP:",
	}
	if len(summary.aging) == 0 {
		return append(lines, "  No active cards")
	}
	for _, entry := range summary.aging {
		lines = append(lines, fmt.Sprintf("  %dd  %s (%s)", entry.days, entry.title, columnNames[entry.columnID]))
	}
	return lines
}

func sparkline(values []int) string {
	blocks := []rune("▁▂▃▄▅▆▇█")
	maximum := 0
	for _, value := range values {
		maximum = max(maximum, value)
	}
	result := make([]rune, 0, len(values))
	for _, value := range values {
		level := 0
		if maximum > 0 {
			level = value * (len(blocks) - 1) / maximum
		}
		result = append(result, blocks[level])
	}
	return string(result)
}

func loadFlowCards(ctx context.Context, repo *tasks.Coordinator, boardID string) tea.Cmd {
	return func() tea.Msg {
		cards, err := repo.ListCardsIncludingDeleted(ctx, boardID)
		return flowLoadedMsg{cards: cards, err: err}
	}
}
