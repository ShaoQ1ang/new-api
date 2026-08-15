package relay

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type TaskChannelErrorHandler func(c *gin.Context, channelError types.ChannelError, err *types.NewAPIError)

type TaskWorkflow struct {
	OnChannelError TaskChannelErrorHandler
}

type TaskWorkflowResult struct {
	Task           *model.Task
	PublicResponse any
}

func (workflow TaskWorkflow) Submit(c *gin.Context, relayInfo *relaycommon.RelayInfo) (workflowResult *TaskWorkflowResult, taskErr *dto.TaskError) {
	if taskErr = ResolveOriginTask(c, relayInfo); taskErr != nil {
		return nil, taskErr
	}
	defer func() {
		if taskErr != nil && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
	}()

	retryParam := &service.RetryParam{
		Ctx: c, TokenGroup: relayInfo.TokenGroup, ModelName: relayInfo.OriginModelName,
		RequestPath: c.Request.URL.Path, Retry: common.GetPointer(0),
	}
	var result *TaskSubmitResult
	for ; retryParam.GetRetry() <= common.RetryTimes; retryParam.IncreaseRetry() {
		var channel *model.Channel
		if locked, ok := relayInfo.LockedChannel.(*model.Channel); ok && locked != nil {
			channel = locked
			if retryParam.GetRetry() > 0 {
				if setupErr := middleware.SetupContextForSelectedChannel(c, channel, relayInfo.OriginModelName); setupErr != nil {
					taskErr = service.TaskErrorWrapperLocal(setupErr.Err, "setup_locked_channel_failed", http.StatusInternalServerError)
					break
				}
			}
		} else {
			var channelErr *types.NewAPIError
			channel, channelErr = selectTaskChannel(c, relayInfo, retryParam)
			if channelErr != nil {
				logger.LogError(c, channelErr.Error())
				taskErr = service.TaskErrorWrapperLocal(channelErr.Err, "get_channel_failed", http.StatusInternalServerError)
				break
			}
		}

		addTaskUsedChannel(c, channel.Id)
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusRequestEntityTooLarge)
			} else {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusBadRequest)
			}
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)

		result, taskErr = RelayTaskSubmit(c, relayInfo)
		if taskErr == nil {
			break
		}
		if !taskErr.LocalError && workflow.OnChannelError != nil {
			workflow.OnChannelError(c,
				*types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey,
					common.GetContextKeyString(c, constant.ContextKeyChannelKey), channel.GetAutoBan()),
				types.NewOpenAIError(taskErr.Error, types.ErrorCodeBadResponseStatusCode, taskErr.StatusCode))
		}
		if !ShouldRetryTaskSubmission(c, taskErr, common.RetryTimes-retryParam.GetRetry()) {
			break
		}
	}

	if usedChannels := c.GetStringSlice("use_channel"); len(usedChannels) > 1 {
		retryLog := fmt.Sprintf("重试：%s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(usedChannels)), "->"), "[]"))
		logger.LogInfo(c, retryLog)
	}
	if taskErr != nil {
		return nil, taskErr
	}
	if result == nil {
		return nil, service.TaskErrorWrapperLocal(errors.New("task workflow returned no result"), "empty_task_result", http.StatusInternalServerError)
	}

	if settleErr := service.SettleBilling(c, relayInfo, result.Quota); settleErr != nil {
		common.SysError("settle task billing error: " + settleErr.Error())
	}
	service.LogTaskConsumption(c, relayInfo)
	task := taskModelFromSubmit(relayInfo, result)
	if insertErr := task.Insert(); insertErr != nil {
		common.SysError("insert task error: " + insertErr.Error())
	}
	return &TaskWorkflowResult{Task: task, PublicResponse: result.PublicResponse}, nil
}

func selectTaskChannel(c *gin.Context, info *relaycommon.RelayInfo, retryParam *service.RetryParam) (*model.Channel, *types.NewAPIError) {
	if !taskWorkflowNeedsChannelSelection(c, info) {
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

func taskWorkflowNeedsChannelSelection(c *gin.Context, info *relaycommon.RelayInfo) bool {
	return info.ChannelMeta != nil || common.GetContextKeyInt(c, constant.ContextKeyChannelId) <= 0
}

func taskModelFromSubmit(relayInfo *relaycommon.RelayInfo, result *TaskSubmitResult) *model.Task {
	task := model.InitTask(result.Platform, relayInfo)
	task.PrivateData.UpstreamTaskID = result.UpstreamTaskID
	task.PrivateData.BillingSource = relayInfo.BillingSource
	task.PrivateData.SubscriptionId = relayInfo.SubscriptionId
	task.PrivateData.TokenId = relayInfo.TokenId
	task.PrivateData.NodeName = common.NodeName
	task.PrivateData.BillingContext = &model.TaskBillingContext{
		ModelPrice: relayInfo.PriceData.ModelPrice, GroupRatio: relayInfo.PriceData.GroupRatioInfo.GroupRatio,
		ModelRatio: relayInfo.PriceData.ModelRatio, ConditionalInputPrice: relayInfo.PriceData.ConditionalInputPrice,
		InputImageCost: relayInfo.PriceData.InputImageCost, InputImageCounts: relayInfo.PriceData.InputImageCounts,
		InputImageFreeCount: relayInfo.PriceData.InputImageFreeCount, VideoSecondsUnitPrice: relayInfo.PriceData.VideoSecondsUnitPrice,
		VideoSecondsTier: relayInfo.PriceData.VideoSecondsTier, VideoDurationSeconds: relayInfo.PriceData.VideoDurationSeconds,
		VideoFixedPrice: relayInfo.PriceData.VideoFixedPrice, VideoAudioEnabled: relayInfo.PriceData.VideoAudioEnabled,
		OtherRatios: relayInfo.PriceData.OtherRatios(), OriginModelName: relayInfo.OriginModelName,
		PerCallBilling: common.StringsContains(constant.TaskPricePatches, relayInfo.OriginModelName) || relayInfo.PriceData.UsePrice,
	}
	task.Quota = result.Quota
	task.Data = result.TaskData
	task.Action = relayInfo.Action
	return task
}

func addTaskUsedChannel(c *gin.Context, channelID int) {
	used := c.GetStringSlice("use_channel")
	c.Set("use_channel", append(used, fmt.Sprintf("%d", channelID)))
}

func ShouldRetryTaskSubmission(c *gin.Context, taskErr *dto.TaskError, remaining int) bool {
	if taskErr == nil || service.ShouldSkipRetryAfterChannelAffinityFailure(c) || remaining <= 0 {
		return false
	}
	if _, specific := c.Get("specific_channel_id"); specific {
		return false
	}
	if taskErr.LocalError {
		return false
	}
	switch taskErr.StatusCode {
	case http.StatusTooManyRequests, http.StatusTemporaryRedirect:
		return true
	case http.StatusBadRequest, http.StatusRequestTimeout:
		return false
	}
	if taskErr.StatusCode/100 == 5 {
		return !operation_setting.IsAlwaysSkipRetryStatusCode(taskErr.StatusCode)
	}
	if taskErr.StatusCode/100 == 2 {
		return false
	}
	return true
}
