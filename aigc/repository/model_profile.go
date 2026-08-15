package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/aigc/entity"
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var ErrVersionConflict = errors.New("AIGC model configuration version conflict")

func (repository *Repository) CreateProfile(ctx context.Context, profile *entity.ModelProfile) error {
	if repository == nil || repository.db == nil {
		return fmt.Errorf("AIGC repository is not configured")
	}
	profile.PublicModelID = strings.TrimSpace(profile.PublicModelID)
	profile.DisplayName = strings.TrimSpace(profile.DisplayName)
	profile.ModelType = strings.TrimSpace(profile.ModelType)
	if profile.PublicModelID == "" || profile.DisplayName == "" || profile.ModelType == "" {
		return fmt.Errorf("public model id, display name and model type are required")
	}
	now := common.GetTimestamp()
	profile.ConfigVersion = 1
	profile.CreatedTime = now
	profile.UpdatedTime = now
	return repository.db.WithContext(ctx).Create(profile).Error
}

func (repository *Repository) GetProfileByPublicID(ctx context.Context, publicModelID string) (*entity.ModelProfile, error) {
	if repository == nil || repository.db == nil {
		return nil, fmt.Errorf("AIGC repository is not configured")
	}
	var profile entity.ModelProfile
	err := repository.db.WithContext(ctx).
		Where("public_model_id = ?", strings.TrimSpace(publicModelID)).
		First(&profile).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func (repository *Repository) UpdateProfile(ctx context.Context, profile *entity.ModelProfile, expectedVersion int) error {
	if repository == nil || repository.db == nil {
		return fmt.Errorf("AIGC repository is not configured")
	}
	if profile.ID == 0 || expectedVersion < 1 {
		return fmt.Errorf("profile id and expected version are required")
	}
	nextVersion := expectedVersion + 1
	now := common.GetTimestamp()
	result := repository.db.WithContext(ctx).
		Model(&entity.ModelProfile{}).
		Where("id = ? AND config_version = ?", profile.ID, expectedVersion).
		Updates(map[string]any{
			"public_model_id": profile.PublicModelID,
			"display_name":    profile.DisplayName,
			"model_type":      profile.ModelType,
			"description":     profile.Description,
			"status":          profile.Status,
			"groups_json":     profile.GroupsJSON,
			"config_json":     profile.ConfigJSON,
			"config_version":  nextVersion,
			"updated_time":    now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrVersionConflict
	}
	profile.ConfigVersion = nextVersion
	profile.UpdatedTime = now
	return nil
}

func (repository *Repository) ListPublishedProfiles(ctx context.Context, modelType string) ([]entity.ModelProfile, error) {
	if repository == nil || repository.db == nil {
		return nil, fmt.Errorf("AIGC repository is not configured")
	}
	query := repository.db.WithContext(ctx).
		Where("status = ?", entity.ModelStatusPublished)
	if modelType = strings.TrimSpace(modelType); modelType != "" {
		query = query.Where("model_type = ?", modelType)
	}
	var profiles []entity.ModelProfile
	if err := query.Order("public_model_id ASC").Find(&profiles).Error; err != nil {
		return nil, err
	}
	return profiles, nil
}

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
