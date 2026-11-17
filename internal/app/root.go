package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/epoxsizer/kan/internal/domain"
	"github.com/epoxsizer/kan/internal/tasks"
)

type screen uint8

const (
	projectsScreen screen = iota
	boardsScreen
	boardScreen
)

type listLayout uint8

const (
	tableLayout listLayout = iota
	cardsLayout
)

func (layout listLayout) String() string {
	switch layout {
	case cardsLayout:
		return "cards"
	default:
		return "table"
	}
}

type Model struct {
	ctx          context.Context
	repo         *tasks.Coordinator
	logger       *slog.Logger
	styles       styles
	workspace    workspaceState
	legacyKeymap bool
	mcpEnabled   bool

	screen           screen
	width            int
	height           int
	loading          bool
	loadGeneration   uint64
	searchGeneration uint64
	err              error
	help             bool
	helpOffset       int
	notice           string
	detail           *detailPopup
	form             *formModal
	confirm          *confirmModal
	discard          *discardModal
	movePicker       *movePicker
	lastMove         *cardMoveRecord
	drag             *cardDrag
	lastCardClick    *cardClickRecord
	now              func() time.Time
	showCardTags     bool

	showSelectedCardDetails bool
	wrapCardTitles          bool
	mouseEnabled            bool
	configFile              string
	planning                PlanningOptions

	filterMode     bool
	filterText     string
	filterCursor   int
	filterLoading  bool
	filterErr      error
	filteredCards  map[string][]domain.Card
	filterScores   map[string]int
	planningFilter planningKind
	sortMode       cardSort
	groupMode      cardGroup
	listLayout     listLayout

	commandMode   bool
	command       string
	commandCursor int
	commandIndex  int
	commandMenu   string
	pendingColumn string
	pendingCard   string

	projects      []domain.Project
	projectCounts map[string]int
	projectIndex  int
	project       *domain.Project
	boards        []domain.Board
	boardCounts   map[string]int
	boardHealth   map[string]boardHealth
	boardIndex    int
	board         *domain.Board
	columns       []domain.Column
	cards         map[string][]domain.Card
	columnIndex   int
	cardIndexes   map[string]int
}

func New(ctx context.Context, repo domain.Repository, logger *slog.Logger) *Model {
	return NewWithOptions(ctx, repo, logger, Options{ShowCardTags: true, Mouse: true, WrapCardTitles: true})
}

type Options struct {
	ShowCardTags            bool
	ShowSelectedCardDetails bool
	StartupNotice           string
	Theme                   Theme
	Planning                PlanningOptions
	LegacyKeymap            bool
	MCPEnabled              bool
	Mouse                   bool
	WrapCardTitles          bool
	ConfigFile              string
}

func NewWithOptions(ctx context.Context, repo domain.Repository, logger *slog.Logger, options Options) *Model {
	coordinator, ok := repo.(*tasks.Coordinator)
	if !ok {
		coordinator = tasks.New(repo)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Model{
		ctx:           ctx,
		repo:          coordinator,
		logger:        logger,
		now:           time.Now,
		styles:        stylesForTheme(themeOrDefault(options.Theme)),
		planning:      planningOrDefault(options.Planning),
		workspace:     workspaceState{navigation: true, inspector: true},
		legacyKeymap:  options.LegacyKeymap,
		mcpEnabled:    options.MCPEnabled,
		screen:        projectsScreen,
		loading:       true,
		cards:         make(map[string][]domain.Card),
		cardIndexes:   make(map[string]int),
		projectCounts: make(map[string]int),
		boardCounts:   make(map[string]int),
		boardHealth:   make(map[string]boardHealth),
		showCardTags:  options.ShowCardTags,
		notice:        options.StartupNotice,

		showSelectedCardDetails: options.ShowSelectedCardDetails,
		wrapCardTitles:          options.WrapCardTitles,
		mouseEnabled:            options.Mouse,
		configFile:              options.ConfigFile,
	}
}

func themeOrDefault(theme Theme) Theme {
	if theme.Primary == "" {
		return DefaultTheme()
	}
	return theme
}

func (model *Model) Init() tea.Cmd {
	return model.loadProjects()
}

func (model *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	_, previousCard := model.selectedIDs()
	defer func() {
		_, currentCard := model.selectedIDs()
		if previousCard != currentCard {
			model.workspace.inspectorOffset = 0
		}
	}()
	switch message := message.(type) {
	case formLoadedMsg:
		if model.form != message.form {
			return model, nil
		}
		return model.Update(message.message)
	case searchLoadedMsg:
		if model.board == nil || model.screen != boardScreen || model.board.ID != message.boardID || model.searchGeneration != message.generation {
			return model, nil
		}
		return model.Update(message.message)
	case cardDraftReloadedMsg:
		if model.form != message.form {
			return model, nil
		}
		model.form.saving = false
		if message.err != nil {
			model.form.err = message.err.Error()
			return model, nil
		}
		populateCardDraft(model.form, message.card, message.column.Name)
		model.form.fields[2].options = withLegacyOption(columnNames(model.columns), message.column.Name)
		model.activateForm(model.form)
		return model, nil
	case workspaceLoadedMsg:
		if message.generation != model.loadGeneration || message.screen != model.screen {
			return model, nil
		}
		return model.Update(message.message)
	case flowLoadedMsg:
		if message.err != nil {
			model.err = message.err
			return model, nil
		}
		columnNames := make(map[string]string, len(model.columns))
		for _, column := range model.columns {
			columnNames[column.ID] = column.Name
		}
		title := "Flow"
		if model.board != nil {
			title = "Flow — " + model.board.Name
		}
		model.detail = &detailPopup{kind: "flow", title: title, lines: flowLines(summarizeFlow(message.cards, time.Now()), columnNames)}
		return model, nil
	case cardQueuedMsg:
		if message.err != nil {
			model.err = message.err
			return model, nil
		}
		cards := model.cards[message.card.ColumnID]
		for index := range cards {
			if cards[index].ID == message.card.ID {
				cards[index] = message.card
			}
		}
		model.notice = "Card queued for agents"
		return model, nil
	case settingsSavedMsg:
		if message.err != nil {
			model.err = message.err
		} else {
			model.notice = "Settings saved"
		}
		return model, nil
	case cardExportFinishedMsg:
		if message.err != nil {
			model.err = message.err
		} else {
			model.notice = "Exported to " + message.path
		}
		return model, nil
	case dragAutoScrollMsg:
		return model, model.autoScrollDrag()
	case NoticeMsg:
		model.notice = message.Text
		return model, nil
	case ExternalChangeMsg:
		model.recordActivity(message)
		model.drag = nil
		model.notice = "MCP: " + message.Action
		model.err = nil
		model.lastMove = nil
		if model.form != nil && model.form.kind == editCardForm && model.form.entityID == message.CardID {
			model.form.err = "Card changed externally. Draft kept; Ctrl-R reloads, Esc cancels."
		}
		switch model.screen {
		case projectsScreen:
			model.loading = true
			return model, model.loadProjects()
		case boardsScreen:
			if model.project != nil {
				model.loading = true
				return model, model.loadBoards(model.project.ID)
			}
		case boardScreen:
			if model.board != nil && model.board.ID == message.BoardID {
				return model, model.loadBoard(model.board.ID)
			}
		}
		return model, nil
	case tea.WindowSizeMsg:
		model.width = message.Width
		model.height = message.Height
		model.normalizePaneFocus()
		return model, nil
	case projectsLoadedMsg:
		model.loading = false
		model.err = message.err
		if message.err == nil {
			selectedID := ""
			if len(model.projects) > 0 {
				selectedID = model.projects[clampIndex(model.projectIndex, len(model.projects))].ID
			}
			model.projects = message.projects
			model.projectCounts = message.counts
			model.projectIndex = clampIndex(model.projectIndex, len(model.projects))
			for index, project := range model.projects {
				if project.ID == selectedID {
					model.projectIndex = index
				}
			}
		}
		return model, nil
	case boardsLoadedMsg:
		model.loading = false
		model.err = message.err
		if message.err == nil {
			selectedID := ""
			if len(model.boards) > 0 {
				selectedID = model.boards[clampIndex(model.boardIndex, len(model.boards))].ID
			}
			model.boards = message.boards
			model.boardCounts = message.counts
			model.boardHealth = message.health
			model.boardIndex = clampIndex(model.boardIndex, len(model.boards))
			for index, board := range model.boards {
				if board.ID == selectedID {
					model.boardIndex = index
				}
			}
		}
		return model, nil
	case boardLoadedMsg:
		model.loading = false
		model.err = message.err
		if message.err == nil {
			if model.board != nil && message.board.ID == model.board.ID && message.board.Name != "" {
				board := message.board
				model.board = &board
				for index := range model.boards {
					if model.boards[index].ID == board.ID {
						model.boards[index] = board
					}
				}
			}
			columnID, cardID := model.selectedIDs()
			model.columns = message.columns
			model.cards = make(map[string][]domain.Card, len(model.columns))
			for _, card := range message.cards {
				model.cards[card.ColumnID] = append(model.cards[card.ColumnID], card)
			}
			model.columnIndex = clampIndex(model.columnIndex, len(model.columns))
			model.restoreSelection(columnID, cardID)
			model.applyPendingPaletteFocus()
			if model.planningFilter != planningNone {
				model.applyPlanningFilter(model.planningFilter)
			}
		} else {
			model.pendingColumn = ""
			model.pendingCard = ""
		}
		if message.err == nil && strings.TrimSpace(model.filterText) != "" {
			model.filterLoading = true
			return model, model.searchBoardCards(model.filterText)
		}
		return model, nil
	case archivedCardsLoadedMsg:
		if message.err != nil {
			model.detail = nil
			model.err = message.err
		} else {
			model.detail = archivedCardsDetail(message.cards, message.columns)
		}
		return model, nil
	case cardTemplatesLoadedMsg:
		return model.handleTemplatesLoaded(message)
	case linkCandidatesLoadedMsg:
		if model.form != nil {
			model.form.linksLoading = false
			model.form.linkCandidates = message.candidates
			if message.err != nil {
				model.form.err = "load related cards: " + message.err.Error()
			}
		}
		return model, nil
	case externalEditorPreparedMsg:
		if model.form == nil || model.form.control == nil || model.form.control.kind != commentControl {
			if message.path != "" {
				_ = os.Remove(message.path)
			}
			return model, nil
		}
		if message.err != nil {
			model.form.err = message.err.Error()
			return model, nil
		}
		return model, tea.ExecProcess(message.command, func(err error) tea.Msg {
			return finishExternalEditor(message.path, err)
		})
	case externalEditorFinishedMsg:
		if model.form == nil || model.form.control == nil || model.form.control.kind != commentControl {
			return model, nil
		}
		if message.apply {
			content := normalizeEditorText(message.content, true)
			if model.form.control.markdown && content != model.form.control.value {
				model.form.control.pushUndo()
			}
			model.form.control.value = content
			model.form.control.cursor = len([]rune(content))
		}
		if message.err != nil {
			model.form.err = message.err.Error()
		} else {
			model.form.err = ""
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
			model.filterScores = message.scores
			for _, card := range message.cards {
				model.filteredCards[card.ColumnID] = append(model.filteredCards[card.ColumnID], card)
			}
		}
		return model, nil
	case mutationDoneMsg:
		if message.form != nil {
			message.form.saving = false
			if model.form != message.form {
				return model, nil
			}
			if message.err != nil {
				model.form.err = message.err.Error()
				return model, nil
			}
			model.form = nil
		}
		model.loading = false
		if message.err != nil {
			model.err = message.err
			return model, nil
		}
		model.notice = message.notice
		model.err = nil
		model.loading = true
		if message.scope == boardScreen {
			model.lastMove = nil
		}
		switch message.scope {
		case projectsScreen:
			model.screen = projectsScreen
			return model, model.loadProjects()
		case boardsScreen:
			model.screen = boardsScreen
			return model, model.loadBoards(model.project.ID)
		case boardScreen:
			model.screen = boardScreen
			return model, model.loadBoard(model.board.ID)
		}
		return model, nil
	case columnMoveDoneMsg:
		model.loading = false
		if message.err != nil {
			model.err = message.err
			return model, nil
		}
		model.notice = "Column reordered"
		model.pendingColumn = message.columnID
		model.pendingCard = ""
		model.loading = true
		return model, model.loadBoard(model.board.ID)
	case cardMoveDoneMsg:
		model.loading = false
		if message.err != nil {
			model.pendingColumn = ""
			model.pendingCard = ""
			model.err = message.err
			return model, nil
		}
		if message.undo {
			model.lastMove = nil
			model.notice = "Card move undone"
		} else {
			record := message.record
			model.lastMove = &record
			model.notice = record.notice
		}
		model.err = nil
		model.loading = true
		return model, model.loadBoard(model.board.ID)
	case tea.MouseMsg:
		return model.handleMouse(message)
	case tea.KeyMsg:
		return model.handleKey(message)
	}
	return model, nil
}

func (model *Model) handleKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.String() == "ctrl+c" {
		return model.requestQuit()
	}
	if model.drag != nil {
		if key.String() == "esc" {
			model.cancelCardDrag()
			return model, nil
		}
		model.cancelCardDrag()
	}
	if model.discard != nil {
		return model.handleDiscardKey(key)
	}
	if model.movePicker != nil {
		return model.handleMovePickerKey(key)
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
		case "e":
			if model.detail.kind == "card" {
				return model.openSelectedCardEdit()
			}
		case "E":
			model.detail.expanded = !model.detail.expanded
			model.clampDetailForCurrentLayout()
		case "j", "down":
			model.scrollDetail(1)
		case "k", "up":
			model.scrollDetail(-1)
		case "pgdown":
			_, height := model.dimensions()
			model.scrollDetail(max(height-6, 1))
		case "pgup":
			_, height := model.dimensions()
			model.scrollDetail(-max(height-6, 1))
		case "g", "home":
			model.detail.offset = 0
		case "G", "end":
			model.scrollDetailToEnd()
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
		model.helpOffset = 0
		return model, nil
	case ":":
		model.help = false
		model.commandMode = true
		model.command = ""
		model.commandCursor = 0
		model.commandIndex = 0
		model.commandMenu = ""
		return model, nil
	case "esc":
		if model.help {
			model.help = false
			return model, nil
		}
		if model.screen == boardScreen && model.workspace.focus != boardFocus {
			model.workspace.focus = boardFocus
			return model, nil
		}
		return model.goBack()
	}
	if model.help {
		return model.handleHelpKey(key)
	}
	if model.loading {
		return model, nil
	}
	if key.String() == "/" && model.screen == boardScreen {
		model.filterMode = true
		model.filterCursor = len([]rune(model.filterText))
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
		model.projectIndex = model.moveListSelection(model.projectIndex, len(model.projects), 1, 0)
	case "k", "up":
		model.projectIndex = model.moveListSelection(model.projectIndex, len(model.projects), -1, 0)
	case "l", "right":
		if model.listLayout == cardsLayout {
			model.projectIndex = model.moveListSelection(model.projectIndex, len(model.projects), 0, 1)
			return model, nil
		}
		return model.openSelectedProject()
	case "h", "left":
		if model.listLayout == cardsLayout {
			model.projectIndex = model.moveListSelection(model.projectIndex, len(model.projects), 0, -1)
		}
	case "g", "home":
		model.projectIndex = 0
	case "G", "end":
		model.projectIndex = max(len(model.projects)-1, 0)
	case "enter":
		return model.openSelectedProject()
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
		model.boardIndex = model.moveListSelection(model.boardIndex, len(model.boards), 1, 0)
	case "k", "up":
		model.boardIndex = model.moveListSelection(model.boardIndex, len(model.boards), -1, 0)
	case "l", "right":
		if model.listLayout == cardsLayout {
			model.boardIndex = model.moveListSelection(model.boardIndex, len(model.boards), 0, 1)
			return model, nil
		}
		return model.openSelectedBoard()
	case "g", "home":
		model.boardIndex = 0
	case "G", "end":
		model.boardIndex = max(len(model.boards)-1, 0)
	case "enter":
		return model.openSelectedBoard()
	case "h", "left":
		if model.listLayout == cardsLayout {
			model.boardIndex = model.moveListSelection(model.boardIndex, len(model.boards), 0, -1)
			return model, nil
		}
		return model.goBack()
	}
	return model, nil
}

func (model *Model) openSelectedProject() (tea.Model, tea.Cmd) {
	if len(model.projects) == 0 {
		return model, nil
	}
	selected := model.projects[model.projectIndex]
	if model.project == nil || model.project.ID != selected.ID {
		model.boards = nil
		model.boardIndex = 0
	}
	model.project = &selected
	model.screen = boardsScreen
	model.loading = true
	model.err = nil
	return model, model.loadBoards(selected.ID)
}

func (model *Model) openSelectedBoard() (tea.Model, tea.Cmd) {
	if len(model.boards) == 0 {
		return model, nil
	}
	selected := model.boards[model.boardIndex]
	if model.board == nil || model.board.ID != selected.ID {
		model.columns = nil
		model.cards = make(map[string][]domain.Card)
		model.cardIndexes = make(map[string]int)
	}
	model.board = &selected
	model.workspace.focus = boardFocus
	model.workspace.inspectorOffset = 0
	model.workspace.navigationIndex = model.boardIndex
	model.lastMove = nil
	model.clearBoardFilter()
	model.screen = boardScreen
	model.loading = true
	model.err = nil
	return model, model.loadBoard(selected.ID)
}

func (model *Model) moveListSelection(index, total, rowDelta, columnDelta int) int {
	if total == 0 {
		return 0
	}
	if model.listLayout != cardsLayout {
		return clampIndex(index+rowDelta, total)
	}
	columns := cardLayoutColumns(model.width)
	if rowDelta != 0 {
		return clampIndex(index+(rowDelta*columns), total)
	}
	return clampIndex(index+columnDelta, total)
}

func (model *Model) handleBoardKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "tab", "shift+tab":
		if !model.legacyKeymap {
			model.cyclePaneFocus(key.String() == "shift+tab")
			return model, nil
		}

	}
	for _, action := range workspaceActions {
		if key.String() == action.key {
			return model.executeCommand(action.command)
		}
	}
	if model.workspace.focus != boardFocus {
		return model.handlePaneKey(key)
	}
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
	case "M":
		model.openMovePicker()
		return model, nil
	case "u":
		return model.undoLastCardMove()
	case "shift+left":
		return model.moveSelectedColumn(-1)
	case "shift+right":
		return model.moveSelectedColumn(1)
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

func (model *Model) openSelectedCardEdit() (tea.Model, tea.Cmd) {
	if model.screen != boardScreen || len(model.columns) == 0 || model.selectedCard().ID == "" {
		return model, nil
	}
	model.detail = nil
	return model, model.startCardForm(true)
}

func (model *Model) handleCommandKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		if model.commandMenu != "" {
			model.commandMenu = ""
			model.command = ""
			model.commandCursor = 0
			model.commandIndex = 0
			return model, nil
		}
		model.commandMode = false
		model.command = ""
		model.commandCursor = 0
		model.commandIndex = 0
	case "enter":
		matches := model.paletteMatches()
		if len(matches) == 0 {
			model.err = fmt.Errorf("no matching menu action")
			return model, nil
		}
		selected := matches[clampIndex(model.commandIndex, len(matches))]
		if selected.menu != "" {
			model.commandMenu = selected.menu
			model.command = ""
			model.commandCursor = 0
			model.commandIndex = 0
			return model, nil
		}
		model.commandMode = false
		model.commandMenu = ""
		model.command = ""
		model.commandCursor = 0
		model.commandIndex = 0
		return model.executeCommand(selected.command)
	case "up":
		model.commandIndex = max(model.commandIndex-1, 0)
	case "down", "tab":
		model.commandIndex = min(model.commandIndex+1, max(len(model.paletteMatches())-1, 0))
	default:
		result := editText(model.command, model.commandCursor, key, false)
		if result.handled {
			model.command, model.commandCursor = result.value, result.cursor
		}
		if result.changed {
			model.commandIndex = 0
		}
	}
	return model, nil
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
	case "toggle-inspector", "toggle-navigation", "activity":
		if model.screen != boardScreen {
			model.err = fmt.Errorf("open a board first")
			return model, nil
		}
		switch command {
		case "toggle-inspector":
			if model.width < 110 {
				model.openSelectedDetail()
				if model.detail != nil {
					model.detail.expanded = true
				}
				return model, nil
			}
			model.workspace.inspector = !model.workspace.inspector
		case "toggle-navigation":
			if model.width < 140 {
				return model.goBack()
			}
			model.workspace.navigation = !model.workspace.navigation
		case "activity":
			model.openActivity()
		}
		model.normalizePaneFocus()
		return model, nil

	case "q", "quit":
		return model, tea.Quit
	case "project-add":
		if model.screen != projectsScreen {
			model.err = fmt.Errorf("open the project list first")
			return model, nil
		}
		model.startProjectForm(false)
	case "project-edit":
		if model.screen != projectsScreen || len(model.projects) == 0 {
			model.err = fmt.Errorf("select a project in the project list first")
			return model, nil
		}
		model.startProjectForm(true)
	case "project-delete":
		if model.screen != projectsScreen || len(model.projects) == 0 {
			model.err = fmt.Errorf("select a project in the project list first")
			return model, nil
		}
		return model.handleProjectsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	case "board-add":
		if model.screen != boardsScreen || model.project == nil {
			model.err = fmt.Errorf("open a project's board list first")
			return model, nil
		}
		model.startBoardForm(false)
	case "board-edit":
		if model.screen != boardsScreen || len(model.boards) == 0 {
			model.err = fmt.Errorf("select a board in the board list first")
			return model, nil
		}
		model.startBoardForm(true)
	case "board-delete":
		if model.screen != boardsScreen || len(model.boards) == 0 {
			model.err = fmt.Errorf("select a board in the board list first")
			return model, nil
		}
		return model.handleBoardsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	case "card-add":
		if model.screen != boardScreen || len(model.columns) == 0 {
			model.err = fmt.Errorf("open a board with at least one column first")
			return model, nil
		}
		return model, model.startCardForm(false)
	case "card-edit":
		if model.screen != boardScreen || len(model.columns) == 0 || model.selectedCard().ID == "" {
			model.err = fmt.Errorf("select a card on a board first")
			return model, nil
		}
		return model, model.startCardForm(true)
	case "flow":
		if model.screen != boardScreen || model.board == nil {
			model.err = fmt.Errorf("open a board first")
			return model, nil
		}
		return model, loadFlowCards(model.ctx, model.repo, model.board.ID)
	case "card-queue":
		if model.screen != boardScreen || len(model.columns) == 0 || model.selectedCard().ID == "" {
			model.err = fmt.Errorf("select a card on a board first")
			return model, nil
		}
		return model, model.queueSelectedCard()
	case "card-export":
		if model.screen != boardScreen || len(model.columns) == 0 || model.selectedCard().ID == "" {
			model.err = fmt.Errorf("select a card on a board first")
			return model, nil
		}
		return model, model.exportSelectedCardMarkdown()
	case "card-archive":
		if model.screen != boardScreen || len(model.columns) == 0 || model.selectedCard().ID == "" {
			model.err = fmt.Errorf("select a card on a board first")
			return model, nil
		}
		return model.handleBoardKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	case "add", "new":
		switch model.screen {
		case projectsScreen:
			model.startProjectForm(false)
		case boardsScreen:
			model.startBoardForm(false)
		case boardScreen:
			if len(model.columns) == 0 {
				model.err = fmt.Errorf("add a column first")
			} else {
				return model, model.startCardForm(false)
			}
		}
	case "template", "new-from-template":
		if model.screen != boardScreen || model.board == nil || len(model.columns) == 0 {
			model.err = fmt.Errorf("open a board with at least one column first")
			return model, nil
		}
		model.loading = true
		return model, loadCardTemplates(model.ctx, model.repo, model.board.ID, templateLoadChoose)
	case "new-template":
		if model.screen != boardScreen || model.board == nil {
			model.err = fmt.Errorf("open a board first")
			return model, nil
		}
		model.startTemplateForm()
	case "save-template":
		return model, model.saveSelectedCardAsTemplate()
	case "templates":
		if model.screen != boardScreen || model.board == nil {
			model.err = fmt.Errorf("open a board first")
			return model, nil
		}
		model.loading = true
		return model, loadCardTemplates(model.ctx, model.repo, model.board.ID, templateLoadList)
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
	case "archive", "archive-column":
		if model.screen != boardScreen || len(model.columns) == 0 {
			model.err = fmt.Errorf("open a board column first")
			return model, nil
		}
		column := model.columns[model.columnIndex]
		model.confirm = &confirmModal{
			kind: archiveColumnCards, title: "Archive active column?",
			message: fmt.Sprintf("Archive %d active cards from %s? They can be restored with the CLI.", len(model.visibleCards(column.ID)), column.Name),
			id:      column.ID,
		}
	case "archived":
		if model.screen != boardScreen || model.board == nil {
			model.err = fmt.Errorf("open a board first")
			return model, nil
		}
		model.detail = &detailPopup{kind: "archive", title: "Archived cards", lines: []string{"Loading archived cards..."}}
		return model, loadArchivedCards(model.ctx, model.repo, model.board.ID)
	case "filter", "search-cards":
		if model.screen != boardScreen || model.board == nil {
			model.err = fmt.Errorf("open a board first")
			return model, nil
		}
		model.planningFilter = planningNone
		model.filterMode = true
		model.filterCursor = len([]rune(model.filterText))
		model.notice = ""
	case "today":
		model.applyPlanningFilter(planningToday)
	case "overdue":
		model.applyPlanningFilter(planningOverdue)
	case "blocked":
		model.applyPlanningFilter(planningBlocked)
	case "stale":
		model.applyPlanningFilter(planningStale)
	case "untriaged":
		model.applyPlanningFilter(planningUntriaged)
	case "clear-filter":
		model.clearBoardFilter()
		model.notice = "Filter cleared"
	case "move":
		if model.screen != boardScreen {
			model.err = fmt.Errorf("open a board first")
			return model, nil
		}
		model.openMovePicker()
	case "undo":
		if model.screen != boardScreen {
			model.err = fmt.Errorf("open a board first")
			return model, nil
		}
		return model.undoLastCardMove()
	case "add-column", "new-column":
		if model.screen != boardScreen || model.board == nil {
			model.err = fmt.Errorf("open a board first")
			return model, nil
		}
		model.startColumnForm(false)
	case "column-settings", "edit-column":
		if model.screen != boardScreen || model.board == nil || len(model.columns) == 0 {
			model.err = fmt.Errorf("open a board column first")
			return model, nil
		}
		model.startColumnForm(true)
	case "delete-column":
		if model.screen != boardScreen || model.board == nil || len(model.columns) == 0 {
			model.err = fmt.Errorf("open a board column first")
			return model, nil
		}
		return model.handleBoardKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	case "move-column-left":
		if model.screen != boardScreen || model.board == nil || len(model.columns) == 0 {
			model.err = fmt.Errorf("open a board column first")
			return model, nil
		}
		return model.moveSelectedColumn(-1)
	case "move-column-right":
		if model.screen != boardScreen || model.board == nil || len(model.columns) == 0 {
			model.err = fmt.Errorf("open a board column first")
			return model, nil
		}
		return model.moveSelectedColumn(1)
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
	case "layout":
		if model.listLayout == tableLayout {
			model.listLayout = cardsLayout
		} else {
			model.listLayout = tableLayout
		}
		model.notice = "Layout: " + model.listLayout.String()
	case "layout table":
		model.listLayout = tableLayout
		model.notice = "Layout: table"
	case "layout cards":
		model.listLayout = cardsLayout
		model.notice = "Layout: cards"
	case "settings":
		model.startSettingsForm()
	case "help", "?":
		model.help = true
	case "project", "projects":
		model.screen = projectsScreen
		model.help = false
		model.lastMove = nil
	case "board", "boards":
		if model.project == nil {
			model.err = fmt.Errorf("open a project first")
			return model, nil
		}
		model.screen = boardsScreen
		model.help = false
		model.lastMove = nil
	case "reload":
		model.loading = true
		model.err = nil
		switch model.screen {
		case projectsScreen:
			return model, model.loadProjects()
		case boardsScreen:
			return model, model.loadBoards(model.project.ID)
		case boardScreen:
			return model, model.loadBoard(model.board.ID)
		}
	case "":
	default:
		model.err = fmt.Errorf("unknown command: %s", command)
	}
	return model, nil
}

func (model *Model) goBack() (tea.Model, tea.Cmd) {
	model.loadGeneration++
	model.loading = false
	switch model.screen {
	case boardScreen:
		model.clearBoardFilter()
		model.lastMove = nil
		model.screen = boardsScreen
	case boardsScreen:
		model.screen = projectsScreen
	}
	model.err = nil
	return model, nil
}
