package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/aigc/capability"
	"github.com/QuantumNous/new-api/aigc/entity"
	"github.com/QuantumNous/new-api/common"
)

var ErrUpstreamUnavailable = errors.New("no AIGC upstream model is available")

type ProfileStore interface {
	GetProfileByPublicID(ctx context.Context, publicModelID string) (*entity.ModelProfile, error)
	UpdateProfile(ctx context.Context, profile *entity.ModelProfile, expectedVersion int) error
	ListPublishedProfiles(ctx context.Context, modelType string) ([]entity.ModelProfile, error)
}

type Availability interface {
	Available(ctx context.Context, group string, upstreamModelIDs []string) (map[string]bool, error)
}

type CatalogService struct {
	profiles     ProfileStore
	availability Availability
}

type PublicModel struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Type          string          `json:"type"`
	Description   string          `json:"description,omitempty"`
	ConfigVersion int             `json:"config_version"`
	Capabilities  json.RawMessage `json:"capabilities"`
}

func NewCatalogService(profiles ProfileStore, availability Availability) *CatalogService {
	return &CatalogService{profiles: profiles, availability: availability}
}

func (service *CatalogService) Publish(ctx context.Context, publicModelID string, expectedVersion int) error {
	profile, err := service.profiles.GetProfileByPublicID(ctx, publicModelID)
	if err != nil {
		return err
	}
	config, err := capability.Parse(capability.ModelType(profile.ModelType), []byte(profile.ConfigJSON))
	if err != nil {
		return err
	}
	groups, err := profileGroups(profile.GroupsJSON)
	if err != nil {
		return err
	}
	if len(groups) == 0 {
		groups = []string{""}
	}
	upstreamIDs := config.UpstreamModelIDs()
	available := false
	for _, group := range groups {
		byModel, availabilityErr := service.availability.Available(ctx, group, upstreamIDs)
		if availabilityErr != nil {
			return availabilityErr
		}
		if anyAvailable(byModel) {
			available = true
			break
		}
	}
	if !available {
		return ErrUpstreamUnavailable
	}
	profile.Status = entity.ModelStatusPublished
	return service.profiles.UpdateProfile(ctx, profile, expectedVersion)
}

func (service *CatalogService) List(ctx context.Context, group, modelType string) ([]PublicModel, error) {
	profiles, err := service.profiles.ListPublishedProfiles(ctx, strings.TrimSpace(modelType))
	if err != nil {
		return nil, err
	}
	items := make([]PublicModel, 0, len(profiles))
	for _, profile := range profiles {
		groups, groupErr := profileGroups(profile.GroupsJSON)
		if groupErr != nil {
			return nil, groupErr
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
			return nil, availabilityErr
		}
		publicCapabilities, visible, publicErr := config.Public(capability.ModelType(profile.ModelType), available)
		if publicErr != nil {
			return nil, publicErr
		}
		if !visible {
			continue
		}
		items = append(items, PublicModel{
			ID: profile.PublicModelID, Name: profile.DisplayName, Type: profile.ModelType,
			Description: profile.Description, ConfigVersion: profile.ConfigVersion,
			Capabilities: json.RawMessage(publicCapabilities),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func profileGroups(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return []string{}, nil
	}
	var groups []string
	if err := common.Unmarshal([]byte(raw), &groups); err != nil {
		return nil, fmt.Errorf("invalid AIGC model groups: %w", err)
	}
	seen := make(map[string]bool)
	result := make([]string, 0, len(groups))
	for _, group := range groups {
		if group = strings.TrimSpace(group); group != "" && !seen[group] {
			seen[group] = true
			result = append(result, group)
		}
	}
	sort.Strings(result)
	return result, nil
}

func groupAllowed(groups []string, target string) bool {
	if len(groups) == 0 {
		return true
	}
	for _, group := range groups {
		if group == target {
			return true
		}
	}
	return false
}

func anyAvailable(available map[string]bool) bool {
	for _, value := range available {
		if value {
			return true
		}
	}
	return false
}
