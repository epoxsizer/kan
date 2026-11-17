package mcpserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/epoxsizer/kan/internal/tasks"
)

// Intake configures the webhook capture endpoint (POST /cards). Empty board
// and column IDs reject captures unless the request supplies its own.
type Intake struct {
	BoardID  string
	ColumnID string
	Desktop  func(title, body string)
}

func intakeHandler(coordinator *tasks.Coordinator, intake Intake, logger *slog.Logger, notify func(Change)) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var input struct {
			Title       string   `json:"title"`
			Description string   `json:"description"`
			BoardID     string   `json:"board_id"`
			ColumnID    string   `json:"column_id"`
			Priority    string   `json:"priority"`
			DueDate     string   `json:"due_date"`
			Tags        []string `json:"tags"`
		}
		request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			http.Error(writer, "invalid JSON body", http.StatusBadRequest)
			return
		}
		boardID, columnID := input.BoardID, input.ColumnID
		if boardID == "" {
			boardID = intake.BoardID
		}
		if columnID == "" {
			columnID = intake.ColumnID
		}
		if boardID == "" || columnID == "" {
			http.Error(writer, "no capture destination: configure the inbox or pass board_id and column_id", http.StatusBadRequest)
			return
		}
		card := domain.Card{BoardID: boardID, ColumnID: columnID, Title: strings.TrimSpace(input.Title), Description: input.Description, Tags: input.Tags}
		if strings.TrimSpace(input.Priority) != "" {
			card.Priority = &input.Priority
		}
		if strings.TrimSpace(input.DueDate) != "" {
			due, err := time.Parse("2006-01-02", input.DueDate)
			if err != nil {
				http.Error(writer, "due_date must use YYYY-MM-DD", http.StatusBadRequest)
				return
			}
			parsed := due.UTC()
			card.DueDate = &parsed
		}
		if err := coordinator.CreateCardAtEnd(request.Context(), &card); err != nil {
			status := http.StatusBadRequest
			switch {
			case errors.Is(err, domain.ErrNotFound):
				status = http.StatusNotFound
			case errors.Is(err, domain.ErrConflict):
				status = http.StatusConflict
			}
			http.Error(writer, err.Error(), status)
			return
		}
		if notify != nil {
			notify(Change{Action: "card captured", BoardID: card.BoardID, ColumnID: card.ColumnID, CardID: card.ID})
		}
		if intake.Desktop != nil {
			intake.Desktop("Kan: new card", card.Title)
		}
		logger.Info("card captured via webhook", "card", card.ID, "board", card.BoardID)
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(writer).Encode(map[string]any{"card": card})
	})
}
