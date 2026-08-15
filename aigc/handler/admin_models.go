package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/aigc/entity"
	"github.com/QuantumNous/new-api/aigc/service"
	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

type AdminCatalog interface {
	Create(ctx context.Context, input service.ProfileInput) (*service.AdminProfile, error)
	Get(ctx context.Context, id int64) (*service.AdminProfile, error)
	Update(ctx context.Context, id int64, input service.ProfileInput) (*service.AdminProfile, error)
	Validate(ctx context.Context, id int64) error
	Disable(ctx context.Context, id int64, expectedVersion int) error
	Delete(ctx context.Context, id int64, expectedVersion int) error
	List(ctx context.Context, filter service.ProfileFilter) ([]service.AdminProfile, int64, error)
}

type Publisher interface {
	Publish(ctx context.Context, publicModelID string, expectedVersion int) error
}

type AdminModelHandler struct {
	admin     AdminCatalog
	publisher Publisher
}

func NewAdminModelHandler(admin AdminCatalog, publisher Publisher) *AdminModelHandler {
	return &AdminModelHandler{admin: admin, publisher: publisher}
}

func (handler *AdminModelHandler) List(c *gin.Context) {
	filter := service.ProfileFilter{ModelType: c.Query("type"), Page: queryInt(c, "p", 1), PageSize: queryInt(c, "page_size", 20)}
	if rawStatus := c.Query("status"); rawStatus != "" {
		status, err := strconv.Atoi(rawStatus)
		if err != nil {
			common.ApiErrorMsg(c, "invalid AIGC model status")
			return
		}
		filter.Status = &status
	}
	items, total, err := handler.admin.List(c.Request.Context(), filter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": items, "total": total, "page": filter.Page, "page_size": filter.PageSize})
}

func (handler *AdminModelHandler) Get(c *gin.Context) {
	id, ok := profileID(c)
	if !ok {
		return
	}
	item, err := handler.admin.Get(c.Request.Context(), id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func (handler *AdminModelHandler) Create(c *gin.Context) {
	var input service.ProfileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiErrorMsg(c, "invalid AIGC model request")
		return
	}
	item, err := handler.admin.Create(c.Request.Context(), input)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func (handler *AdminModelHandler) Update(c *gin.Context) {
	id, ok := profileID(c)
	if !ok {
		return
	}
	var input service.ProfileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiErrorMsg(c, "invalid AIGC model request")
		return
	}
	item, err := handler.admin.Update(c.Request.Context(), id, input)
	if err != nil {
		adminAPIError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func (handler *AdminModelHandler) Validate(c *gin.Context) {
	id, ok := profileID(c)
	if !ok {
		return
	}
	if err := handler.admin.Validate(c.Request.Context(), id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"valid": true})
}

func (handler *AdminModelHandler) Publish(c *gin.Context) {
	id, ok := profileID(c)
	if !ok {
		return
	}
	version, ok := requestedVersion(c)
	if !ok {
		return
	}
	profile, err := handler.admin.Get(c.Request.Context(), id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := handler.publisher.Publish(c.Request.Context(), profile.PublicModelID, version); err != nil {
		adminAPIError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"published": true})
}

func (handler *AdminModelHandler) Disable(c *gin.Context) {
	id, ok := profileID(c)
	if !ok {
		return
	}
	version, ok := requestedVersion(c)
	if !ok {
		return
	}
	if err := handler.admin.Disable(c.Request.Context(), id, version); err != nil {
		adminAPIError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"disabled": true})
}

func (handler *AdminModelHandler) Delete(c *gin.Context) {
	id, ok := profileID(c)
	if !ok {
		return
	}
	version, ok := requestedVersion(c)
	if !ok {
		return
	}
	if err := handler.admin.Delete(c.Request.Context(), id, version); err != nil {
		adminAPIError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"deleted": true})
}

func adminAPIError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, entity.ErrConfigVersionConflict):
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": err.Error(),
			"code":    "CONFIG_VERSION_CONFLICT",
		})
	case errors.Is(err, service.ErrOnlyDraftCanBeDeleted):
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": err.Error(),
			"code":    "MODEL_NOT_DRAFT",
		})
	default:
		common.ApiError(c, err)
	}
}

func profileID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid AIGC model id")
		return 0, false
	}
	return id, true
}

func requestedVersion(c *gin.Context) (int, bool) {
	var body struct {
		ConfigVersion int `json:"config_version"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.ConfigVersion < 1 {
		common.ApiErrorMsg(c, "config_version is required")
		return 0, false
	}
	return body.ConfigVersion, true
}

func queryInt(c *gin.Context, key string, fallback int) int {
	raw := c.Query(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return fallback
	}
	return value
}
