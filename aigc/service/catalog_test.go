package service

import (
	"context"
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/aigc/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type profileStoreStub struct {
	profiles map[string]*entity.ModelProfile
	updated  *entity.ModelProfile
}

func (store *profileStoreStub) GetProfileByPublicID(_ context.Context, publicModelID string) (*entity.ModelProfile, error) {
	profile, ok := store.profiles[publicModelID]
	if !ok {
		return nil, errors.New("not found")
	}
	copy := *profile
	return &copy, nil
}

func (store *profileStoreStub) UpdateProfile(_ context.Context, profile *entity.ModelProfile, expectedVersion int) error {
	if profile.ConfigVersion != expectedVersion {
		return errors.New("unexpected version")
	}
	profile.ConfigVersion++
	copy := *profile
	store.updated = &copy
	store.profiles[profile.PublicModelID] = &copy
	return nil
}

func (store *profileStoreStub) ListPublishedProfiles(_ context.Context, modelType string) ([]entity.ModelProfile, error) {
	items := make([]entity.ModelProfile, 0)
	for _, profile := range store.profiles {
		if profile.Status == entity.ModelStatusPublished && (modelType == "" || modelType == profile.ModelType) {
			items = append(items, *profile)
		}
	}
	return items, nil
}

type availabilityStub struct {
	byGroup map[string]map[string]bool
}

func (stub availabilityStub) Available(_ context.Context, group string, upstreamModelIDs []string) (map[string]bool, error) {
	result := make(map[string]bool, len(upstreamModelIDs))
	for _, modelID := range upstreamModelIDs {
		result[modelID] = stub.byGroup[group][modelID]
	}
	return result, nil
}

func validTextProfile(status int) *entity.ModelProfile {
	return &entity.ModelProfile{
		PublicModelID: "writer-pro", DisplayName: "Writer Pro", ModelType: "text", Status: status,
		GroupsJSON: `[]`, ConfigJSON: `{"text":{"upstream_model_id":"gpt-5","max_output_tokens":8192}}`, ConfigVersion: 1,
	}
}

func TestPublishRejectsInvalidCapabilityConfiguration(t *testing.T) {
	profile := validTextProfile(entity.ModelStatusDraft)
	profile.ConfigJSON = `{"text":{"upstream_model_id":""}}`
	store := &profileStoreStub{profiles: map[string]*entity.ModelProfile{profile.PublicModelID: profile}}
	service := NewCatalogService(store, availabilityStub{})

	err := service.Publish(context.Background(), profile.PublicModelID, profile.ConfigVersion)

	require.Error(t, err)
	assert.Nil(t, store.updated)
	assert.Contains(t, err.Error(), "upstream model")
}

func TestPublishRejectsProfileWithoutAnyAvailableRoute(t *testing.T) {
	profile := validTextProfile(entity.ModelStatusDraft)
	store := &profileStoreStub{profiles: map[string]*entity.ModelProfile{profile.PublicModelID: profile}}
	service := NewCatalogService(store, availabilityStub{byGroup: map[string]map[string]bool{"": {"gpt-5": false}}})

	err := service.Publish(context.Background(), profile.PublicModelID, profile.ConfigVersion)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUpstreamUnavailable)
	assert.Nil(t, store.updated)
}

func TestPublishMarksValidProfilePublished(t *testing.T) {
	profile := validTextProfile(entity.ModelStatusDraft)
	store := &profileStoreStub{profiles: map[string]*entity.ModelProfile{profile.PublicModelID: profile}}
	service := NewCatalogService(store, availabilityStub{byGroup: map[string]map[string]bool{"": {"gpt-5": true}}})

	err := service.Publish(context.Background(), profile.PublicModelID, profile.ConfigVersion)

	require.NoError(t, err)
	require.NotNil(t, store.updated)
	assert.Equal(t, entity.ModelStatusPublished, store.updated.Status)
	assert.Equal(t, 2, store.updated.ConfigVersion)
}

func TestListCatalogFiltersGroupAndRedactsRouting(t *testing.T) {
	visible := validTextProfile(entity.ModelStatusPublished)
	visible.GroupsJSON = `["vip"]`
	hiddenByGroup := &entity.ModelProfile{
		PublicModelID: "default-only", DisplayName: "Default", ModelType: "text", Status: entity.ModelStatusPublished,
		GroupsJSON: `["default"]`, ConfigJSON: `{"text":{"upstream_model_id":"gpt-default"}}`, ConfigVersion: 1,
	}
	hiddenByRoute := &entity.ModelProfile{
		PublicModelID: "offline", DisplayName: "Offline", ModelType: "text", Status: entity.ModelStatusPublished,
		GroupsJSON: `["vip"]`, ConfigJSON: `{"text":{"upstream_model_id":"offline-model"}}`, ConfigVersion: 1,
	}
	store := &profileStoreStub{profiles: map[string]*entity.ModelProfile{
		visible.PublicModelID:       visible,
		hiddenByGroup.PublicModelID: hiddenByGroup,
		hiddenByRoute.PublicModelID: hiddenByRoute,
	}}
	service := NewCatalogService(store, availabilityStub{byGroup: map[string]map[string]bool{
		"vip": {"gpt-5": true, "gpt-default": true, "offline-model": false},
	}})

	items, err := service.List(context.Background(), "vip", "text")

	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "writer-pro", items[0].ID)
	assert.NotContains(t, string(items[0].Capabilities), "gpt-5")
	assert.NotContains(t, string(items[0].Capabilities), "upstream_model_id")
}
