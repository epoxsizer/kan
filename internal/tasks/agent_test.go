package tasks

import (
	"context"
	"sync"
	"testing"

	"github.com/epoxsizer/kan/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestAgentLifecycleQueueClaimComplete(t *testing.T) {
	ctx := context.Background()
	coordinator := openTestCoordinator(t)
	board, first, _ := createHierarchy(t, coordinator)
	card := createCard(t, coordinator, board, first, "Task")

	queued, err := coordinator.QueueCard(ctx, card.ID)
	require.NoError(t, err)
	require.Equal(t, domain.AgentQueued, queued.AgentStatus)

	claimed, err := coordinator.ClaimCard(ctx, card.ID, "runner-1")
	require.NoError(t, err)
	require.Equal(t, domain.AgentRunning, claimed.AgentStatus)
	require.Equal(t, "runner-1", claimed.AgentName)

	_, err = coordinator.ClaimCard(ctx, card.ID, "runner-2")
	require.ErrorIs(t, err, domain.ErrConflict, "a running card cannot be claimed twice")

	done, err := coordinator.CompleteCard(ctx, card.ID, true, "all good")
	require.NoError(t, err)
	require.Equal(t, domain.AgentDone, done.AgentStatus)
	require.Equal(t, "all good", done.AgentResult)

	_, err = coordinator.CompleteCard(ctx, card.ID, false, "again")
	require.ErrorIs(t, err, domain.ErrConflict, "only a running card can complete")

	failed, err := coordinator.QueueCard(ctx, card.ID)
	require.NoError(t, err)
	require.Equal(t, domain.AgentQueued, failed.AgentStatus)
	require.Empty(t, failed.AgentResult, "re-queueing clears the previous result")
}

func TestConcurrentClaimsOnlyOneWins(t *testing.T) {
	ctx := context.Background()
	coordinator := openTestCoordinator(t)
	board, first, _ := createHierarchy(t, coordinator)
	card := createCard(t, coordinator, board, first, "Race")
	_, err := coordinator.QueueCard(ctx, card.ID)
	require.NoError(t, err)

	const claimers = 8
	results := make(chan error, claimers)
	var wg sync.WaitGroup
	for range claimers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, claimErr := coordinator.ClaimCard(ctx, card.ID, "runner")
			results <- claimErr
		}()
	}
	wg.Wait()
	close(results)
	wins := 0
	for claimErr := range results {
		if claimErr == nil {
			wins++
		} else {
			require.ErrorIs(t, claimErr, domain.ErrConflict)
		}
	}
	require.Equal(t, 1, wins, "exactly one claimer wins the card")
}
