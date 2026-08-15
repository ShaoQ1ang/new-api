package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/aigc/entity"
	"github.com/QuantumNous/new-api/aigc/repository"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newAdminService(t *testing.T) (*AdminService, *repository.Repository) {
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
	profiles := repository.New(db)
	return NewAdminService(profiles), profiles
}

func TestAdminCreateDraftNormalizesGroups(t *testing.T) {
	admin, profiles := newAdminService(t)
	input := ProfileInput{
		PublicModelID: " writer-pro ", DisplayName: " Writer Pro ", ModelType: "text",
		Groups: []string{"vip", "default", "vip", ""},
		Config: json.RawMessage(`{"text":{"upstream_model_id":"gpt-5"}}`),
	}

	created, err := admin.Create(context.Background(), input)

	require.NoError(t, err)
	assert.Equal(t, "writer-pro", created.PublicModelID)
	assert.Equal(t, entity.ModelStatusDraft, created.Status)
	assert.Equal(t, []string{"default", "vip"}, created.Groups)
	loaded, loadErr := profiles.GetProfileByPublicID(context.Background(), "writer-pro")
	require.NoError(t, loadErr)
	assert.Equal(t, `["default","vip"]`, loaded.GroupsJSON)
}

func TestAdminAllowsIncompleteDraftButValidateRejectsIt(t *testing.T) {
	admin, _ := newAdminService(t)
	created, err := admin.Create(context.Background(), ProfileInput{
		PublicModelID: "draft-image", DisplayName: "Draft Image", ModelType: "image",
		Config: json.RawMessage(`{"image":{"adapter":"image-relay","modes":{}}}`),
	})
	require.NoError(t, err)

	err = admin.Validate(context.Background(), created.ID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "image mode")
}

func TestAdminUpdateRequiresCurrentVersionAndCanDisable(t *testing.T) {
	admin, _ := newAdminService(t)
	created, err := admin.Create(context.Background(), ProfileInput{
		PublicModelID: "music-public", DisplayName: "Music", ModelType: "music",
		Config: json.RawMessage(`{"music":{"adapter":"music-task","modes":{}}}`),
	})
	require.NoError(t, err)

	updated, err := admin.Update(context.Background(), created.ID, ProfileInput{
		PublicModelID: created.PublicModelID, DisplayName: "Music Updated", ModelType: created.ModelType,
		Config: created.Config, ConfigVersion: created.ConfigVersion,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, updated.ConfigVersion)
	assert.Equal(t, "Music Updated", updated.DisplayName)

	err = admin.Disable(context.Background(), created.ID, updated.ConfigVersion)
	require.NoError(t, err)
	loaded, loadErr := admin.Get(context.Background(), created.ID)
	require.NoError(t, loadErr)
	assert.Equal(t, entity.ModelStatusDisabled, loaded.Status)
}

func TestAdminListReturnsPaginatedProfiles(t *testing.T) {
	admin, _ := newAdminService(t)
	for _, id := range []string{"model-c", "model-a", "model-b"} {
		_, err := admin.Create(context.Background(), ProfileInput{
			PublicModelID: id, DisplayName: id, ModelType: "text",
			Config: json.RawMessage(`{"text":{"upstream_model_id":"gpt-5"}}`),
		})
		require.NoError(t, err)
	}

	items, total, err := admin.List(context.Background(), ProfileFilter{ModelType: "text", Page: 1, PageSize: 2})

	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	require.Len(t, items, 2)
	assert.Equal(t, "model-a", items[0].PublicModelID)
	assert.Equal(t, "model-b", items[1].PublicModelID)
}
