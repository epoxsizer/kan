package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gitlab.digital-spirit.ru/solutions/common/kan/internal/domain"
)

type screen uint8

const (
	projectsScreen screen = iota
	boardsScreen
	boardScreen
)

type Model struct {
	ctx    context.Context
	repo   domain.Repository
	logger *slog.Logger
	styles styles

	screen        screen
	width         int
	height        int
	loading       bool
	err           error
	help          bool
	notice        string
	detail        *detailPopup
	form          *formModal
	confirm       *confirmModal
	showCardTags  bool
	filterMode    bool
	filterText    string
	filterLoading bool
	filterErr     error
	filteredCards map[string][]domain.Card
	sortMode      cardSort
	groupMode     cardGroup

	commandMode    bool
	command        string
	commandIndex   int
	paletteItems   []paletteItem
	paletteLoading bool
	paletteErr     error
	pendingColumn  string
	pendingCard    string

	projects      []domain.Project
	projectCounts map[string]int
	projectIndex  int
	project       *domain.Project
	boards        []domain.Board
	boardCounts   map[string]int
	boardIndex    int
	board         *domain.Board
	columns       []domain.Column
	cards         map[string][]domain.Card
	columnIndex   int
	cardIndexes   map[string]int
}

func New(ctx context.Context, repo domain.Repository, logger *slog.Logger) *Model {
	return NewWithOptions(ctx, repo, logger, Options{ShowCardTags: true})
}

type Options struct {
	ShowCardTags bool
	Theme        Theme
}

func NewWithOptions(ctx context.Context, repo domain.Repository, logger *slog.Logger, options Options) *Model {
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Model{
		ctx:           ctx,
		repo:          repo,
		logger:        logger,
		styles:        stylesForTheme(themeOrDefault(options.Theme)),
		screen:        projectsScreen,
		loading:       true,
		cards:         make(map[string][]domain.Card),
		cardIndexes:   make(map[string]int),
		projectCounts: make(map[string]int),
		boardCounts:   make(map[string]int),
		showCardTags:  options.ShowCardTags,
	}
}

func themeOrDefault(theme Theme) Theme {
	if theme.Primary == "" {
		return DefaultTheme()
	}
	return theme
}

func (model *Model) Init() tea.Cmd {
	return loadProjects(model.ctx, model.repo)
}

func (model *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		model.width = message.Width
		model.height = message.Height
		return model, nil
	case projectsLoadedMsg:
		model.loading = false
		model.err = message.err
		if message.err == nil {
			model.projects = message.projects
			model.projectCounts = message.counts
			model.projectIndex = clampIndex(model.projectIndex, len(model.projects))
		}
		return model, nil
	case boardsLoadedMsg:
		model.loading = false
		model.err = message.err
		if message.err == nil {
			model.boards = message.boards
			model.boardCounts = message.counts
			model.boardIndex = 0
		}
		return model, nil
	case boardLoadedMsg:
		model.loading = false
		model.err = message.err
		if message.err == nil {
			model.columns = message.columns
			model.cards = make(map[string][]domain.Card, len(model.columns))
			model.cardIndexes = make(map[string]int, len(model.columns))
			for _, card := range message.cards {
				model.cards[card.ColumnID] = append(model.cards[card.ColumnID], card)
			}
			model.columnIndex = clampIndex(model.columnIndex, len(model.columns))
			model.applyPendingPaletteFocus()
		} else {
			model.pendingColumn = ""
			model.pendingCard = ""
		}
		if message.err == nil && model.filterActive() {
			model.filterLoading = true
			return model, searchBoardCards(model.ctx, model.repo, model.board.ID, model.filterText)
		}
		return model, nil
	case paletteLoadedMsg:
		model.paletteLoading = false
		model.paletteErr = message.err
		if message.err == nil {
			model.paletteItems = message.items
			model.commandIndex = clampIndex(model.commandIndex, len(model.paletteMatches()))
		}
		return model, nil
	case linkCandidatesLoadedMsg:
		if model.form != nil {
			model.form.linksLoading = false
			model.form.linkCandidates = message.candidates
			if message.err != nil {
				model.form.err = "load related cards: " + message.err.Error()
			}
		}
		return model, nil
	case boardFilterMsg:
		if message.query != model.filterText {
			return model, nil
		}
		model.filterLoading = false
		model.filterErr = message.err
		if message.err == nil {
			model.filteredCards = make(map[string][]domain.Card, len(model.columns))
			for _, card := range message.cards {
				model.filteredCards[card.ColumnID] = append(model.filteredCards[card.ColumnID], card)
			}
		}
		return model, nil
	case mutationDoneMsg:
		model.loading = false
		if message.err != nil {
			model.err = message.err
			return model, nil
		}
		model.notice = message.notice
		model.err = nil
		model.loading = true
		switch message.scope {
		case projectsScreen:
			model.screen = projectsScreen
			return model, loadProjects(model.ctx, model.repo)
		case boardsScreen:
			model.screen = boardsScreen
			return model, loadBoards(model.ctx, model.repo, model.project.ID)
		case boardScreen:
			model.screen = boardScreen
			return model, loadBoard(model.ctx, model.repo, model.board.ID)
		}
		return model, nil
	case tea.KeyMsg:
		return model.handleKey(message)
	}
	return model, nil
}

func (model *Model) handleKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.String() == "ctrl+c" {
		return model, tea.Quit
	}
	if model.form != nil {
		return model.handleFormKey(key)
	}
	if model.confirm != nil {
		return model.handleConfirmKey(key)
	}
	if model.filterMode {
		return model.handleFilterKey(key)
	}
	if model.commandMode {
		return model.handleCommandKey(key)
	}
	if model.detail != nil {
		switch key.String() {
		case "q":
			return model, tea.Quit
		case "esc", "d", "enter":
			model.detail = nil
		}
		return model, nil
	}
	switch key.String() {
	case "q":
		return model, tea.Quit
	case "?":
		model.help = !model.help
		return model, nil
	case ":":
		model.help = false
		model.commandMode = true
		model.command = ""
		model.commandIndex = 0
		model.paletteLoading = true
		model.paletteErr = nil
		model.paletteItems = nil
		return model, loadPaletteIndex(model.ctx, model.repo)
	case "esc":
		if model.help {
			model.help = false
			return model, nil
		}
		return model.goBack()
	}
	if model.help || model.loading {
		return model, nil
	}
	if key.String() == "/" && model.screen == boardScreen {
		model.filterMode = true
		model.notice = ""
		return model, nil
	}
	if key.String() == "d" {
		model.openSelectedDetail()
		return model, nil
	}
	if key.String() == "m" {
		return model, model.startStandaloneCommentEdit()
	}
	model.notice = ""

	switch model.screen {
	case projectsScreen:
		return model.handleProjectsKey(key)
	case boardsScreen:
		return model.handleBoardsKey(key)
	case boardScreen:
		return model.handleBoardKey(key)
	default:
		return model, nil
	}
}

func (model *Model) handleProjectsKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "a":
		model.startProjectForm(false)
	case "e":
		if len(model.projects) > 0 {
			model.startProjectForm(true)
		}
	case "D":
		if len(model.projects) > 0 {
			project := model.projects[model.projectIndex]
			model.confirm = &confirmModal{kind: deleteProject, title: "Delete project?", message: "Delete " + project.Name + " and all nested data?", id: project.ID}
		}
	case "j", "down":
		model.projectIndex = min(model.projectIndex+1, len(model.projects)-1)
	case "k", "up":
		model.projectIndex = max(model.projectIndex-1, 0)
	case "g", "home":
		model.projectIndex = 0
	case "G", "end":
		model.projectIndex = max(len(model.projects)-1, 0)
	case "enter", "l", "right":
		if len(model.projects) == 0 {
			return model, nil
		}
		selected := model.projects[model.projectIndex]
		model.project = &selected
		model.screen = boardsScreen
		model.loading = true
		model.err = nil
		return model, loadBoards(model.ctx, model.repo, selected.ID)
	}
	return model, nil
}

func (model *Model) handleBoardsKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "a":
		model.startBoardForm(false)
	case "e":
		if len(model.boards) > 0 {
			model.startBoardForm(true)
		}
	case "D":
		if len(model.boards) > 0 {
			board := model.boards[model.boardIndex]
			model.confirm = &confirmModal{kind: deleteBoard, title: "Delete board?", message: "Delete " + board.Name + " and all columns/cards?", id: board.ID}
		}
	case "j", "down":
		model.boardIndex = min(model.boardIndex+1, len(model.boards)-1)
	case "k", "up":
		model.boardIndex = max(model.boardIndex-1, 0)
	case "g", "home":
		model.boardIndex = 0
	case "G", "end":
		model.boardIndex = max(len(model.boards)-1, 0)
	case "enter", "l", "right":
		if len(model.boards) == 0 {
			return model, nil
		}
		selected := model.boards[model.boardIndex]
		model.board = &selected
		model.clearBoardFilter()
		model.screen = boardScreen
		model.loading = true
		model.err = nil
		return model, loadBoard(model.ctx, model.repo, selected.ID)
	case "h", "left":
		return model.goBack()
	}
	return model, nil
}

func (model *Model) handleBoardKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "s":
		model.cycleSort()
		return model, nil
	case "v":
		model.cycleGroup()
		return model, nil
	case "c":
		model.startColumnForm(false)
		return model, nil
	case "E":
		if len(model.columns) > 0 {
			model.startColumnForm(true)
		}
		return model, nil
	case "X":
		if len(model.columns) > 0 {
			column := model.columns[model.columnIndex]
			model.confirm = &confirmModal{kind: deleteColumn, title: "Delete column?", message: fmt.Sprintf("Delete %s and its %d cards?", column.Name, len(model.cards[column.ID])), id: column.ID}
		}
		return model, nil
	}
	if len(model.columns) == 0 {
		return model, nil
	}
	column := model.columns[model.columnIndex]
	cards := model.visibleCards(column.ID)
	index := model.cardIndexes[column.ID]
	switch key.String() {
	case "a":
		return model, model.startCardForm(false)
	case "e":
		if len(cards) > 0 {
			return model, model.startCardForm(true)
		}
	case "enter":
		model.openSelectedDetail()
	case "D":
		if len(cards) > 0 {
			card := cards[clampIndex(index, len(cards))]
			model.confirm = &confirmModal{kind: deleteCard, title: "Delete card?", message: "Soft-delete " + card.Title + "?", id: card.ID}
		}
	case "H", "shift+tab":
		return model.moveSelectedCard(-1)
	case "L", "tab":
		return model.moveSelectedCard(1)
	case "J":
		return model.reorderSelectedCard(1)
	case "K":
		return model.reorderSelectedCard(-1)
	case "h", "left":
		model.columnIndex = max(model.columnIndex-1, 0)
	case "l", "right":
		model.columnIndex = min(model.columnIndex+1, len(model.columns)-1)
	case "j", "down":
		model.cardIndexes[column.ID] = min(index+1, len(cards)-1)
	case "k", "up":
		model.cardIndexes[column.ID] = max(index-1, 0)
	case "g", "home":
		model.cardIndexes[column.ID] = 0
	case "G", "end":
		model.cardIndexes[column.ID] = max(len(cards)-1, 0)
	}
	return model, nil
}

func (model *Model) handleCommandKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyEsc:
		model.commandMode = false
		model.command = ""
		model.commandIndex = 0
	case tea.KeyEnter:
		matches := model.paletteMatches()
		model.commandMode = false
		model.command = ""
		if len(matches) == 0 {
			model.err = fmt.Errorf("no matching command or data")
			return model, nil
		}
		selected := matches[clampIndex(model.commandIndex, len(matches))]
		model.commandIndex = 0
		return model.executePaletteMatch(selected)
	case tea.KeyUp:
		model.commandIndex = max(model.commandIndex-1, 0)
	case tea.KeyDown, tea.KeyTab:
		model.commandIndex = min(model.commandIndex+1, max(len(model.paletteMatches())-1, 0))
	case tea.KeyBackspace, tea.KeyDelete:
		runes := []rune(model.command)
		if len(runes) > 0 {
			model.command = string(runes[:len(runes)-1])
		}
		model.commandIndex = 0
	case tea.KeyRunes:
		model.command += string(key.Runes)
		model.commandIndex = 0
	}
	return model, nil
}

func (model *Model) executePaletteMatch(match paletteMatch) (tea.Model, tea.Cmd) {
	if match.command != "" {
		return model.executeCommand(match.command)
	}
	if match.item == nil {
		return model, nil
	}
	item := *match.item
	model.help = false
	model.err = nil
	model.notice = ""
	model.clearBoardFilter()
	model.project = &item.project
	switch item.kind {
	case projectItem:
		model.board = nil
		model.screen = boardsScreen
		model.loading = true
		return model, loadBoards(model.ctx, model.repo, item.project.ID)
	case boardItem:
		model.board = &item.board
		model.pendingColumn = ""
		model.pendingCard = ""
	case columnItem:
		model.board = &item.board
		model.pendingColumn = item.column.ID
		model.pendingCard = ""
	case cardItem:
		model.board = &item.board
		model.pendingColumn = item.card.ColumnID
		model.pendingCard = item.card.ID
	}
	model.screen = boardScreen
	model.loading = true
	return model, loadBoard(model.ctx, model.repo, item.board.ID)
}

func (model *Model) applyPendingPaletteFocus() {
	if model.pendingColumn == "" && model.pendingCard == "" {
		return
	}
	for columnIndex, column := range model.columns {
		if column.ID != model.pendingColumn {
			continue
		}
		model.columnIndex = columnIndex
		if model.pendingCard != "" {
			for cardIndex, card := range model.visibleCards(column.ID) {
				if card.ID == model.pendingCard {
					model.cardIndexes[column.ID] = cardIndex
					break
				}
			}
		}
		break
	}
	model.pendingColumn = ""
	model.pendingCard = ""
}

func (model *Model) executeCommand(command string) (tea.Model, tea.Cmd) {
	switch command {
	case "q", "quit":
		return model, tea.Quit
	case "new":
		switch model.screen {
		case projectsScreen:
			model.startProjectForm(false)
		case boardsScreen:
			model.startBoardForm(false)
		case boardScreen:
			if len(model.columns) == 0 {
				model.err = fmt.Errorf("create a column first")
			} else {
				return model, model.startCardForm(false)
			}
		}
	case "edit":
		switch model.screen {
		case projectsScreen:
			if len(model.projects) > 0 {
				model.startProjectForm(true)
			}
		case boardsScreen:
			if len(model.boards) > 0 {
				model.startBoardForm(true)
			}
		case boardScreen:
			if len(model.columns) > 0 && model.selectedCard().ID != "" {
				return model, model.startCardForm(true)
			}
		}
	case "delete":
		switch model.screen {
		case projectsScreen:
			return model.handleProjectsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
		case boardsScreen:
			return model.handleBoardsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
		case boardScreen:
			return model.handleBoardKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
		}
	case "new-column":
		if model.screen != boardScreen || model.board == nil {
			model.err = fmt.Errorf("open a board first")
			return model, nil
		}
		model.startColumnForm(false)
	case "sort":
		if model.screen != boardScreen {
			model.err = fmt.Errorf("open a board first")
			return model, nil
		}
		model.cycleSort()
	case "group":
		if model.screen != boardScreen {
			model.err = fmt.Errorf("open a board first")
			return model, nil
		}
		model.cycleGroup()
	case "help", "?":
		model.help = true
	case "project", "projects":
		model.screen = projectsScreen
		model.help = false
	case "board", "boards":
		if model.project == nil {
			model.err = fmt.Errorf("open a project first")
			return model, nil
		}
		model.screen = boardsScreen
		model.help = false
	case "reload":
		model.loading = true
		model.err = nil
		switch model.screen {
		case projectsScreen:
			return model, loadProjects(model.ctx, model.repo)
		case boardsScreen:
			return model, loadBoards(model.ctx, model.repo, model.project.ID)
		case boardScreen:
			return model, loadBoard(model.ctx, model.repo, model.board.ID)
		}
	case "":
	default:
		model.err = fmt.Errorf("unknown command: %s", command)
	}
	return model, nil
}

func (model *Model) goBack() (tea.Model, tea.Cmd) {
	switch model.screen {
	case boardScreen:
		model.clearBoardFilter()
		model.screen = boardsScreen
	case boardsScreen:
		model.screen = projectsScreen
	}
	model.err = nil
	return model, nil
}

func (model *Model) View() string {
	width, height := model.dimensions()
	if model.form != nil {
		return model.renderForm(width, height)
	}
	if model.confirm != nil {
		return model.renderConfirm(width, height)
	}
	if model.commandMode {
		return model.renderCommandPalette(width, height)
	}
	if model.detail != nil {
		return model.renderDetailPopup(width, height)
	}
	if model.help {
		boxWidth := min(64, max(width-4, 20))
		help := model.styles.help.Width(max(boxWidth-6, 10)).Render(model.renderHelpText())
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, help)
	}

	header := model.renderHeader(width)
	contentHeight := max(height-3, 1)
	var content string
	if model.loading {
		content = model.styles.subtle.Render("Loading...")
	} else if model.err != nil {
		content = model.styles.error.Render("Error: " + model.err.Error())
	} else {
		switch model.screen {
		case projectsScreen:
			content = model.renderProjects(width)
		case boardsScreen:
			content = model.renderBoards(width)
		case boardScreen:
			content = model.renderBoard(width, contentHeight)
		}
	}
	content = model.styles.body.Copy().Width(width).Height(contentHeight).MaxHeight(contentHeight).Render(content)
	return header + "\n" + content + "\n" + model.renderStatus(width)
}

func (model *Model) dimensions() (int, int) {
	width, height := model.width, model.height
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	return width, height
}

func (model *Model) renderHeader(width int) string {
	title := model.styles.header.Render("kan")
	subtitle := model.styles.subtle.Render("  local kanban")
	return lipgloss.NewStyle().Width(width).Render(title + subtitle)
}

func (model *Model) renderProjects(width int) string {
	if len(model.projects) == 0 {
		return model.styles.subtle.Render("No projects. Press a to create one.")
	}
	rows := make([]tableRow, 0, len(model.projects))
	for index, project := range model.projects {
		rows = append(rows, tableRow{name: project.Name, comments: project.Description, items: model.projectCounts[project.ID], selected: index == model.projectIndex})
	}
	return model.renderTable("Projects", "BOARDS", rows, width)
}

func (model *Model) renderBoards(width int) string {
	if len(model.boards) == 0 {
		return model.styles.subtle.Render("No boards in this project. Press a to create one.")
	}
	rows := make([]tableRow, 0, len(model.boards))
	for index, board := range model.boards {
		rows = append(rows, tableRow{name: board.Name, comments: board.Description, items: model.boardCounts[board.ID], selected: index == model.boardIndex})
	}
	return model.renderTable("Boards", "CARDS", rows, width)
}

func (model *Model) renderBoard(width, contentHeight int) string {
	if len(model.columns) == 0 {
		return model.styles.subtle.Render("This board has no columns. Press c to create one.")
	}
	const minimumColumnWidth = 22
	visible := min(len(model.columns), max(1, width/minimumColumnWidth))
	start := max(0, model.columnIndex-visible+1)
	start = min(start, len(model.columns)-visible)
	panels := make([]string, 0, visible)
	baseWidth := width / visible
	remainder := width % visible
	for offset, index := 0, start; index < start+visible; offset, index = offset+1, index+1 {
		columnWidth := baseWidth
		if offset < remainder {
			columnWidth++
		}
		panels = append(panels, model.renderColumn(index, columnWidth, contentHeight))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, panels...)
}

func (model *Model) renderColumn(index, width, height int) string {
	column := model.columns[index]
	cards := model.visibleCards(column.ID)
	count := fmt.Sprintf("%d", len(cards))
	if model.filterActive() {
		count = fmt.Sprintf("%d/%d", len(cards), len(model.cards[column.ID]))
	}
	title := fmt.Sprintf("%s  %s", column.Name, count)
	if column.WIPLimit != nil {
		title += fmt.Sprintf("/%d", *column.WIPLimit)
	}
	innerWidth := max(width-4, 10)
	columnColor := namedColors["blue"]
	if column.Color != nil {
		columnColor = colorForName(*column.Color)
	}
	headerStyle := model.styles.header.Copy().Foreground(columnColor)
	if index == model.columnIndex {
		title = "ACTIVE • " + title
		headerStyle = headerStyle.Foreground(lipgloss.Color("#000000")).Background(columnColor).Padding(0, 1)
	}
	lines := []string{headerStyle.Render(truncate(title, innerWidth))}
	if len(cards) == 0 {
		empty := "No cards"
		if model.filterActive() {
			empty = "No matches"
		}
		lines = append(lines, model.styles.subtle.Render(empty))
	} else {
		selected := clampIndex(model.cardIndexes[column.ID], len(cards))
		maxRows := max(height-5, 1)
		cardContentWidth := max(innerWidth-2, 1)
		rows, selectedRow := model.cardDisplayRows(cards, selected)
		start := max(0, selectedRow-maxRows+1)
		for _, row := range rows[start:min(start+maxRows, len(rows))] {
			if row.group != "" {
				lines = append(lines, model.styles.subtle.Copy().Bold(true).Render("─ "+truncate(row.group, max(cardContentWidth-2, 1))))
				continue
			}
			label := model.cardLabel(row.card, max(cardContentWidth-4, 1), true)
			if index == model.columnIndex && row.cardIndex == selected {
				label = model.cardLabel(row.card, max(cardContentWidth-4, 1), false)
				selectedStyle := model.styles.selectedCard.Copy().Foreground(lipgloss.Color("#000000")).Background(columnColor)
				lines = append(lines, selectedStyle.Width(cardContentWidth).Render("> "+label))
			} else {
				lines = append(lines, model.styles.card.Width(cardContentWidth).Render("  "+label))
			}
		}
	}
	style := model.styles.panel
	if index == model.columnIndex {
		style = model.styles.focusedPanel.Copy().Border(lipgloss.DoubleBorder())
	}
	style = style.BorderForeground(columnColor)
	return style.Width(innerWidth).Height(max(height-2, 1)).Render(strings.Join(lines, "\n"))
}

type cardDisplayRow struct {
	card      domain.Card
	cardIndex int
	group     string
}

func (model *Model) cardDisplayRows(cards []domain.Card, selected int) ([]cardDisplayRow, int) {
	rows := []cardDisplayRow{}
	selectedRow := 0
	lastGroup := ""
	for index, card := range cards {
		if model.groupMode != groupNone {
			group := model.cardGroupLabel(card)
			if group != lastGroup {
				rows = append(rows, cardDisplayRow{group: group})
				lastGroup = group
			}
		}
		if index == selected {
			selectedRow = len(rows)
		}
		rows = append(rows, cardDisplayRow{card: card, cardIndex: index})
	}
	return rows, selectedRow
}

func (model *Model) cardLabel(card domain.Card, width int, colorPriority bool) string {
	prefix := ""
	if model.showCardTags {
		for _, tag := range card.Tags {
			prefix += "[" + tag + "]"
		}
		if prefix != "" {
			prefix += " "
		}
	}
	suffix := ""
	if len(card.Checklist) > 0 {
		done := 0
		for _, item := range card.Checklist {
			if item.Done {
				done++
			}
		}
		suffix = fmt.Sprintf(" [%d/%d]", done, len(card.Checklist))
	}
	duePrefix := ""
	if card.DueDate != nil {
		duePrefix = "@" + card.DueDate.Format("2006-01-02") + " "
	}
	content := duePrefix + prefix + card.Title + suffix
	label := truncate(content, width)
	if card.Priority == nil || *card.Priority == "" || width < 3 {
		return label
	}
	priority := strings.ToUpper((*card.Priority)[:1])
	if !colorPriority {
		return priority + " " + truncate(content, max(width-2, 1))
	}
	marker := lipgloss.NewStyle().Bold(true).Foreground(priorityColor(*card.Priority)).Render(priority)
	return marker + " " + truncate(content, max(width-2, 1))
}

func (model *Model) renderStatus(width int) string {
	if model.filterMode {
		hint := "  live FTS · Enter/Esc close · Ctrl-U clear"
		if model.filterLoading {
			hint = "  searching…"
		} else if model.filterErr != nil {
			hint = "  error: " + model.filterErr.Error()
		}
		info := lipgloss.NewStyle().Width(width).Render(model.styles.command.Render("/"+model.filterText+"█") + model.styles.subtle.Render(hint))
		return info + "\n" + model.renderShortcutBar(width)
	}
	if model.commandMode {
		line := model.styles.command.Render(":" + model.command + "█")
		return lipgloss.NewStyle().Width(width).Render(line)
	}
	breadcrumb := "Projects"
	count := fmt.Sprintf("%d projects", len(model.projects))
	if model.screen >= boardsScreen && model.project != nil {
		breadcrumb += " › " + model.project.Name
		count = fmt.Sprintf("%d boards", len(model.boards))
	}
	if model.screen == boardScreen && model.board != nil {
		breadcrumb += " › " + model.board.Name
		count = fmt.Sprintf("%d columns · %d cards", len(model.columns), model.cardCount())
		if model.filterActive() {
			count = fmt.Sprintf("%d matches · /%s", model.visibleCardCount(), model.filterText)
			if model.filterErr != nil {
				count = "filter error: " + model.filterErr.Error()
			}
		}
		count += " · sort:" + model.sortMode.String() + " · group:" + model.groupMode.String()
	}
	left := model.styles.statusAccent.Render(breadcrumb)
	if lipgloss.Width(left) > width/2 {
		breadcrumb = truncate(breadcrumb, max(width/2-2, 1))
		left = model.styles.statusAccent.Render(breadcrumb)
	}
	if model.notice != "" {
		count = model.notice
	}
	right := model.styles.status.Render(truncate(count, max(width-lipgloss.Width(left)-2, 1)))
	gap := max(width-lipgloss.Width(left)-lipgloss.Width(right), 0)
	info := left + strings.Repeat(" ", gap) + right
	return info + "\n" + model.renderShortcutBar(width)
}

type shortcut struct {
	key   string
	label string
}

func (model *Model) renderShortcutBar(width int) string {
	shortcuts := []shortcut{{"?", "Help"}, {":", "Cmd"}, {"q", "Quit"}}
	switch model.screen {
	case projectsScreen:
		shortcuts = append(shortcuts, shortcut{"j/k", "Navigate"}, shortcut{"Enter", "Open"}, shortcut{"a", "Add"}, shortcut{"e", "Edit"}, shortcut{"D", "Delete"}, shortcut{"d", "Describe"}, shortcut{"m", "Comment"})
	case boardsScreen:
		shortcuts = append(shortcuts, shortcut{"j/k", "Navigate"}, shortcut{"Enter", "Open"}, shortcut{"a", "Add"}, shortcut{"e", "Edit"}, shortcut{"D", "Delete"}, shortcut{"d", "Describe"})
	case boardScreen:
		shortcuts = append(shortcuts, shortcut{"j/k", "Card"}, shortcut{"h/l", "Column"}, shortcut{"a", "Add"}, shortcut{"e", "Edit"}, shortcut{"D", "Delete"}, shortcut{"/", "Filter"}, shortcut{"s", "Sort"}, shortcut{"v", "Group"}, shortcut{"Tab", "Move"})
	}
	if model.filterMode {
		shortcuts = []shortcut{{"Enter", "Keep"}, {"Esc", "Close"}, {"Ctrl-U", "Clear"}, {"Ctrl-C", "Quit"}}
	}
	line := ""
	for _, item := range shortcuts {
		token := model.styles.shortcutKey.Render("<"+item.key+">") + model.styles.shortcutText.Render(item.label) + " "
		if lipgloss.Width(line)+lipgloss.Width(token) > width {
			break
		}
		line += token
	}
	return model.styles.shortcutText.Copy().Width(width).MaxWidth(width).Render(line)
}

func (model *Model) cardCount() int {
	count := 0
	for _, cards := range model.cards {
		count += len(cards)
	}
	return count
}

func clampIndex(index, length int) int {
	if length <= 0 {
		return 0
	}
	return min(max(index, 0), length-1)
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}
