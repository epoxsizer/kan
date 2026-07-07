package app

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/epoxsizer/kan/internal/domain"
)

type PlanningOptions struct {
	StaleAfterDays           int
	BlockedTags              []string
	UntriagedWithoutPriority bool
}

func defaultPlanningOptions() PlanningOptions {
	return PlanningOptions{StaleAfterDays: 7, BlockedTags: []string{"blocked", "blocker"}, UntriagedWithoutPriority: true}
}

func planningOrDefault(value PlanningOptions) PlanningOptions {
	defaults := defaultPlanningOptions()
	if value.StaleAfterDays <= 0 {
		value.StaleAfterDays = defaults.StaleAfterDays
	}
	if len(value.BlockedTags) == 0 {
		value.BlockedTags = defaults.BlockedTags
	}
	return value
}

type planningKind string

const (
	planningNone      planningKind = ""
	planningToday     planningKind = "today"
	planningOverdue   planningKind = "overdue"
	planningBlocked   planningKind = "blocked"
	planningStale     planningKind = "stale"
	planningUntriaged planningKind = "untriaged"
)

type planningHealth struct {
	Overdue     int
	Today       int
	Blocked     int
	Stale       int
	Untriaged   int
	WIPPressure []string
}

func (model *Model) applyPlanningFilter(kind planningKind) {
	if model.screen != boardScreen || model.board == nil {
		model.err = fmt.Errorf("open a board first")
		return
	}
	model.filterMode = false
	model.filterText = ""
	model.filterCursor = 0
	model.filterLoading = false
	model.filterErr = nil
	model.planningFilter = kind
	model.filteredCards = make(map[string][]domain.Card, len(model.columns))
	model.filterScores = make(map[string]int)
	now := time.Now()
	for _, column := range model.columns {
		for _, card := range model.cards[column.ID] {
			if model.cardMatchesPlanning(kind, card, now) {
				model.filteredCards[column.ID] = append(model.filteredCards[column.ID], card)
				model.filterScores[card.ID] = 1000
			}
		}
	}
	model.cardIndexes = make(map[string]int, len(model.columns))
	model.notice = "Planning: " + string(kind)
}

func (model *Model) cardMatchesPlanning(kind planningKind, card domain.Card, now time.Time) bool {
	switch kind {
	case planningToday:
		return card.DueDate != nil && calendarDayDelta(*card.DueDate, now) == 0
	case planningOverdue:
		return card.DueDate != nil && calendarDayDelta(*card.DueDate, now) < 0
	case planningBlocked:
		return model.cardBlocked(card)
	case planningStale:
		return card.UpdatedAt.Before(now.AddDate(0, 0, -model.planning.StaleAfterDays))
	case planningUntriaged:
		return model.cardUntriaged(card)
	default:
		return true
	}
}

func (model *Model) cardBlocked(card domain.Card) bool {
	tags := map[string]struct{}{}
	for _, tag := range model.planning.BlockedTags {
		tags[strings.ToLower(strings.TrimSpace(tag))] = struct{}{}
	}
	for _, tag := range card.Tags {
		if _, ok := tags[strings.ToLower(strings.TrimSpace(tag))]; ok {
			return true
		}
	}
	return false
}

func (model *Model) cardUntriaged(card domain.Card) bool {
	if !model.planning.UntriagedWithoutPriority {
		return card.DueDate == nil
	}
	return (card.Priority == nil || strings.TrimSpace(*card.Priority) == "") && card.DueDate == nil
}

func (model *Model) boardPlanningHealth() planningHealth {
	var health planningHealth
	now := time.Now()
	for _, column := range model.columns {
		cards := model.cards[column.ID]
		if column.WIPLimit != nil && *column.WIPLimit > 0 && len(cards) >= *column.WIPLimit {
			health.WIPPressure = append(health.WIPPressure, fmt.Sprintf("%s %d/%d", column.Name, len(cards), *column.WIPLimit))
		}
		for _, card := range cards {
			if card.DueDate != nil {
				delta := calendarDayDelta(*card.DueDate, now)
				if delta < 0 {
					health.Overdue++
				} else if delta == 0 {
					health.Today++
				}
			}
			if model.cardBlocked(card) {
				health.Blocked++
			}
			if model.cardMatchesPlanning(planningStale, card, now) {
				health.Stale++
			}
			if model.cardUntriaged(card) {
				health.Untriaged++
			}
		}
	}
	sort.Strings(health.WIPPressure)
	return health
}

func (health planningHealth) Summary() string {
	parts := []string{}
	if health.Overdue > 0 {
		parts = append(parts, fmt.Sprintf("overdue:%d", health.Overdue))
	}
	if health.Today > 0 {
		parts = append(parts, fmt.Sprintf("today:%d", health.Today))
	}
	if health.Blocked > 0 {
		parts = append(parts, fmt.Sprintf("blocked:%d", health.Blocked))
	}
	if health.Stale > 0 {
		parts = append(parts, fmt.Sprintf("stale:%d", health.Stale))
	}
	if health.Untriaged > 0 {
		parts = append(parts, fmt.Sprintf("untriaged:%d", health.Untriaged))
	}
	if len(health.WIPPressure) > 0 {
		parts = append(parts, "wip:"+strings.Join(health.WIPPressure, ","))
	}
	return strings.Join(parts, " · ")
}
