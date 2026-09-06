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
	quotaPerUnit := common.QuotaPerUnit
	if quotaPerUnit <= 0 || math.IsNaN(quotaPerUnit) || math.IsInf(quotaPerUnit, 0) {
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
	if !validPricingNumber(ratio) {
		return PublicPricingDocument{}, errors.New("effective group ratio must be non-negative and finite")
	}
	document := PublicPricingDocument{
		Currency: "USD", QuotaPerUnit: decimalString(quotaPerUnit), UserGroup: group,
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
		routes, routeErr := pricingRoutes(capability.ModelType(profile.ModelType), config, available, prices, ratio, quotaPerUnit)
		if routeErr != nil {
			return PublicPricingDocument{}, fmt.Errorf("project pricing for model %s: %w", profile.PublicModelID, routeErr)
		}
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

func pricingRoutes(modelType capability.ModelType, config capability.Config, available map[string]bool, prices map[string]model.Pricing, ratio, quotaPerUnit float64) ([]PublicPricingRoute, error) {
	result := make([]PublicPricingRoute, 0)
	appendRoute := func(mode, upstreamID string, resolutions, aspectRatios []string, durations []int) error {
		upstreamID = strings.TrimSpace(upstreamID)
		if upstreamID == "" || !available[upstreamID] {
			return nil
		}
		price, configured := prices[upstreamID]
		route, err := projectPricingRoute(string(modelType), mode, resolutions, price, configured, ratio, quotaPerUnit)
		if err != nil {
			return fmt.Errorf("route %s: %w", mode, err)
		}
		route.AspectRatios = sortedUnique(aspectRatios)
		route.Durations = sortedUniqueInts(durations)
		result = append(result, route)
		return nil
	}
	switch modelType {
	case capability.ModelTypeText:
		if err := appendRoute("text", config.Text.UpstreamModelID, nil, nil, nil); err != nil {
			return nil, err
		}
	case capability.ModelTypeImage:
		for mode, item := range config.Image.Modes {
			if err := appendRoute(mode, item.UpstreamModelID, item.Output.Sizes, nil, nil); err != nil {
				return nil, err
			}
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
				if err := appendRoute(mode, upstreamID, output.Resolutions, output.AspectRatios, output.Durations); err != nil {
					return nil, err
				}
			}
		}
	case capability.ModelTypeMusic:
		for mode, item := range config.Music.Modes {
			if err := appendRoute(mode, item.UpstreamModelID, nil, nil, nil); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

func projectPricingRoute(modelType, mode string, resolutions []string, price model.Pricing, configured bool, ratio, quotaPerUnit float64) (PublicPricingRoute, error) {
	route := PublicPricingRoute{Mode: mode, Resolutions: sortedUnique(resolutions), BillingUnit: "unconfigured"}
	if !configured {
		return route, nil
	}
	if !validPricingNumber(ratio) {
		return PublicPricingRoute{}, errors.New("effective group ratio must be non-negative and finite")
	}
	if modelType == string(capability.ModelTypeImage) && len(price.ImageResolutionPrice) > 0 {
		allowed := make(map[string]bool)
		for _, size := range resolutions {
			if tier, ok := ratio_setting.ResolveImageResolutionTier(size); ok {
				allowed[tier] = true
			}
		}
		var err error
		route.ImageResolutionPrice, err = scaledPriceMap(price.ImageResolutionPrice, allowed, ratio)
		if err != nil {
			return PublicPricingRoute{}, err
		}
		if len(route.ImageResolutionPrice) > 0 {
			route.BillingUnit = "image"
			return route, nil
		}
	}
	if modelType == string(capability.ModelTypeVideo) && len(price.VideoSecondsPrice) > 0 {
		allowed := make(map[string]bool, len(resolutions))
		for _, resolution := range resolutions {
			allowed[strings.ToLower(strings.TrimSpace(resolution))] = true
		}
		var err error
		route.VideoSecondsPrice, err = scaledNestedPriceMap(price.VideoSecondsPrice, allowed, ratio)
		if err != nil {
			return PublicPricingRoute{}, err
		}
		if len(route.VideoSecondsPrice) > 0 {
			route.BillingUnit = "second"
			return route, nil
		}
	}
	if price.QuotaType == 1 {
		if !validPricingNumber(price.ModelPrice) {
			return PublicPricingRoute{}, errors.New("generation price must be non-negative and finite")
		}
		route.BillingUnit = "generation"
		route.GenerationPrice = scaledDecimalString(price.ModelPrice, ratio)
		return route, nil
	}
	if modelType == string(capability.ModelTypeText) {
		if !validPricingNumber(price.ModelRatio) || !validPricingNumber(price.CompletionRatio) {
			return PublicPricingRoute{}, errors.New("token price ratios must be non-negative and finite")
		}
		if quotaPerUnit <= 0 || math.IsNaN(quotaPerUnit) || math.IsInf(quotaPerUnit, 0) {
			return PublicPricingRoute{}, errors.New("quota per unit must be positive and finite")
		}
		route.BillingUnit = "token"
		inputPrice := decimal.NewFromFloat(price.ModelRatio).
			Mul(decimal.NewFromInt(1_000_000)).
			Div(decimal.NewFromFloat(quotaPerUnit)).
			Mul(decimal.NewFromFloat(ratio))
		route.InputPricePerMillionTokens = inputPrice.StringFixed(6)
		route.OutputPricePerMillionTokens = inputPrice.Mul(decimal.NewFromFloat(price.CompletionRatio)).StringFixed(6)
		return route, nil
	}
	route.BillingUnit = "dynamic"
	return route, nil
}

func effectivePricingGroupRatio(group string) float64 {
	if value, ok := ratio_setting.GetGroupGroupRatio(group, group); ok {
		return value
	}
	return ratio_setting.GetGroupRatio(group)
}

func scaledPriceMap(source map[string]float64, allowed map[string]bool, ratio float64) (map[string]string, error) {
	result := make(map[string]string)
	for key, value := range source {
		if !validPricingNumber(value) {
			return nil, fmt.Errorf("image resolution price %q must be non-negative and finite", key)
		}
		normalized := strings.ToLower(strings.TrimSpace(key))
		if len(allowed) > 0 && !allowed[normalized] {
			continue
		}
		result[normalized] = scaledDecimalString(value, ratio)
	}
	return result, nil
}

func scaledNestedPriceMap(source map[string]map[string]float64, allowed map[string]bool, ratio float64) (map[string]map[string]string, error) {
	result := make(map[string]map[string]string)
	for key, variants := range source {
		converted := make(map[string]string, len(variants))
		for variant, value := range variants {
			if !validPricingNumber(value) {
				return nil, fmt.Errorf("video seconds price %q/%q must be non-negative and finite", key, variant)
			}
			converted[strings.ToLower(strings.TrimSpace(variant))] = scaledDecimalString(value, ratio)
		}
		normalized := strings.ToLower(strings.TrimSpace(key))
		if len(allowed) > 0 && !allowed[normalized] {
			continue
		}
		if len(converted) > 0 {
			result[normalized] = converted
		}
	}
	return result, nil
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

func validPricingNumber(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
