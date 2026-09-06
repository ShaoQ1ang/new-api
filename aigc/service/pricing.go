package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/aigc/capability"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/shopspring/decimal"
)

type PricingSource interface {
	Pricing() []model.Pricing
}

type PricingService struct {
	profiles     ProfileStore
	availability Availability
	source       PricingSource
}

type PublicPricingDocument struct {
	PricingVersion      string               `json:"pricing_version"`
	Currency            string               `json:"currency"`
	QuotaPerUnit        string               `json:"quota_per_unit"`
	UserGroup           string               `json:"user_group"`
	EffectiveGroupRatio string               `json:"effective_group_ratio"`
	Models              []PublicModelPricing `json:"models"`
}

type PublicModelPricing struct {
	ModelID   string               `json:"model_id"`
	MediaType string               `json:"media_type"`
	Routes    []PublicPricingRoute `json:"routes"`
}

type PublicPricingRoute struct {
	Mode                        string                       `json:"mode"`
	Resolutions                 []string                     `json:"resolutions,omitempty"`
	AspectRatios                []string                     `json:"aspect_ratios,omitempty"`
	Durations                   []int                        `json:"durations,omitempty"`
	BillingUnit                 string                       `json:"billing_unit"`
	ImageResolutionPrice        map[string]string            `json:"image_resolution_price,omitempty"`
	VideoSecondsPrice           map[string]map[string]string `json:"video_seconds_price,omitempty"`
	GenerationPrice             string                       `json:"generation_price,omitempty"`
	InputPricePerMillionTokens  string                       `json:"input_price_per_million_tokens,omitempty"`
	OutputPricePerMillionTokens string                       `json:"output_price_per_million_tokens,omitempty"`
}

func NewPricingService(profiles ProfileStore, availability Availability, source PricingSource) *PricingService {
	return &PricingService{profiles: profiles, availability: availability, source: source}
}

func (service *PricingService) List(ctx context.Context, group string) (PublicPricingDocument, error) {
	if common.QuotaPerUnit <= 0 || math.IsNaN(common.QuotaPerUnit) || math.IsInf(common.QuotaPerUnit, 0) {
		return PublicPricingDocument{}, errors.New("quota per unit must be positive and finite")
	}
	profiles, err := service.profiles.ListPublishedProfiles(ctx, "")
	if err != nil {
		return PublicPricingDocument{}, err
	}
	prices := make(map[string]model.Pricing)
	for _, price := range service.source.Pricing() {
		prices[strings.TrimSpace(price.ModelName)] = price
	}
	ratio := effectivePricingGroupRatio(group)
	document := PublicPricingDocument{
		Currency: "USD", QuotaPerUnit: decimalString(common.QuotaPerUnit), UserGroup: group,
		EffectiveGroupRatio: decimalString(ratio), Models: make([]PublicModelPricing, 0, len(profiles)),
	}
	for _, profile := range profiles {
		groups, groupErr := profileGroups(profile.GroupsJSON)
		if groupErr != nil {
			return PublicPricingDocument{}, groupErr
		}
		if !groupAllowed(groups, group) {
			continue
		}
		config, parseErr := capability.Parse(capability.ModelType(profile.ModelType), []byte(profile.ConfigJSON))
		if parseErr != nil {
			continue
		}
		available, availabilityErr := service.availability.Available(ctx, group, config.UpstreamModelIDs())
		if availabilityErr != nil {
			return PublicPricingDocument{}, availabilityErr
		}
		routes := pricingRoutes(capability.ModelType(profile.ModelType), config, available, prices, ratio)
		if len(routes) == 0 {
			continue
		}
		document.Models = append(document.Models, PublicModelPricing{
			ModelID: profile.PublicModelID, MediaType: profile.ModelType, Routes: routes,
		})
	}
	sort.Slice(document.Models, func(i, j int) bool { return document.Models[i].ModelID < document.Models[j].ModelID })
	for i := range document.Models {
		sort.Slice(document.Models[i].Routes, func(a, b int) bool {
			left, right := document.Models[i].Routes[a], document.Models[i].Routes[b]
			return pricingRouteSortKey(left) < pricingRouteSortKey(right)
		})
	}
	encoded, err := common.Marshal(document)
	if err != nil {
		return PublicPricingDocument{}, fmt.Errorf("encode AIGC pricing version: %w", err)
	}
	digest := sha256.Sum256(encoded)
	document.PricingVersion = "sha256:" + hex.EncodeToString(digest[:])
	return document, nil
}

func pricingRouteSortKey(route PublicPricingRoute) string {
	return route.Mode + "|" + strings.Join(route.Resolutions, ",") + "|" + strings.Join(route.AspectRatios, ",") + "|" + fmt.Sprint(route.Durations)
}

func pricingRoutes(modelType capability.ModelType, config capability.Config, available map[string]bool, prices map[string]model.Pricing, ratio float64) []PublicPricingRoute {
	result := make([]PublicPricingRoute, 0)
	appendRoute := func(mode, upstreamID string, resolutions, aspectRatios []string, durations []int) {
		upstreamID = strings.TrimSpace(upstreamID)
		if upstreamID == "" || !available[upstreamID] {
			return
		}
		price, configured := prices[upstreamID]
		route := projectPricingRoute(string(modelType), mode, resolutions, price, configured, ratio)
		route.AspectRatios = sortedUnique(aspectRatios)
		route.Durations = sortedUniqueInts(durations)
		result = append(result, route)
	}
	switch modelType {
	case capability.ModelTypeText:
		appendRoute("text", config.Text.UpstreamModelID, nil, nil, nil)
	case capability.ModelTypeImage:
		for mode, item := range config.Image.Modes {
			appendRoute(mode, item.UpstreamModelID, item.Output.Sizes, nil, nil)
		}
	case capability.ModelTypeVideo:
		for _, output := range config.Video.OutputSpecs {
			for _, mode := range output.Modes {
				configured, exists := config.Video.Modes[mode]
				if !exists || !available[strings.TrimSpace(configured.UpstreamModelID)] {
					continue
				}
				upstreamID := configured.UpstreamModelID
				if output.Target != nil && strings.TrimSpace(output.Target.UpstreamModelID) != "" {
					upstreamID = output.Target.UpstreamModelID
				}
				appendRoute(mode, upstreamID, output.Resolutions, output.AspectRatios, output.Durations)
			}
		}
	case capability.ModelTypeMusic:
		for mode, item := range config.Music.Modes {
			appendRoute(mode, item.UpstreamModelID, nil, nil, nil)
		}
	}
	return result
}

func projectPricingRoute(modelType, mode string, resolutions []string, price model.Pricing, configured bool, ratio float64) PublicPricingRoute {
	route := PublicPricingRoute{Mode: mode, Resolutions: sortedUnique(resolutions), BillingUnit: "unconfigured"}
	if !configured {
		return route
	}
	if modelType == string(capability.ModelTypeImage) && len(price.ImageResolutionPrice) > 0 {
		allowed := make(map[string]bool)
		for _, size := range resolutions {
			if tier, ok := ratio_setting.ResolveImageResolutionTier(size); ok {
				allowed[tier] = true
			}
		}
		route.ImageResolutionPrice = scaledPriceMap(price.ImageResolutionPrice, allowed, ratio)
		if len(route.ImageResolutionPrice) > 0 {
			route.BillingUnit = "image"
			return route
		}
	}
	if modelType == string(capability.ModelTypeVideo) && len(price.VideoSecondsPrice) > 0 {
		allowed := make(map[string]bool, len(resolutions))
		for _, resolution := range resolutions {
			allowed[strings.ToLower(strings.TrimSpace(resolution))] = true
		}
		route.VideoSecondsPrice = scaledNestedPriceMap(price.VideoSecondsPrice, allowed, ratio)
		if len(route.VideoSecondsPrice) > 0 {
			route.BillingUnit = "second"
			return route
		}
	}
	if price.QuotaType == 1 {
		route.BillingUnit = "generation"
		route.GenerationPrice = scaledDecimalString(price.ModelPrice, ratio)
		return route
	}
	if modelType == string(capability.ModelTypeText) {
		route.BillingUnit = "token"
		inputPrice := decimal.NewFromFloat(price.ModelRatio).
			Mul(decimal.NewFromInt(1_000_000)).
			Div(decimal.NewFromFloat(common.QuotaPerUnit)).
			Mul(decimal.NewFromFloat(ratio))
		route.InputPricePerMillionTokens = inputPrice.StringFixed(6)
		route.OutputPricePerMillionTokens = inputPrice.Mul(decimal.NewFromFloat(price.CompletionRatio)).StringFixed(6)
		return route
	}
	route.BillingUnit = "dynamic"
	return route
}

func effectivePricingGroupRatio(group string) float64 {
	if value, ok := ratio_setting.GetGroupGroupRatio(group, group); ok {
		return value
	}
	return ratio_setting.GetGroupRatio(group)
}

func scaledPriceMap(source map[string]float64, allowed map[string]bool, ratio float64) map[string]string {
	result := make(map[string]string)
	for key, value := range source {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if len(allowed) > 0 && !allowed[normalized] {
			continue
		}
		result[normalized] = scaledDecimalString(value, ratio)
	}
	return result
}

func scaledNestedPriceMap(source map[string]map[string]float64, allowed map[string]bool, ratio float64) map[string]map[string]string {
	result := make(map[string]map[string]string)
	for key, variants := range source {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if len(allowed) > 0 && !allowed[normalized] {
			continue
		}
		converted := make(map[string]string, len(variants))
		for variant, value := range variants {
			converted[strings.ToLower(strings.TrimSpace(variant))] = scaledDecimalString(value, ratio)
		}
		if len(converted) > 0 {
			result[normalized] = converted
		}
	}
	return result
}

func sortedUnique(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func sortedUniqueInts(values []int) []int {
	seen := make(map[int]bool, len(values))
	result := make([]int, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Ints(result)
	return result
}

func scaledDecimalString(value, ratio float64) string {
	return decimal.NewFromFloat(value).Mul(decimal.NewFromFloat(ratio)).StringFixed(6)
}

func decimalString(value float64) string {
	return decimal.NewFromFloat(value).StringFixed(6)
}
