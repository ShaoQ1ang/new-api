package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type userChatModelsPayload struct {
	Success bool                       `json:"success"`
	Data    dto.UserChatModelsResponse `json:"data"`
}

type adminChatModelPayload struct {
	Success bool                   `json:"success"`
	Data    dto.AdminChatModelItem `json:"data"`
}

func seedChatModelChannel(t *testing.T, channelID int, group string, modelName string) {
	t.Helper()

	priority := int64(0)
	require.NoError(t, model.DB.Create(&model.Channel{
		Id:       channelID,
		Type:     constant.ChannelTypeOpenAI,
		Key:      "test-key",
		Status:   common.ChannelStatusEnabled,
		Name:     "test-channel",
		Models:   modelName,
		Group:    group,
		Priority: &priority,
	}).Error)
	require.NoError(t, model.DB.Create(&model.Ability{
		Group:     group,
		Model:     modelName,
		ChannelId: channelID,
		Enabled:   true,
		Priority:  &priority,
	}).Error)
	model.InvalidatePricingCache()
}

func TestGetUserChatModelsReturnsAutoAndFiltersUnavailableModels(t *testing.T) {
	ratio_setting.InitRatioSettings()
	setupModelListControllerTestDB(t)

	require.NoError(t, model.DB.Create(&model.User{
		Id:       2001,
		Username: "chat-model-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	seedChatModelChannel(t, 1, "default", "gpt-4o-mini")
	seedChatModelChannel(t, 2, "private", "gpt-3.5-turbo")

	require.NoError(t, model.CreateChatModelOption(&model.ChatModelOption{
		ModelName:   "gpt-4o-mini",
		DisplayName: "GPT-4o mini",
		Enabled:     true,
		IsAuto:      true,
		Sort:        1,
	}))
	require.NoError(t, model.CreateChatModelOption(&model.ChatModelOption{
		ModelName:   "gpt-3.5-turbo",
		DisplayName: "Private model",
		Enabled:     true,
		Sort:        2,
	}))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/self/chat-models", nil)
	ctx.Set("id", 2001)

	GetUserChatModels(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload userChatModelsPayload
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, 1, payload.Data.Total)
	require.Len(t, payload.Data.Models, 1)
	require.Equal(t, "gpt-4o-mini", payload.Data.Models[0].Model)
	require.Equal(t, "Auto", payload.Data.Models[0].Name)
	require.Equal(t, 0.15, payload.Data.Models[0].Price)
}

func TestUpdateChatModelSetsSingleAutoWithOptionalFields(t *testing.T) {
	ratio_setting.InitRatioSettings()
	setupModelListControllerTestDB(t)

	seedChatModelChannel(t, 1, "default", "gpt-4o-mini")
	seedChatModelChannel(t, 2, "default", "gpt-3.5-turbo")

	first := model.ChatModelOption{
		ModelName:   "gpt-4o-mini",
		DisplayName: "First",
		Enabled:     true,
		IsAuto:      true,
		Sort:        1,
	}
	require.NoError(t, model.CreateChatModelOption(&first))
	second := model.ChatModelOption{
		ModelName:   "gpt-3.5-turbo",
		DisplayName: "Second",
		Enabled:     true,
		Sort:        2,
	}
	require.NoError(t, model.CreateChatModelOption(&second))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPatch, "/api/chat-models/2", strings.NewReader(`{"is_auto":true,"name":"Auto target"}`))
	ctx.Set("id", 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(second.Id)}}

	UpdateChatModel(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload adminChatModelPayload
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, "gpt-3.5-turbo", payload.Data.Model)
	require.Equal(t, "Auto target", payload.Data.Name)
	require.True(t, payload.Data.IsAuto)

	var options []model.ChatModelOption
	require.NoError(t, model.DB.Order("id ASC").Find(&options).Error)
	require.Len(t, options, 2)
	require.False(t, options[0].IsAuto)
	require.True(t, options[1].IsAuto)
	require.Equal(t, "gpt-4o-mini", options[0].ModelName)
	require.Equal(t, "gpt-3.5-turbo", options[1].ModelName)
}
