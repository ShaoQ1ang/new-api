package relay

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type SyncWorkflow struct {
	OnChannelError TaskChannelErrorHandler
}

type SyncWorkflowResult struct {
	ResponseBody []byte
}

func (workflow *SyncWorkflow) Execute(source *gin.Context, relayFormat types.RelayFormat) (result *SyncWorkflowResult, workflowErr *types.NewAPIError) {
	c, _ := newSyncRelayContext(source)
	defer common.CleanupBodyStorage(c)
	request, err := helper.GetAndValidateRequest(c, relayFormat)
	if err != nil {
		if common.IsRequestBodyTooLargeError(err) || errors.Is(err, common.ErrRequestBodyTooLarge) {
			return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
		}
		return nil, types.NewError(err, types.ErrorCodeInvalidRequest)
	}
	relayInfo, err := relaycommon.GenRelayInfo(c, relayFormat, request, nil)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeGenRelayInfoFailed)
	}

	meta := syncTokenCountMeta(request)
	if setting.ShouldCheckPromptSensitive() && meta != nil {
		contains, words := service.CheckSensitiveText(meta.CombineText)
		if contains {
			return nil, types.NewError(fmt.Errorf("sensitive words detected: %s", strings.Join(words, ", ")), types.ErrorCodeSensitiveWordsDetected)
		}
	}
	tokens, err := service.EstimateRequestToken(c, meta, relayInfo)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeCountTokenFailed)
	}
	relayInfo.SetEstimatePromptTokens(tokens)
	priceData, err := helper.ModelPriceHelper(c, relayInfo, tokens, meta)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithStatusCode(http.StatusBadRequest))
	}
	if !priceData.FreeModel {
		if workflowErr = service.PreConsumeBilling(c, priceData.QuotaToPreConsume, relayInfo); workflowErr != nil {
			return nil, workflowErr
		}
	}
	defer func() {
		if workflowErr == nil {
			return
		}
		workflowErr = service.NormalizeViolationFeeError(workflowErr)
		if relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
		service.ChargeViolationFeeIfNeeded(c, relayInfo, workflowErr)
	}()

	retryParam := &service.RetryParam{
		Ctx: c, TokenGroup: relayInfo.TokenGroup, ModelName: relayInfo.OriginModelName,
		RequestPath: c.Request.URL.Path, Retry: common.GetPointer(0),
	}
	relayInfo.RetryIndex = 0
	relayInfo.LastError = nil
	for ; retryParam.GetRetry() <= common.RetryTimes; retryParam.IncreaseRetry() {
		relayInfo.RetryIndex = retryParam.GetRetry()
		channel, channelErr := selectSyncChannel(c, relayInfo, retryParam)
		if channelErr != nil {
			logger.LogError(c, channelErr.Error())
			workflowErr = channelErr
			break
		}
		addSyncUsedChannel(c, channel.Id)
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				workflowErr = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
			} else {
				workflowErr = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			}
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)
		attempt, recorder := newSyncRelayContext(c)
		workflowErr = executeSyncRelay(attempt, relayInfo)
		if workflowErr == nil {
			relayInfo.LastError = nil
			return &SyncWorkflowResult{ResponseBody: append([]byte(nil), recorder.Body.Bytes()...)}, nil
		}
		workflowErr = service.NormalizeViolationFeeError(workflowErr)
		relayInfo.LastError = workflowErr
		if workflow.OnChannelError != nil {
			workflow.OnChannelError(attempt,
				*types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey,
					common.GetContextKeyString(attempt, constant.ContextKeyChannelKey), channel.GetAutoBan()), workflowErr)
		}
		if !shouldRetrySync(c, workflowErr, common.RetryTimes-retryParam.GetRetry()) {
			break
		}
	}
	if usedChannels := c.GetStringSlice("use_channel"); len(usedChannels) > 1 {
		logger.LogInfo(c, fmt.Sprintf("重试：%s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(usedChannels)), "->"), "[]")))
	}
	return nil, workflowErr
}

func newSyncRelayContext(source *gin.Context) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = source.Request
	c.Params = append(gin.Params(nil), source.Params...)
	for key, value := range source.Keys {
		c.Set(key, value)
	}
	return c, recorder
}

func syncTokenCountMeta(request dto.Request) *types.TokenCountMeta {
	if request == nil {
		return &types.TokenCountMeta{}
	}
	if setting.ShouldCheckPromptSensitive() || constant.CountToken {
		return request.GetTokenCountMeta()
	}
	if imageRequest, ok := request.(*dto.ImageRequest); ok {
		return imageRequest.GetTokenCountMeta()
	}
	return &types.TokenCountMeta{TokenType: types.TokenTypeTokenizer}
}

func executeSyncRelay(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	switch info.RelayMode {
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		return ImageHelper(c, info)
	case relayconstant.RelayModeChatCompletions:
		return TextHelper(c, info)
	default:
		return types.NewErrorWithStatusCode(fmt.Errorf("unsupported synchronous relay mode %d", info.RelayMode), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
}

func selectSyncChannel(c *gin.Context, info *relaycommon.RelayInfo, retryParam *service.RetryParam) (*model.Channel, *types.NewAPIError) {
	if !syncWorkflowNeedsChannelSelection(c, info) {
		autoBanInt := 0
		if c.GetBool("auto_ban") {
			autoBanInt = 1
		}
		return &model.Channel{Id: c.GetInt("channel_id"), Type: c.GetInt("channel_type"), Name: c.GetString("channel_name"), AutoBan: &autoBanInt}, nil
	}
	channel, selectGroup, err := service.CacheGetRandomSatisfiedChannel(retryParam)
	info.PriceData.GroupRatioInfo = helper.HandleGroupRatio(c, info)
	if err != nil {
		return nil, types.NewError(fmt.Errorf("获取分组 %s 下模型 %s 的可用渠道失败（retry）: %s", selectGroup, info.OriginModelName, err.Error()), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	if channel == nil {
		return nil, types.NewError(fmt.Errorf("分组 %s 下模型 %s 的可用渠道不存在（retry）", selectGroup, info.OriginModelName), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	if setupErr := middleware.SetupContextForSelectedChannel(c, channel, info.OriginModelName); setupErr != nil {
		return nil, setupErr
	}
	return channel, nil
}

func syncWorkflowNeedsChannelSelection(c *gin.Context, info *relaycommon.RelayInfo) bool {
	return info.ChannelMeta != nil || common.GetContextKeyInt(c, constant.ContextKeyChannelId) <= 0
}

func addSyncUsedChannel(c *gin.Context, channelID int) {
	used := c.GetStringSlice("use_channel")
	c.Set("use_channel", append(used, fmt.Sprintf("%d", channelID)))
}

func shouldRetrySync(c *gin.Context, relayErr *types.NewAPIError, remaining int) bool {
	if relayErr == nil || service.ShouldSkipRetryAfterChannelAffinityFailure(c) || remaining <= 0 {
		return false
	}
	if types.IsChannelError(relayErr) {
		return true
	}
	if types.IsSkipRetryError(relayErr) {
		return false
	}
	if _, specific := c.Get("specific_channel_id"); specific {
		return false
	}
	status := relayErr.StatusCode
	if status >= http.StatusOK && status < http.StatusMultipleChoices {
		return false
	}
	if status < 100 || status > 599 {
		return true
	}
	if operation_setting.IsAlwaysSkipRetryCode(relayErr.GetErrorCode()) {
		return false
	}
	return operation_setting.ShouldRetryByStatusCode(status)
}
