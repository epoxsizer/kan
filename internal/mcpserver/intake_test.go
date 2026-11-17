package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/epoxsizer/kan/internal/config"
	"github.com/epoxsizer/kan/internal/domain"
	"github.com/stretchr/testify/require"
)

const intakeToken = "12345678901234567890123456789012"

func TestIntakeCapturesCardAndNotifies(t *testing.T) {
	ctx := context.Background()
	coordinator, project := openTestCoordinator(t)
	board := domain.Board{ProjectID: project.ID, Name: "Board", Position: 1024}
	require.NoError(t, coordinator.CreateBoard(ctx, &board))
	column := domain.Column{BoardID: board.ID, Name: "Inbox", Position: 1024}
	require.NoError(t, coordinator.CreateColumn(ctx, &column))

	var changes []Change
	var desktop []string
	server, err := Start(config.MCP{Enabled: true, Address: "127.0.0.1:0", Token: intakeToken}, "test", coordinator, nil,
		func(change Change) { changes = append(changes, change) },
		Intake{BoardID: board.ID, ColumnID: column.ID, Desktop: func(title, body string) { desktop = append(desktop, title+"|"+body) }})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, server.Shutdown(ctx)) })

	endpoint := "http://" + server.Address() + "/cards"
	body := `{"title":"From webhook","description":"hi","tags":["capture"]}`
	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBufferString(body))
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer "+intakeToken)
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusCreated, response.StatusCode)
	var payload struct {
		Card domain.Card `json:"card"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&payload))
	require.Equal(t, "From webhook", payload.Card.Title)
	require.Equal(t, column.ID, payload.Card.ColumnID)

	cards, err := coordinator.ListCards(ctx, board.ID)
	require.NoError(t, err)
	require.Len(t, cards, 1)
	require.Len(t, changes, 1)
	require.Equal(t, "card captured", changes[0].Action)
	require.Len(t, desktop, 1)
	require.Contains(t, desktop[0], "From webhook")

	unauthorized, err := http.Post(endpoint, "application/json", bytes.NewBufferString(body))
	require.NoError(t, err)
	unauthorized.Body.Close()
	require.Equal(t, http.StatusUnauthorized, unauthorized.StatusCode)

	invalid, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBufferString(`{"title":""}`))
	require.NoError(t, err)
	invalid.Header.Set("Authorization", "Bearer "+intakeToken)
	invalidResponse, err := http.DefaultClient.Do(invalid)
	require.NoError(t, err)
	invalidResponse.Body.Close()
	require.Equal(t, http.StatusBadRequest, invalidResponse.StatusCode)

	wrongMethod, err := http.NewRequest(http.MethodGet, endpoint, nil)
	require.NoError(t, err)
	wrongMethod.Header.Set("Authorization", "Bearer "+intakeToken)
	methodResponse, err := http.DefaultClient.Do(wrongMethod)
	require.NoError(t, err)
	methodResponse.Body.Close()
	require.Equal(t, http.StatusMethodNotAllowed, methodResponse.StatusCode)
}

func TestIntakeRequiresDestination(t *testing.T) {
	ctx := context.Background()
	coordinator, _ := openTestCoordinator(t)
	server, err := Start(config.MCP{Enabled: true, Address: "127.0.0.1:0", Token: intakeToken}, "test", coordinator, nil, nil, Intake{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, server.Shutdown(ctx)) })

	request, err := http.NewRequest(http.MethodPost, "http://"+server.Address()+"/cards", bytes.NewBufferString(`{"title":"No home"}`))
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer "+intakeToken)
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusBadRequest, response.StatusCode)
}
