package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/aigc/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateRequestStateUsesExpectedStatus(t *testing.T) {
	repository := newRequestRepository(t)
	request := &entity.AigcRequest{
		RequestID: "turn-state", GenerationID: "aigc_gen_state", UserID: 7, TokenID: 11,
		GroupName: "default", PublicModelID: "video", UpstreamModelID: "video-upstream",
		ModelType: "video", Mode: "text_to_video", Status: entity.RequestStatusSubmitted, RequestDigest: strings.Repeat("c", 64),
	}
	created, _, err := repository.CreateOrGetRequest(context.Background(), request)
	require.NoError(t, err)

	updated, err := repository.UpdateRequestState(context.Background(), created.ID, entity.RequestStatusSubmitted, entity.RequestStateUpdate{
		Status: entity.RequestStatusQueued, Progress: 10, NativeTaskID: "task-1", ResultJSON: `{"outputs":[]}`,
	})
	require.NoError(t, err)
	assert.Equal(t, entity.RequestStatusQueued, updated.Status)
	assert.Equal(t, "task-1", updated.NativeTaskID)

	_, err = repository.UpdateRequestState(context.Background(), created.ID, entity.RequestStatusSubmitted, entity.RequestStateUpdate{Status: entity.RequestStatusCompleted})
	assert.ErrorIs(t, err, entity.ErrRequestStateConflict)
}
