package controller

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const maxManagementPermissionRequestBytes int64 = 16 << 10

type updateManagementPermissionsRequest struct {
	Permissions []string `json:"permissions"`
}

type managementPermissionsResponse struct {
	UserId               int      `json:"user_id"`
	Role                 int      `json:"role"`
	Status               int      `json:"status"`
	Permissions          []string `json:"permissions"`
	EffectivePermissions []string `json:"effective_permissions"`
	AvailablePermissions []string `json:"available_permissions"`
}

func buildManagementPermissionsResponse(state *model.UserManagementPermissionState) managementPermissionsResponse {
	return managementPermissionsResponse{
		UserId:               state.UserId,
		Role:                 state.Role,
		Status:               state.Status,
		Permissions:          state.Assigned,
		EffectivePermissions: state.Effective,
		AvailablePermissions: constant.AllManagementPermissions(),
	}
}

func requireEnabledRootForManagementPermissions(c *gin.Context) bool {
	allowed, err := model.IsEnabledRootUser(c.GetInt("id"))
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return false
	}
	if !allowed {
		common.ApiErrorI18n(c, i18n.MsgAuthInsufficientPrivilege)
		return false
	}
	return true
}

func GetUserManagementPermissions(c *gin.Context) {
	if !requireEnabledRootForManagementPermissions(c) {
		return
	}
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	state, err := model.GetUserManagementPermissionState(userId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorI18n(c, i18n.MsgUserNotExists)
			return
		}
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	common.ApiSuccess(c, buildManagementPermissionsResponse(state))
}

func UpdateUserManagementPermissions(c *gin.Context) {
	if !requireEnabledRootForManagementPermissions(c) {
		return
	}
	userId, err := strconv.Atoi(c.Param("id"))
	if err != nil || userId <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxManagementPermissionRequestBytes)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	var req updateManagementPermissionsRequest
	if err := common.Unmarshal(body, &req); err != nil || req.Permissions == nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	grantedBy := c.GetInt("id")
	previous, current, err := model.ReplaceUserManagementPermissions(userId, req.Permissions, grantedBy)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			common.ApiErrorI18n(c, i18n.MsgUserNotExists)
		case errors.Is(err, model.ErrInvalidManagementPermission),
			errors.Is(err, model.ErrManagementPermissionsRequireCommonUser):
			common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		case errors.Is(err, model.ErrManagementPermissionGrantorNotRoot):
			common.ApiErrorI18n(c, i18n.MsgAuthInsufficientPrivilege)
		default:
			common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		}
		return
	}

	model.RecordLogWithAdminInfo(userId, model.LogTypeManage,
		"root updated user management permissions", map[string]interface{}{
			"admin_id":           grantedBy,
			"admin_username":     c.GetString("username"),
			"permissions_before": previous,
			"permissions_after":  current,
		})

	state, err := model.GetUserManagementPermissionState(userId)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgDatabaseError)
		return
	}
	common.ApiSuccess(c, buildManagementPermissionsResponse(state))
}
