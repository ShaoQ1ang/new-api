package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/aigc/entity"
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm/clause"
)

func (repository *Repository) CreateOrGetRequest(ctx context.Context, request *entity.AigcRequest) (*entity.AigcRequest, bool, error) {
	if repository == nil || repository.db == nil {
		return nil, false, fmt.Errorf("AIGC repository is not configured")
	}
	if request == nil || request.UserID <= 0 || strings.TrimSpace(request.RequestID) == "" || strings.TrimSpace(request.GenerationID) == "" {
		return nil, false, fmt.Errorf("user id, request id and generation id are required")
	}
	now := common.GetTimestamp()
	if request.CreatedTime == 0 {
		request.CreatedTime = now
	}
	request.UpdatedTime = now
	if request.Status == "" {
		request.Status = entity.RequestStatusSubmitted
	}
	if strings.TrimSpace(request.RequestJSON) == "" {
		request.RequestJSON = "{}"
	}
	if strings.TrimSpace(request.ResultJSON) == "" {
		request.ResultJSON = "{}"
	}
	result := repository.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(request)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 1 {
		return request, true, nil
	}
	var existing entity.AigcRequest
	if err := repository.db.WithContext(ctx).
		Where("user_id = ? AND request_id = ?", request.UserID, request.RequestID).
		First(&existing).Error; err != nil {
		return nil, false, err
	}
	return &existing, false, nil
}

func (repository *Repository) GetRequestByGenerationID(ctx context.Context, userID int, generationID string) (*entity.AigcRequest, error) {
	if repository == nil || repository.db == nil {
		return nil, fmt.Errorf("AIGC repository is not configured")
	}
	var request entity.AigcRequest
	if err := repository.db.WithContext(ctx).
		Where("user_id = ? AND generation_id = ?", userID, strings.TrimSpace(generationID)).
		First(&request).Error; err != nil {
		return nil, err
	}
	return &request, nil
}
