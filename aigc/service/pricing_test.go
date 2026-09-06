package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/aigc/entity"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pricingSourceStub struct {
	items []model.Pricing
}

func (stub pricingSourceStub) Pricing() []model.Pricing { return stub.items }

func TestPricingServiceProjectsPublishedRoutesByPublicModel(t *testing.T) {
	profiles := map[string]*entity.ModelProfile{
		"happyhorse-1.1": {
			PublicModelID: "happyhorse-1.1", DisplayName: "HappyHorse", ModelType: "video", Status: entity.ModelStatusPublished,
			GroupsJSON: `["default"]`, ConfigVersion: 1,
			ConfigJSON: `{"video":{"adapter":"openrouter-video","task_protocol":"newapi-video","modes":{"text_to_video":{"upstream_model_id":"hh-t2v"},"first_frame":{"upstream_model_id":"hh-i2v","inputs":{"first_frame":{"min":1,"max":1}}}},"output_specs":[{"id":"base","modes":["text_to_video"],"resolutions":["720p"],"aspect_ratios":["16:9"],"durations":[5],"generate_audio":{"supported":true,"default":true}},{"id":"image-hq","modes":["first_frame"],"resolutions":["1080p"],"aspect_ratios":["16:9"],"durations":[5],"generate_audio":{"supported":true,"default":true},"target":{"upstream_model_id":"hh-i2v-hq"}}]}}`,
		},
		"image-fixed": {
			PublicModelID: "image-fixed", DisplayName: "Image", ModelType: "image", Status: entity.ModelStatusPublished,
			GroupsJSON: `[]`, ConfigVersion: 1,
			ConfigJSON: `{"image":{"adapter":"openai-image","modes":{"text_to_image":{"upstream_model_id":"image-upstream","output":{"sizes":["1024x1024"],"counts":[1],"default_size":"1024x1024","default_count":1}}}}}`,
		},
		"writer": {
			PublicModelID: "writer", DisplayName: "Writer", ModelType: "text", Status: entity.ModelStatusPublished,
			GroupsJSON: `[]`, ConfigVersion: 1, ConfigJSON: `{"text":{"upstream_model_id":"writer-upstream"}}`,
		},
		"wan-public": {
			PublicModelID: "wan-public", DisplayName: "Wan", ModelType: "video", Status: entity.ModelStatusPublished,
			GroupsJSON: `[]`, ConfigVersion: 1,
			ConfigJSON: `{"video":{"adapter":"video-task","task_protocol":"newapi-video","modes":{"text_to_video":{"upstream_model_id":"wan-upstream"}},"output_specs":[{"id":"base","modes":["text_to_video"],"resolutions":["720p"],"aspect_ratios":["16:9"],"durations":[5],"generate_audio":{"supported":false,"default":false}}]}}`,
		},
	}
	availability := availabilityStub{byGroup: map[string]map[string]bool{"default": {
		"hh-t2v": true, "hh-i2v": true, "hh-i2v-hq": true, "image-upstream": true, "writer-upstream": true, "wan-upstream": true,
	}}}
	pricing := pricingSourceStub{items: []model.Pricing{
		{ModelName: "hh-t2v", QuotaType: 1, VideoSecondsPrice: map[string]map[string]float64{"720p": {"default": 0.12, "silent": 0.08}}},
		{ModelName: "hh-i2v-hq", QuotaType: 1, VideoSecondsPrice: map[string]map[string]float64{"1080p": {"default": 0.2}}},
		{ModelName: "image-upstream", QuotaType: 1, ModelPrice: 0.05},
		{ModelName: "writer-upstream", QuotaType: 0, ModelRatio: 0.5, CompletionRatio: 4},
		{ModelName: "wan-upstream", QuotaType: 0, ModelRatio: 0.025, CompletionRatio: 1},
	}}
	service := NewPricingService(&profileStoreStub{profiles: profiles}, availability, pricing)

	document, err := service.List(context.Background(), "default")

	require.NoError(t, err)
	require.Len(t, document.Models, 4)
	assert.Equal(t, "USD", document.Currency)
	assert.NotEmpty(t, document.PricingVersion)
	byID := make(map[string]PublicModelPricing)
	for _, item := range document.Models {
		byID[item.ModelID] = item
	}
	require.Len(t, byID["happyhorse-1.1"].Routes, 2)
	assert.Equal(t, "second", byID["happyhorse-1.1"].Routes[0].BillingUnit)
	assert.Equal(t, []string{"16:9"}, byID["happyhorse-1.1"].Routes[0].AspectRatios)
	assert.Equal(t, []int{5}, byID["happyhorse-1.1"].Routes[0].Durations)
	assert.Equal(t, "0.200000", byID["happyhorse-1.1"].Routes[0].VideoSecondsPrice["1080p"]["default"])
	assert.Equal(t, "generation", byID["image-fixed"].Routes[0].BillingUnit)
	assert.Equal(t, "0.050000", byID["image-fixed"].Routes[0].GenerationPrice)
	assert.Equal(t, "token", byID["writer"].Routes[0].BillingUnit)
	assert.Equal(t, "1.000000", byID["writer"].Routes[0].InputPricePerMillionTokens)
	assert.Equal(t, "4.000000", byID["writer"].Routes[0].OutputPricePerMillionTokens)
	assert.Equal(t, "dynamic", byID["wan-public"].Routes[0].BillingUnit)

	repeated, err := service.List(context.Background(), "default")
	require.NoError(t, err)
	assert.Equal(t, document.PricingVersion, repeated.PricingVersion)
}

func TestPricingServiceUsesConfiguredQuotaPerUnitForTokenPrices(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 250_000
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })

	profile := validTextProfile(entity.ModelStatusPublished)
	service := NewPricingService(
		&profileStoreStub{profiles: map[string]*entity.ModelProfile{profile.PublicModelID: profile}},
		availabilityStub{byGroup: map[string]map[string]bool{"default": {"gpt-5": true}}},
		pricingSourceStub{items: []model.Pricing{{
			ModelName: "gpt-5", QuotaType: 0, ModelRatio: 0.5, CompletionRatio: 4,
		}}},
	)

	document, err := service.List(context.Background(), "default")

	require.NoError(t, err)
	require.Len(t, document.Models, 1)
	require.Len(t, document.Models[0].Routes, 1)
	assert.Equal(t, "2.000000", document.Models[0].Routes[0].InputPricePerMillionTokens)
	assert.Equal(t, "8.000000", document.Models[0].Routes[0].OutputPricePerMillionTokens)
}

func TestPricingServiceRejectsNonPositiveQuotaPerUnit(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 0
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })

	_, err := NewPricingService(nil, nil, nil).List(context.Background(), "default")

	require.EqualError(t, err, "quota per unit must be positive and finite")
}

func TestPricingServiceAppliesEffectiveGroupRatioOnce(t *testing.T) {
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	originalGroupGroupRatios := ratio_setting.GroupGroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"vip":9}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"vip":{"vip":1.5}}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(originalGroupGroupRatios))
	})

	profile := validTextProfile(entity.ModelStatusPublished)
	profile.GroupsJSON = `["vip"]`
	service := NewPricingService(
		&profileStoreStub{profiles: map[string]*entity.ModelProfile{profile.PublicModelID: profile}},
		availabilityStub{byGroup: map[string]map[string]bool{"vip": {"gpt-5": true}}},
		pricingSourceStub{items: []model.Pricing{{
			ModelName: "gpt-5", QuotaType: 0, ModelRatio: 0.5, CompletionRatio: 4,
		}}},
	)

	document, err := service.List(context.Background(), "vip")

	require.NoError(t, err)
	assert.Equal(t, "1.500000", document.EffectiveGroupRatio)
	require.Len(t, document.Models, 1)
	require.Len(t, document.Models[0].Routes, 1)
	assert.Equal(t, "1.500000", document.Models[0].Routes[0].InputPricePerMillionTokens)
	assert.Equal(t, "6.000000", document.Models[0].Routes[0].OutputPricePerMillionTokens)
}

func TestPricingServiceOmitsProfilesAndRoutesUnavailableToGroup(t *testing.T) {
	visible := validTextProfile(entity.ModelStatusPublished)
	visible.GroupsJSON = `["vip"]`
	hidden := validTextProfile(entity.ModelStatusPublished)
	hidden.PublicModelID = "default-only"
	hidden.GroupsJSON = `["default"]`
	store := &profileStoreStub{profiles: map[string]*entity.ModelProfile{visible.PublicModelID: visible, hidden.PublicModelID: hidden}}
	service := NewPricingService(store, availabilityStub{byGroup: map[string]map[string]bool{"vip": {"gpt-5": true}}}, pricingSourceStub{items: []model.Pricing{{ModelName: "gpt-5", QuotaType: 1, ModelPrice: 1}}})

	document, err := service.List(context.Background(), "vip")

	require.NoError(t, err)
	require.Len(t, document.Models, 1)
	assert.Equal(t, "writer-pro", document.Models[0].ModelID)
}
