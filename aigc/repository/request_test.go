package repository

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/aigc/entity"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newRequestRepository(t *testing.T) *Repository {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&entity.AigcRequest{}))
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return New(db)
}

func TestCreateOrGetRequestIsIdempotentPerUserAndRequestID(t *testing.T) {
	repository := newRequestRepository(t)
	first := &entity.AigcRequest{
		RequestID: "turn-1", GenerationID: "aigc_gen_first", UserID: 7, TokenID: 11,
		GroupName: "default", PublicModelID: "writer-pro", UpstreamModelID: "gpt-5",
		ModelType: "text", Mode: "text", Status: entity.RequestStatusSubmitted, RequestDigest: strings.Repeat("a", 64),
	}

	created, wasCreated, err := repository.CreateOrGetRequest(context.Background(), first)
	require.NoError(t, err)
	assert.True(t, wasCreated)
	assert.Equal(t, "aigc_gen_first", created.GenerationID)

	duplicate := *first
	duplicate.ID = 0
	duplicate.GenerationID = "aigc_gen_second"
	existing, wasCreated, err := repository.CreateOrGetRequest(context.Background(), &duplicate)
	require.NoError(t, err)
	assert.False(t, wasCreated)
	assert.Equal(t, "aigc_gen_first", existing.GenerationID)

	otherUser := duplicate
	otherUser.UserID = 8
	otherUser.GenerationID = "aigc_gen_other_user"
	_, wasCreated, err = repository.CreateOrGetRequest(context.Background(), &otherUser)
	require.NoError(t, err)
	assert.True(t, wasCreated)
}

func TestGetRequestByGenerationIDIsScopedToUser(t *testing.T) {
	repository := newRequestRepository(t)
	request := &entity.AigcRequest{
		RequestID: "turn-private", GenerationID: "aigc_gen_private", UserID: 7, TokenID: 11,
		GroupName: "default", PublicModelID: "writer-pro", UpstreamModelID: "gpt-5",
		ModelType: "text", Mode: "text", Status: entity.RequestStatusSubmitted, RequestDigest: strings.Repeat("b", 64),
	}
	_, _, err := repository.CreateOrGetRequest(context.Background(), request)
	require.NoError(t, err)

	loaded, err := repository.GetRequestByGenerationID(context.Background(), 7, request.GenerationID)
	require.NoError(t, err)
	assert.Equal(t, request.RequestID, loaded.RequestID)
	byRequestID, err := repository.GetRequestByUserRequestID(context.Background(), 7, request.RequestID)
	require.NoError(t, err)
	assert.Equal(t, request.GenerationID, byRequestID.GenerationID)

	_, err = repository.GetRequestByGenerationID(context.Background(), 8, request.GenerationID)
	assert.ErrorIs(t, err, entity.ErrGenerationNotFound)
}
