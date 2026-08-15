package service

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

var ErrUpstreamModelNotFound = errors.New("AIGC upstream model not found")

type UpstreamModelSource interface {
	Pricing() []model.Pricing
	Abilities() ([]model.AbilityWithChannel, error)
}

type ModelUpstreamSource struct{}

func NewModelUpstreamSource() ModelUpstreamSource {
	return ModelUpstreamSource{}
}

func (ModelUpstreamSource) Pricing() []model.Pricing {
	return model.GetPricing()
}

func (ModelUpstreamSource) Abilities() ([]model.AbilityWithChannel, error) {
	return model.GetAllEnableAbilityWithChannels()
}

type UpstreamPricing struct {
	Available            bool                          `json:"available"`
	QuotaType            int                           `json:"quota_type,omitempty"`
	ModelRatio           float64                       `json:"model_ratio,omitempty"`
	ModelPrice           float64                       `json:"model_price,omitempty"`
	CompletionRatio      float64                       `json:"completion_ratio,omitempty"`
	ImageResolutionPrice map[string]float64            `json:"image_resolution_price,omitempty"`
	TaskConditionPrice   map[string]map[string]float64 `json:"task_condition_price,omitempty"`
	VideoSecondsPrice    map[string]map[string]float64 `json:"video_seconds_price,omitempty"`
	BillingMode          string                        `json:"billing_mode,omitempty"`
}

type UpstreamModel struct {
	ID                     string                  `json:"id"`
	Description            string                  `json:"description,omitempty"`
	Icon                   string                  `json:"icon,omitempty"`
	Tags                   string                  `json:"tags,omitempty"`
	VendorID               int                     `json:"vendor_id,omitempty"`
	Groups                 []string                `json:"groups"`
	SupportedEndpointTypes []constant.EndpointType `json:"supported_endpoint_types"`
	ChannelCount           int                     `json:"channel_count"`
	Pricing                UpstreamPricing         `json:"pricing"`
}

type UpstreamModelService struct {
	source UpstreamModelSource
}

func NewUpstreamModelService(source UpstreamModelSource) *UpstreamModelService {
	return &UpstreamModelService{source: source}
}

func (service *UpstreamModelService) List(_ context.Context) ([]UpstreamModel, error) {
	byID := make(map[string]*upstreamModelBuilder)
	for _, pricing := range service.source.Pricing() {
		id := strings.TrimSpace(pricing.ModelName)
		if id == "" {
			continue
		}
		builder := upstreamBuilder(byID, id)
		builder.item.Description = pricing.Description
		builder.item.Icon = pricing.Icon
		builder.item.Tags = pricing.Tags
		builder.item.VendorID = pricing.VendorID
		builder.item.Pricing = pricingSummary(pricing)
		for _, group := range pricing.EnableGroup {
			builder.addGroup(group)
		}
		for _, endpoint := range pricing.SupportedEndpointTypes {
			builder.addEndpoint(endpoint)
		}
	}

	abilities, err := service.source.Abilities()
	if err != nil {
		return nil, err
	}
	for _, ability := range abilities {
		id := strings.TrimSpace(ability.Model)
		if id == "" || !ability.Enabled {
			continue
		}
		builder := upstreamBuilder(byID, id)
		builder.addGroup(ability.Group)
		builder.channels[ability.ChannelId] = struct{}{}
	}

	items := make([]UpstreamModel, 0, len(byID))
	for _, builder := range byID {
		builder.finish()
		items = append(items, builder.item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func (service *UpstreamModelService) Get(ctx context.Context, id string) (*UpstreamModel, error) {
	id = strings.TrimSpace(id)
	items, err := service.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ID == id {
			return &items[i], nil
		}
	}
	return nil, ErrUpstreamModelNotFound
}

type upstreamModelBuilder struct {
	item      UpstreamModel
	groups    map[string]struct{}
	endpoints map[constant.EndpointType]struct{}
	channels  map[int]struct{}
}

func upstreamBuilder(builders map[string]*upstreamModelBuilder, id string) *upstreamModelBuilder {
	if builder := builders[id]; builder != nil {
		return builder
	}
	builder := &upstreamModelBuilder{
		item:      UpstreamModel{ID: id},
		groups:    make(map[string]struct{}),
		endpoints: make(map[constant.EndpointType]struct{}),
		channels:  make(map[int]struct{}),
	}
	builders[id] = builder
	return builder
}

func (builder *upstreamModelBuilder) addGroup(group string) {
	if group = strings.TrimSpace(group); group != "" {
		builder.groups[group] = struct{}{}
	}
}

func (builder *upstreamModelBuilder) addEndpoint(endpoint constant.EndpointType) {
	if endpoint != "" {
		builder.endpoints[endpoint] = struct{}{}
	}
}

func (builder *upstreamModelBuilder) finish() {
	builder.item.Groups = make([]string, 0, len(builder.groups))
	for group := range builder.groups {
		builder.item.Groups = append(builder.item.Groups, group)
	}
	sort.Strings(builder.item.Groups)
	builder.item.SupportedEndpointTypes = make([]constant.EndpointType, 0, len(builder.endpoints))
	for endpoint := range builder.endpoints {
		builder.item.SupportedEndpointTypes = append(builder.item.SupportedEndpointTypes, endpoint)
	}
	sort.Slice(builder.item.SupportedEndpointTypes, func(i, j int) bool {
		return builder.item.SupportedEndpointTypes[i] < builder.item.SupportedEndpointTypes[j]
	})
	builder.item.ChannelCount = len(builder.channels)
}

func pricingSummary(pricing model.Pricing) UpstreamPricing {
	return UpstreamPricing{
		Available: true, QuotaType: pricing.QuotaType, ModelRatio: pricing.ModelRatio,
		ModelPrice: pricing.ModelPrice, CompletionRatio: pricing.CompletionRatio,
		ImageResolutionPrice: pricing.ImageResolutionPrice, TaskConditionPrice: pricing.TaskConditionPrice,
		VideoSecondsPrice: pricing.VideoSecondsPrice, BillingMode: pricing.BillingMode,
	}
}
