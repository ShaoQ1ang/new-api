package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/aigc/entity"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newProfileRepository(t *testing.T) *Repository {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&entity.ModelProfile{}))
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return New(db)
}

func TestModelProfileCreateAndLoadByPublicID(t *testing.T) {
	repository := newProfileRepository(t)
	profile := &entity.ModelProfile{
		PublicModelID: "happyhorse-1.1",
		DisplayName:   "HappyHorse 1.1",
		ModelType:     "video",
		Status:        entity.ModelStatusDraft,
		GroupsJSON:    `["default"]`,
		ConfigJSON:    `{"video":{}}`,
	}

	require.NoError(t, repository.CreateProfile(context.Background(), profile))
	assert.NotZero(t, profile.ID)
	assert.Equal(t, 1, profile.ConfigVersion)
	assert.NotZero(t, profile.CreatedTime)

	loaded, err := repository.GetProfileByPublicID(context.Background(), profile.PublicModelID)
	require.NoError(t, err)
	assert.Equal(t, profile.PublicModelID, loaded.PublicModelID)
	assert.Equal(t, profile.ConfigJSON, loaded.ConfigJSON)
}

func TestModelProfilePublicIDIsUnique(t *testing.T) {
	repository := newProfileRepository(t)
	first := &entity.ModelProfile{PublicModelID: "public-model", DisplayName: "One", ModelType: "text", Status: entity.ModelStatusDraft, ConfigJSON: `{"text":{}}`}
	second := &entity.ModelProfile{PublicModelID: "public-model", DisplayName: "Two", ModelType: "text", Status: entity.ModelStatusDraft, ConfigJSON: `{"text":{}}`}

	require.NoError(t, repository.CreateProfile(context.Background(), first))
	require.Error(t, repository.CreateProfile(context.Background(), second))
}

func TestModelProfileUpdateUsesOptimisticVersion(t *testing.T) {
	repository := newProfileRepository(t)
	profile := &entity.ModelProfile{PublicModelID: "image-public", DisplayName: "Image", ModelType: "image", Status: entity.ModelStatusDraft, ConfigJSON: `{"image":{}}`}
	require.NoError(t, repository.CreateProfile(context.Background(), profile))

	profile.DisplayName = "Image Updated"
	require.NoError(t, repository.UpdateProfile(context.Background(), profile, 1))
	assert.Equal(t, 2, profile.ConfigVersion)

	profile.DisplayName = "Stale Update"
	err := repository.UpdateProfile(context.Background(), profile, 1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrVersionConflict))

	loaded, loadErr := repository.GetProfileByPublicID(context.Background(), profile.PublicModelID)
	require.NoError(t, loadErr)
	assert.Equal(t, "Image Updated", loaded.DisplayName)
	assert.Equal(t, 2, loaded.ConfigVersion)
}

func TestListPublishedProfilesFiltersType(t *testing.T) {
	repository := newProfileRepository(t)
	profiles := []*entity.ModelProfile{
		{PublicModelID: "image-published", DisplayName: "Published Image", ModelType: "image", Status: entity.ModelStatusPublished, ConfigJSON: `{"image":{}}`},
		{PublicModelID: "image-draft", DisplayName: "Draft Image", ModelType: "image", Status: entity.ModelStatusDraft, ConfigJSON: `{"image":{}}`},
		{PublicModelID: "video-published", DisplayName: "Published Video", ModelType: "video", Status: entity.ModelStatusPublished, ConfigJSON: `{"video":{}}`},
	}
	for _, profile := range profiles {
		require.NoError(t, repository.CreateProfile(context.Background(), profile))
	}

	items, err := repository.ListPublishedProfiles(context.Background(), "image")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "image-published", items[0].PublicModelID)
}
