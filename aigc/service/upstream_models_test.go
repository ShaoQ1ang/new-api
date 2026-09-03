package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type upstreamModelSourceStub struct {
	pricing   []model.Pricing
	abilities []model.AbilityWithChannel
	err       error
}

func (stub upstreamModelSourceStub) Pricing() []model.Pricing {
	return stub.pricing
}

func (stub upstreamModelSourceStub) Abilities() ([]model.AbilityWithChannel, error) {
	return stub.abilities, stub.err
}

func TestUpstreamModelServiceAggregatesPricingAndAbilities(t *testing.T) {
	service := NewUpstreamModelService(upstreamModelSourceStub{
		pricing: []model.Pricing{
			{
				ModelName: "video-pro", Description: "Video", EnableGroup: []string{"vip", "default", "vip"},
				SupportedEndpointTypes: []constant.EndpointType{constant.EndpointTypeOpenAIVideo, constant.EndpointTypeOpenAI, constant.EndpointTypeOpenAIVideo},
				QuotaType:              1, ModelPrice: 2.5, BillingMode: "video_seconds",
			},
		},
		abilities: []model.AbilityWithChannel{
			{Ability: model.Ability{Model: "video-pro", Group: "vip", ChannelId: 9, Enabled: true}},
			{Ability: model.Ability{Model: "video-pro", Group: "default", ChannelId: 9, Enabled: true}},
			{Ability: model.Ability{Model: "video-pro", Group: "default", ChannelId: 10, Enabled: true}},
			{Ability: model.Ability{Model: "ability-only", Group: "default", ChannelId: 11, Enabled: true}},
		},
	})

	items, err := service.List(context.Background())

	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "ability-only", items[0].ID)
	assert.False(t, items[0].Pricing.Available)
	assert.Equal(t, 1, items[0].ChannelCount)
	assert.Equal(t, []string{"default"}, items[0].Groups)
	assert.Equal(t, "video-pro", items[1].ID)
	assert.Equal(t, []string{"default", "vip"}, items[1].Groups)
	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAI, constant.EndpointTypeOpenAIVideo}, items[1].SupportedEndpointTypes)
	assert.Equal(t, 2, items[1].ChannelCount)
	assert.True(t, items[1].Pricing.Available)
	assert.Equal(t, 2.5, items[1].Pricing.ModelPrice)
	assert.Equal(t, "video_seconds", items[1].Pricing.BillingMode)
}

func TestUpstreamModelServiceGetsOneModel(t *testing.T) {
	service := NewUpstreamModelService(upstreamModelSourceStub{
		pricing: []model.Pricing{{ModelName: "openai/gpt-image-1", EnableGroup: []string{"default"}}},
	})

	item, err := service.Get(context.Background(), "openai/gpt-image-1")
	require.NoError(t, err)
	assert.Equal(t, "openai/gpt-image-1", item.ID)

	_, err = service.Get(context.Background(), "missing")
	assert.ErrorIs(t, err, ErrUpstreamModelNotFound)
}

func TestUpstreamModelServiceReturnsAbilityErrors(t *testing.T) {
	service := NewUpstreamModelService(upstreamModelSourceStub{err: assert.AnError})

	_, err := service.List(context.Background())

	assert.ErrorIs(t, err, assert.AnError)
}
