package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/aigc/capability"
	"github.com/QuantumNous/new-api/aigc/entity"
	"github.com/QuantumNous/new-api/common"
)

type AdminProfileStore interface {
	CreateProfile(ctx context.Context, profile *entity.ModelProfile) error
	GetProfileByID(ctx context.Context, id int64) (*entity.ModelProfile, error)
	UpdateProfile(ctx context.Context, profile *entity.ModelProfile, expectedVersion int) error
	SearchProfiles(ctx context.Context, modelType string, status *int, offset, limit int) ([]entity.ModelProfile, int64, error)
}

type AdminService struct {
	profiles AdminProfileStore
}

type ProfileInput struct {
	PublicModelID string          `json:"public_model_id"`
	DisplayName   string          `json:"display_name"`
	ModelType     string          `json:"model_type"`
	Description   string          `json:"description,omitempty"`
	Groups        []string        `json:"groups"`
	Config        json.RawMessage `json:"config"`
	ConfigVersion int             `json:"config_version,omitempty"`
}

type AdminProfile struct {
	ID            int64           `json:"id"`
	PublicModelID string          `json:"public_model_id"`
	DisplayName   string          `json:"display_name"`
	ModelType     string          `json:"model_type"`
	Description   string          `json:"description,omitempty"`
	Status        int             `json:"status"`
	Groups        []string        `json:"groups"`
	Config        json.RawMessage `json:"config"`
	ConfigVersion int             `json:"config_version"`
	CreatedTime   int64           `json:"created_time"`
	UpdatedTime   int64           `json:"updated_time"`
}

type ProfileFilter struct {
	ModelType string
	Status    *int
	Page      int
	PageSize  int
}

func NewAdminService(profiles AdminProfileStore) *AdminService {
	return &AdminService{profiles: profiles}
}

func (service *AdminService) Create(ctx context.Context, input ProfileInput) (*AdminProfile, error) {
	profile, err := profileFromInput(input)
	if err != nil {
		return nil, err
	}
	profile.Status = entity.ModelStatusDraft
	if err := service.profiles.CreateProfile(ctx, profile); err != nil {
		return nil, err
	}
	return adminProfileFromEntity(*profile)
}

func (service *AdminService) Get(ctx context.Context, id int64) (*AdminProfile, error) {
	profile, err := service.profiles.GetProfileByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return adminProfileFromEntity(*profile)
}

func (service *AdminService) Update(ctx context.Context, id int64, input ProfileInput) (*AdminProfile, error) {
	stored, err := service.profiles.GetProfileByID(ctx, id)
	if err != nil {
		return nil, err
	}
	updated, err := profileFromInput(input)
	if err != nil {
		return nil, err
	}
	if updated.PublicModelID != stored.PublicModelID {
		return nil, fmt.Errorf("public model id cannot be changed")
	}
	updated.ID = stored.ID
	updated.Status = stored.Status
	updated.CreatedTime = stored.CreatedTime
	updated.ConfigVersion = stored.ConfigVersion
	if err := service.profiles.UpdateProfile(ctx, updated, input.ConfigVersion); err != nil {
		return nil, err
	}
	return adminProfileFromEntity(*updated)
}

func (service *AdminService) Validate(ctx context.Context, id int64) error {
	profile, err := service.profiles.GetProfileByID(ctx, id)
	if err != nil {
		return err
	}
	_, err = capability.Parse(capability.ModelType(profile.ModelType), []byte(profile.ConfigJSON))
	return err
}

func (service *AdminService) Disable(ctx context.Context, id int64, expectedVersion int) error {
	profile, err := service.profiles.GetProfileByID(ctx, id)
	if err != nil {
		return err
	}
	profile.Status = entity.ModelStatusDisabled
	return service.profiles.UpdateProfile(ctx, profile, expectedVersion)
}

func (service *AdminService) List(ctx context.Context, filter ProfileFilter) ([]AdminProfile, int64, error) {
	page, pageSize := filter.Page, filter.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	profiles, total, err := service.profiles.SearchProfiles(ctx, filter.ModelType, filter.Status, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, err
	}
	items := make([]AdminProfile, 0, len(profiles))
	for _, profile := range profiles {
		item, convertErr := adminProfileFromEntity(profile)
		if convertErr != nil {
			return nil, 0, convertErr
		}
		items = append(items, *item)
	}
	return items, total, nil
}

func profileFromInput(input ProfileInput) (*entity.ModelProfile, error) {
	input.PublicModelID = strings.TrimSpace(input.PublicModelID)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.ModelType = strings.TrimSpace(input.ModelType)
	if input.PublicModelID == "" || input.DisplayName == "" {
		return nil, fmt.Errorf("public model id and display name are required")
	}
	switch capability.ModelType(input.ModelType) {
	case capability.ModelTypeText, capability.ModelTypeImage, capability.ModelTypeVideo, capability.ModelTypeMusic:
	default:
		return nil, fmt.Errorf("unsupported AIGC model type %q", input.ModelType)
	}
	var envelope map[string]any
	if len(input.Config) == 0 || common.Unmarshal(input.Config, &envelope) != nil || envelope == nil {
		return nil, fmt.Errorf("configuration must be a JSON object")
	}
	groups := normalizeGroups(input.Groups)
	groupsJSON, err := common.Marshal(groups)
	if err != nil {
		return nil, err
	}
	configJSON, err := common.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	return &entity.ModelProfile{
		PublicModelID: input.PublicModelID,
		DisplayName:   input.DisplayName,
		ModelType:     input.ModelType,
		Description:   strings.TrimSpace(input.Description),
		GroupsJSON:    string(groupsJSON),
		ConfigJSON:    string(configJSON),
	}, nil
}

func adminProfileFromEntity(profile entity.ModelProfile) (*AdminProfile, error) {
	groups, err := profileGroups(profile.GroupsJSON)
	if err != nil {
		return nil, err
	}
	var config map[string]any
	if err := common.Unmarshal([]byte(profile.ConfigJSON), &config); err != nil {
		return nil, fmt.Errorf("invalid stored AIGC configuration: %w", err)
	}
	configJSON, err := common.Marshal(config)
	if err != nil {
		return nil, err
	}
	return &AdminProfile{
		ID: profile.ID, PublicModelID: profile.PublicModelID, DisplayName: profile.DisplayName,
		ModelType: profile.ModelType, Description: profile.Description, Status: profile.Status,
		Groups: groups, Config: json.RawMessage(configJSON), ConfigVersion: profile.ConfigVersion,
		CreatedTime: profile.CreatedTime, UpdatedTime: profile.UpdatedTime,
	}, nil
}

func normalizeGroups(groups []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(groups))
	for _, group := range groups {
		if group = strings.TrimSpace(group); group != "" && !seen[group] {
			seen[group] = true
			result = append(result, group)
		}
	}
	sort.Strings(result)
	return result
}
