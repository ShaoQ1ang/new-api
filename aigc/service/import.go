package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/aigc/capability"
	"github.com/QuantumNous/new-api/aigc/entity"
	"github.com/QuantumNous/new-api/common"
)

type ProfileImportStore interface {
	FindProfileByPublicID(ctx context.Context, publicModelID string) (*entity.ModelProfile, bool, error)
	CreateProfile(ctx context.Context, profile *entity.ModelProfile) error
}

type ImportUpstreamCatalog interface {
	List(ctx context.Context) ([]UpstreamModel, error)
}

type LegacyProfileInput struct {
	ModelID     string          `json:"model_id"`
	DisplayName string          `json:"display_name"`
	ModelType   string          `json:"model_type"`
	Description string          `json:"description,omitempty"`
	Groups      []string        `json:"groups,omitempty"`
	Config      json.RawMessage `json:"config,omitempty"`
	ConfigJSON  string          `json:"config_json,omitempty"`
	Enabled     bool            `json:"enabled"`
}

type ProfileImportIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ProfileImportItem struct {
	ModelID string               `json:"model_id"`
	Action  string               `json:"action"`
	Status  string               `json:"status,omitempty"`
	Issues  []ProfileImportIssue `json:"issues"`
}

type ProfileImportReport struct {
	Total     int                 `json:"total"`
	Imported  int                 `json:"imported"`
	Published int                 `json:"published"`
	Drafts    int                 `json:"drafts"`
	Skipped   int                 `json:"skipped"`
	Failed    int                 `json:"failed"`
	Items     []ProfileImportItem `json:"items"`
}

type ProfileImportService struct {
	profiles  ProfileImportStore
	upstreams ImportUpstreamCatalog
}

func NewProfileImportService(profiles ProfileImportStore, upstreams ImportUpstreamCatalog) *ProfileImportService {
	return &ProfileImportService{profiles: profiles, upstreams: upstreams}
}

func (service *ProfileImportService) Import(ctx context.Context, inputs []LegacyProfileInput) (*ProfileImportReport, error) {
	upstreamItems, err := service.upstreams.List(ctx)
	if err != nil {
		return nil, err
	}
	upstreams := make(map[string]UpstreamModel, len(upstreamItems))
	for _, item := range upstreamItems {
		upstreams[item.ID] = item
	}

	report := &ProfileImportReport{Total: len(inputs), Items: make([]ProfileImportItem, 0, len(inputs))}
	for _, input := range inputs {
		result, importErr := service.importOne(ctx, input, upstreams)
		if importErr != nil {
			return nil, importErr
		}
		report.Items = append(report.Items, result)
		switch result.Action {
		case "imported":
			report.Imported++
			if result.Status == "published" {
				report.Published++
			} else {
				report.Drafts++
			}
		case "skipped":
			report.Skipped++
		case "failed":
			report.Failed++
		}
	}
	return report, nil
}

func (service *ProfileImportService) importOne(ctx context.Context, input LegacyProfileInput, upstreams map[string]UpstreamModel) (ProfileImportItem, error) {
	input.ModelID = strings.TrimSpace(input.ModelID)
	result := ProfileImportItem{ModelID: input.ModelID, Issues: make([]ProfileImportIssue, 0)}
	if input.ModelID == "" {
		result.Action = "failed"
		result.Issues = append(result.Issues, importIssue("INVALID_PROFILE", "model_id is required"))
		return result, nil
	}
	modelType := capability.ModelType(strings.TrimSpace(input.ModelType))
	if !supportedImportModelType(modelType) {
		result.Action = "failed"
		result.Issues = append(result.Issues, importIssue("INVALID_PROFILE", fmt.Sprintf("unsupported AIGC model type %q", input.ModelType)))
		return result, nil
	}
	_, found, err := service.profiles.FindProfileByPublicID(ctx, input.ModelID)
	if err != nil {
		return result, err
	}
	if found {
		result.Action = "skipped"
		result.Issues = append(result.Issues, importIssue("ALREADY_EXISTS", "profile already exists"))
		return result, nil
	}

	raw, description, err := normalizeLegacyConfig(input)
	if err != nil {
		result.Action = "failed"
		result.Issues = append(result.Issues, importIssue("INVALID_JSON", err.Error()))
		return result, nil
	}
	parsed, parseErr := capability.Parse(modelType, raw)
	if parseErr != nil {
		result.Issues = append(result.Issues, importIssue("INVALID_CONFIG", parseErr.Error()))
	} else {
		canonical, marshalErr := common.Marshal(parsed)
		if marshalErr != nil {
			return result, marshalErr
		}
		raw = canonical
		groups := normalizeGroups(input.Groups)
		for _, upstreamID := range parsed.UpstreamModelIDs() {
			upstream, exists := upstreams[upstreamID]
			if !exists || upstream.ChannelCount < 1 {
				result.Issues = append(result.Issues, importIssue("UPSTREAM_MODEL_NOT_FOUND", fmt.Sprintf("upstream model %s has no enabled channel", upstreamID)))
				continue
			}
			if len(groups) > 0 && !groupsIntersect(groups, upstream.Groups) {
				result.Issues = append(result.Issues, importIssue("UPSTREAM_MODEL_NOT_AVAILABLE_FOR_GROUP", fmt.Sprintf("upstream model %s is not available for imported groups", upstreamID)))
			}
			if !upstream.Pricing.Available {
				result.Issues = append(result.Issues, importIssue("PRICING_MISSING", fmt.Sprintf("upstream model %s has no pricing", upstreamID)))
			}
		}
	}

	status := entity.ModelStatusDraft
	if input.Enabled && len(result.Issues) == 0 {
		status = entity.ModelStatusPublished
	}
	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		displayName = input.ModelID
	}
	if input.Description = strings.TrimSpace(input.Description); input.Description == "" {
		input.Description = description
	}
	groupsJSON, err := common.Marshal(normalizeGroups(input.Groups))
	if err != nil {
		return result, err
	}
	profile := &entity.ModelProfile{
		PublicModelID: input.ModelID, DisplayName: displayName, ModelType: strings.TrimSpace(input.ModelType),
		Description: input.Description, Status: status, GroupsJSON: string(groupsJSON), ConfigJSON: string(raw),
	}
	if err := service.profiles.CreateProfile(ctx, profile); err != nil {
		return result, err
	}
	result.Action = "imported"
	if status == entity.ModelStatusPublished {
		result.Status = "published"
	} else {
		result.Status = "draft"
	}
	return result, nil
}

func normalizeLegacyConfig(input LegacyProfileInput) ([]byte, string, error) {
	raw := []byte(input.Config)
	if len(raw) == 0 {
		raw = []byte(strings.TrimSpace(input.ConfigJSON))
	}
	var envelope map[string]any
	if len(raw) == 0 || common.Unmarshal(raw, &envelope) != nil || envelope == nil {
		return nil, "", fmt.Errorf("configuration must be a JSON object")
	}
	description, _ := envelope["description"].(string)
	if strings.TrimSpace(input.ModelType) == string(capability.ModelTypeText) {
		if _, wrapped := envelope["text"]; !wrapped {
			textConfig := make(map[string]any)
			for _, key := range []string{"upstream_model_id", "max_output_tokens"} {
				if value, ok := envelope[key]; ok {
					textConfig[key] = value
				}
			}
			envelope = map[string]any{"text": textConfig}
		}
	}
	normalized, err := common.Marshal(envelope)
	if err != nil {
		return nil, "", err
	}
	return normalized, strings.TrimSpace(description), nil
}

func importIssue(code, message string) ProfileImportIssue {
	return ProfileImportIssue{Code: code, Message: message}
}

func supportedImportModelType(modelType capability.ModelType) bool {
	switch modelType {
	case capability.ModelTypeText, capability.ModelTypeImage, capability.ModelTypeVideo, capability.ModelTypeMusic:
		return true
	default:
		return false
	}
}

func groupsIntersect(profileGroups, upstreamGroups []string) bool {
	available := make(map[string]struct{}, len(upstreamGroups))
	for _, group := range upstreamGroups {
		available[group] = struct{}{}
	}
	if _, all := available["all"]; all {
		return true
	}
	for _, group := range profileGroups {
		if _, ok := available[group]; ok {
			return true
		}
	}
	return false
}
