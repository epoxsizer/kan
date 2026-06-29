package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/epoxsizer/kan/internal/domain"
)

type cardSort uint8

const (
	sortPosition cardSort = iota
	sortPriority
	sortDue
	sortTitle
)

type cardGroup uint8

const (
	groupNone cardGroup = iota
	groupPriority
	groupDue
	groupTag
)

func searchBoardCards(ctx context.Context, repo domain.Repository, boardID, query string) tea.Cmd {
	return func() tea.Msg {
		ftsQuery := buildFTSQuery(query)
		if ftsQuery == "" {
			return boardFilterMsg{query: query}
		}
		cards, err := repo.SearchCards(ctx, boardID, ftsQuery)
		return boardFilterMsg{query: query, cards: cards, err: err}
	}
}

func buildFTSQuery(query string) string {
	terms := strings.Fields(query)
	quoted := make([]string, 0, len(terms))
	for _, term := range terms {
		term = strings.ReplaceAll(term, `"`, `""`)
		if term != "" {
			quoted = append(quoted, `"`+term+`"*`)
		}
	}
	return strings.Join(quoted, " AND ")
}

func (model *Model) handleFilterKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "ctrl+c":
		return model, tea.Quit
	case "enter", "esc":
		model.filterMode = false
		return model, nil
	}
	result := editText(model.filterText, model.filterCursor, key, false)
	if !result.handled {
		return model, nil
	}
	model.filterText, model.filterCursor = result.value, result.cursor
	if !result.changed {
		return model, nil
	}
	model.cardIndexes = make(map[string]int, len(model.columns))
	if strings.TrimSpace(model.filterText) == "" {
		model.filteredCards = nil
		model.filterLoading = false
		model.filterErr = nil
		return model, nil
	}
	model.filterLoading = true
	model.filterErr = nil
	return model, searchBoardCards(model.ctx, model.repo, model.board.ID, model.filterText)
}

func (model *Model) clearBoardFilter() {
	model.filterMode = false
	model.filterText = ""
	model.filterCursor = 0
	model.filteredCards = nil
	model.filterLoading = false
	model.filterErr = nil
}

func (model *Model) filterActive() bool {
	return strings.TrimSpace(model.filterText) != ""
}

func (model *Model) visibleCards(columnID string) []domain.Card {
	values := model.cards[columnID]
	if model.filterActive() {
		values = model.filteredCards[columnID]
	}
	result := append([]domain.Card(nil), values...)
	sort.SliceStable(result, func(left, right int) bool {
		if model.groupMode != groupNone {
			leftGroup, rightGroup := model.cardGroupKey(result[left]), model.cardGroupKey(result[right])
			if leftGroup != rightGroup {
				return leftGroup < rightGroup
			}
		}
		comparison := model.compareCards(result[left], result[right])
		if comparison == 0 {
			return result[left].Position < result[right].Position
		}
		return comparison < 0
	})
	return result
}

func (model *Model) compareCards(left, right domain.Card) int {
	switch model.sortMode {
	case sortPriority:
		return compare(priorityRank(left.Priority), priorityRank(right.Priority))
	case sortDue:
		return compareTime(left.DueDate, right.DueDate)
	case sortTitle:
		return strings.Compare(strings.ToLower(left.Title), strings.ToLower(right.Title))
	default:
		return compare(left.Position, right.Position)
	}
}

func compare[T ~int | ~float64](left, right T) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

func priorityRank(priority *string) int {
	if priority == nil {
		return 4
	}
	switch strings.ToLower(*priority) {
	case "urgent":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	default:
		return 4
	}
}

func compareTime(left, right *time.Time) int {
	if left == nil && right == nil {
		return 0
	}
	if left == nil {
		return 1
	}
	if right == nil {
		return -1
	}
	if left.Before(*right) {
		return -1
	}
	if left.After(*right) {
		return 1
	}
	return 0
}

func (model *Model) cycleSort() {
	model.sortMode = (model.sortMode + 1) % 4
	model.cardIndexes = make(map[string]int, len(model.columns))
	model.notice = "Sort: " + model.sortMode.String()
}

func (model *Model) cycleGroup() {
	model.groupMode = (model.groupMode + 1) % 4
	model.cardIndexes = make(map[string]int, len(model.columns))
	model.notice = "Group: " + model.groupMode.String()
}

func (value cardSort) String() string {
	return []string{"position", "priority", "due date", "title"}[value]
}

func (value cardGroup) String() string {
	return []string{"none", "priority", "due date", "first tag"}[value]
}

func (model *Model) cardGroupKey(card domain.Card) string {
	switch model.groupMode {
	case groupPriority:
		return fmt.Sprintf("%d:%s", priorityRank(card.Priority), groupLabelPriority(card.Priority))
	case groupDue:
		return dueGroup(card.DueDate)
	case groupTag:
		if len(card.Tags) == 0 {
			return "z:No tag"
		}
		return "a:" + strings.ToLower(card.Tags[0])
	default:
		return ""
	}
}

func (model *Model) cardGroupLabel(card domain.Card) string {
	if model.groupMode == groupTag {
		if len(card.Tags) == 0 {
			return "No tag"
		}
		return card.Tags[0]
	}
	key := model.cardGroupKey(card)
	if separator := strings.IndexByte(key, ':'); separator >= 0 {
		return key[separator+1:]
	}
	return key
}

func groupLabelPriority(priority *string) string {
	if priority == nil || *priority == "" {
		return "No priority"
	}
	return strings.ToUpper(*priority)
}

func dueGroup(due *time.Time) string {
	if due == nil {
		return "z:No due date"
	}
	today := time.Now().In(time.Local)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local)
	date := due.In(time.Local)
	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.Local)
	if date.Before(today) {
		return "a:Overdue"
	}
	if date.Equal(today) {
		return "b:Today"
	}
	if date.Before(today.AddDate(0, 0, 8)) {
		return "c:Next 7 days"
	}
	return "d:Later"
}

func (model *Model) derivedBoardView() bool {
	return model.filterActive() || model.sortMode != sortPosition || model.groupMode != groupNone
}

func (model *Model) visibleCardCount() int {
	count := 0
	for _, column := range model.columns {
		count += len(model.visibleCards(column.ID))
	}
	return count
}
