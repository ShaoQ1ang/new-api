package controller

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

const (
	wechatPayNotifyBodyLimit = 64 << 10
	wechatPayAdminBodyLimit  = 256 << 10
)

type WechatPayPayRequest struct {
	Amount        int64  `json:"amount"`
	PaymentMethod string `json:"payment_method"`
}

type WechatPayAdminUpdateRequest struct {
	Enabled              *bool    `json:"enabled,omitempty"`
	AppID                *string  `json:"app_id,omitempty"`
	MchID                *string  `json:"mch_id,omitempty"`
	MerchantCertSerialNo *string  `json:"merchant_cert_serial_no,omitempty"`
	MerchantPrivateKey   *string  `json:"merchant_private_key,omitempty"`
	APIV3Key             *string  `json:"api_v3_key,omitempty"`
	PublicKeyID          *string  `json:"public_key_id,omitempty"`
	PublicKey            *string  `json:"public_key,omitempty"`
	NotifyURL            *string  `json:"notify_url,omitempty"`
	MinTopUp             *int     `json:"min_topup,omitempty"`
	MaxTopUp             *int     `json:"max_topup,omitempty"`
	OrderExpireMinutes   *int     `json:"order_expire_minutes,omitempty"`
	ClearSecrets         []string `json:"clear_secrets,omitempty"`
	ForceClearSecrets    bool     `json:"force_clear_secrets,omitempty"`
}

func getWechatPayDisplayAmountLimit(value int) (int64, error) {
	if value <= 0 {
		return 0, model.ErrWechatPayAmountInvalid
	}
	limit := decimal.NewFromInt(int64(value))
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		if math.IsNaN(common.QuotaPerUnit) || math.IsInf(common.QuotaPerUnit, 0) || common.QuotaPerUnit <= 0 {
			return 0, model.ErrWechatPayAmountInvalid
		}
		limit = limit.Mul(decimal.NewFromFloat(common.QuotaPerUnit)).Truncate(0)
	}
	if limit.LessThanOrEqual(decimal.Zero) || limit.GreaterThan(decimal.NewFromInt(math.MaxInt64)) {
		return 0, model.ErrWechatPayAmountInvalid
	}
	return limit.IntPart(), nil
}

func normalizeWechatPayTopUpAmount(amount int64) (int64, error) {
	if operation_setting.GetQuotaDisplayType() != operation_setting.QuotaDisplayTypeTokens {
		return amount, nil
	}
	if math.IsNaN(common.QuotaPerUnit) || math.IsInf(common.QuotaPerUnit, 0) || common.QuotaPerUnit <= 0 {
		return 0, model.ErrWechatPayAmountInvalid
	}
	normalized := decimal.NewFromInt(amount).Div(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart()
	if normalized < 1 {
		return 1, nil
	}
	return normalized, nil
}

func RequestWechatPayNative(c *gin.Context) {
	config := setting.GetWechatPayConfig()
	if !isWechatPayTopUpEnabledWithConfig(config) {
		common.ApiErrorI18n(c, i18n.MsgPaymentNotConfigured)
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var request WechatPayPayRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if request.PaymentMethod != model.PaymentMethodWechatPay {
		common.ApiErrorI18n(c, i18n.MsgPaymentChannelNotSupported)
		return
	}
	minTopUp, err := getWechatPayDisplayAmountLimit(config.MinTopUp)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgPaymentConfigError)
		return
	}
	maxTopUp, err := getWechatPayDisplayAmountLimit(config.MaxTopUp)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgPaymentConfigError)
		return
	}
	if request.Amount < minTopUp {
		common.ApiErrorI18n(c, i18n.MsgPaymentMinTopup, map[string]any{"Min": minTopUp})
		return
	}
	if request.Amount > maxTopUp {
		common.ApiErrorI18n(c, i18n.MsgPaymentAmountExceedMax, map[string]any{"Max": maxTopUp})
		return
	}

	userID := c.GetInt("id")
	group, err := model.GetUserGroup(userID, true)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgPaymentUserGroupFailed)
		return
	}
	payMoney := getPayMoney(request.Amount, group)
	totalFen, err := model.WechatPayAmountFen(payMoney)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgPaymentAmountTooLow)
		return
	}
	normalizedAmount, err := normalizeWechatPayTopUpAmount(request.Amount)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgPaymentAmountExceedMax, map[string]any{"Max": maxTopUp})
		return
	}
	quotaToAdd, err := model.WechatPayQuotaToAdd(normalizedAmount)
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgPaymentAmountExceedMax, map[string]any{"Max": maxTopUp})
		return
	}
	if err := model.CheckWechatPayQuotaCapacity(userID, quotaToAdd); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("WeChat Pay quota capacity check failed user_id=%d error=%q", userID, err.Error()))
		common.ApiErrorI18n(c, i18n.MsgPaymentStartFailed)
		return
	}
	tradeNo, err := service.NewWechatPayTradeNo()
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("WeChat Pay trade number generation failed user_id=%d", userID))
		common.ApiErrorI18n(c, i18n.MsgPaymentStartFailed)
		return
	}

	providerOrder, err := service.CreateWechatPayNativeOrder(
		c.Request.Context(),
		config,
		tradeNo,
		totalFen,
		fmt.Sprintf("Wallet top-up %d", request.Amount),
	)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("WeChat Pay order creation failed user_id=%d trade_no=%s error=%q", userID, tradeNo, err.Error()))
		common.ApiErrorI18n(c, i18n.MsgPaymentStartFailed)
		return
	}

	topUp := &model.TopUp{
		UserId:          userID,
		Amount:          normalizedAmount,
		Money:           payMoney,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodWechatPay,
		PaymentProvider: model.PaymentProviderWechatPay,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	topUp.ApplyPaymentSnapshot(buildPaymentSnapshot(float64(request.Amount), payMoney, "CNY"))
	if err := model.CreateWechatPayTopUpWithPendingTask(
		topUp,
		service.NextWechatPayPendingQueryTime(time.Now()),
		providerOrder.ExpiresAt.Unix(),
	); err != nil {
		closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = service.CloseWechatPayOrder(closeCtx, config, tradeNo)
		cancel()
		logger.LogError(c.Request.Context(), fmt.Sprintf("WeChat Pay local order creation failed user_id=%d trade_no=%s", userID, tradeNo))
		common.ApiErrorI18n(c, i18n.MsgPaymentCreateFailed)
		return
	}

	common.ApiSuccess(c, gin.H{
		"pay_type":   "qrcode",
		"code_url":   providerOrder.CodeURL,
		"trade_no":   tradeNo,
		"expires_at": providerOrder.ExpiresAt.Unix(),
	})
}

func WechatPayNotify(c *gin.Context) {
	config := setting.GetWechatPayConfig()
	if !isWechatPayWebhookEnabledWithConfig(config) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, wechatPayNotifyBodyLimit)
	transaction, err := service.ParseWechatPayNotify(c.Request.Context(), c.Request, config)
	if err != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("WeChat Pay callback verification or decryption failed client_ip=%s", c.ClientIP()))
		wechatPayNotifyFailure(c, http.StatusUnauthorized, "invalid notification")
		return
	}
	if transaction.TradeState == nil || *transaction.TradeState != "SUCCESS" {
		wechatPayNotifyFailure(c, http.StatusBadRequest, "unsupported trade state")
		return
	}
	if _, err := service.SettleWechatPayTransaction(config, transaction, c.ClientIP()); err != nil {
		tradeNo := ""
		if transaction.OutTradeNo != nil {
			tradeNo = *transaction.OutTradeNo
		}
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("WeChat Pay callback business validation failed trade_no=%s error=%q", tradeNo, err.Error()))
		status := http.StatusBadRequest
		if !errors.Is(err, model.ErrTopUpNotFound) &&
			!errors.Is(err, model.ErrPaymentMethodMismatch) &&
			!errors.Is(err, model.ErrWechatPayAmountMismatch) &&
			!errors.Is(err, model.ErrTopUpStatusInvalid) {
			status = http.StatusInternalServerError
		}
		wechatPayNotifyFailure(c, status, "payment processing failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "success"})
}

func wechatPayNotifyFailure(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"code": "FAIL", "message": message})
}

func GetWechatPayAdminSettings(c *gin.Context) {
	pendingCount, err := model.CountPendingTopUpsByProvider(model.PaymentProviderWechatPay)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("WeChat Pay pending order count failed: %v", err))
		common.ApiErrorMsg(c, "微信支付待处理订单数量读取失败")
		return
	}
	respondWechatPayAdminSettings(c, pendingCount)
}

func respondWechatPayAdminSettings(c *gin.Context, pendingCount int64) {
	config := setting.GetWechatPayConfig()
	common.ApiSuccess(c, gin.H{
		"enabled":                         config.Enabled,
		"app_id":                          config.AppID,
		"mch_id":                          config.MchID,
		"merchant_cert_serial_no":         config.MerchantCertSerialNo,
		"merchant_private_key_configured": config.MerchantPrivateKey != "",
		"api_v3_key_configured":           config.APIV3Key != "",
		"public_key_id":                   config.PublicKeyID,
		"public_key_configured":           config.PublicKey != "",
		"notify_url":                      config.NotifyURL,
		"resolved_notify_url":             service.ResolveWechatPayNotifyURL(config),
		"min_topup":                       config.MinTopUp,
		"max_topup":                       config.MaxTopUp,
		"order_expire_minutes":            config.OrderExpireMinutes,
		"pending_order_count":             pendingCount,
		"option_crypt_key_configured":     common.GetOptionCryptKey() != "",
	})
}

func UpdateWechatPayAdminSettings(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, wechatPayAdminBodyLimit)
	var request WechatPayAdminUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorMsg(c, "无效的微信支付配置")
		return
	}
	if request.MinTopUp != nil && *request.MinTopUp <= 0 {
		common.ApiErrorMsg(c, "最低充值数量必须大于零")
		return
	}
	if request.MaxTopUp != nil && (*request.MaxTopUp <= 0 || *request.MaxTopUp > setting.WechatPayHardMaxTopUp) {
		common.ApiErrorMsg(c, "最高充值数量必须在 1 到 4000 之间")
		return
	}
	if request.OrderExpireMinutes != nil && (*request.OrderExpireMinutes < 1 || *request.OrderExpireMinutes > 120) {
		common.ApiErrorMsg(c, "订单有效期必须在 1 到 120 分钟之间")
		return
	}
	pendingCount, err := model.CountPendingTopUpsByProvider(model.PaymentProviderWechatPay)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("WeChat Pay pending order count failed: %v", err))
		common.ApiErrorMsg(c, "无法确认微信支付待处理订单，配置未更新")
		return
	}

	config := setting.GetWechatPayConfig()
	updates := make(map[string]string)
	if request.Enabled != nil {
		config.Enabled = *request.Enabled
		updates[setting.WechatPayEnabledOptionKey] = strconv.FormatBool(*request.Enabled)
	}
	applyWechatPayStringUpdate(request.AppID, &config.AppID, setting.WechatPayAppIDOptionKey, updates, true, false)
	applyWechatPayStringUpdate(request.MchID, &config.MchID, setting.WechatPayMchIDOptionKey, updates, true, false)
	applyWechatPayStringUpdate(request.MerchantCertSerialNo, &config.MerchantCertSerialNo, setting.WechatPayMerchantCertSerialNoOptionKey, updates, true, false)
	applyWechatPayStringUpdate(request.MerchantPrivateKey, &config.MerchantPrivateKey, setting.WechatPayMerchantPrivateKeyOptionKey, updates, true, true)
	applyWechatPayStringUpdate(request.APIV3Key, &config.APIV3Key, setting.WechatPayAPIV3KeyOptionKey, updates, false, true)
	applyWechatPayStringUpdate(request.PublicKeyID, &config.PublicKeyID, setting.WechatPayPublicKeyIDOptionKey, updates, true, false)
	applyWechatPayStringUpdate(request.PublicKey, &config.PublicKey, setting.WechatPayPublicKeyOptionKey, updates, true, true)
	applyWechatPayStringUpdate(request.NotifyURL, &config.NotifyURL, setting.WechatPayNotifyURLOptionKey, updates, true, false)
	if request.MinTopUp != nil {
		config.MinTopUp = *request.MinTopUp
		updates[setting.WechatPayMinTopUpOptionKey] = strconv.Itoa(*request.MinTopUp)
	}
	if request.MaxTopUp != nil {
		config.MaxTopUp = *request.MaxTopUp
		updates[setting.WechatPayMaxTopUpOptionKey] = strconv.Itoa(*request.MaxTopUp)
	}
	if request.OrderExpireMinutes != nil {
		config.OrderExpireMinutes = *request.OrderExpireMinutes
		updates[setting.WechatPayOrderExpireMinutesOptionKey] = strconv.Itoa(*request.OrderExpireMinutes)
	}
	if len(request.ClearSecrets) > 0 && !request.ForceClearSecrets {
		if pendingCount > 0 {
			common.ApiErrorMsg(c, fmt.Sprintf("仍有 %d 笔待处理订单；请先完成对账，紧急吊销时显式设置 force_clear_secrets", pendingCount))
			return
		}
	}
	if err := clearWechatPaySecrets(&config, updates, request.ClearSecrets); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	for key := range updates {
		if common.IsSensitiveOptionKey(key) && common.GetOptionCryptKey() == "" {
			common.ApiErrorMsg(c, "保存微信支付密钥前必须配置环境变量 OPTION_CRYPT_KEY")
			return
		}
	}
	validateFinalConfig := func(finalConfig setting.WechatPayConfig) error {
		if err := service.ValidateWechatPayConfigFields(finalConfig); err != nil {
			return err
		}
		if finalConfig.Enabled {
			if common.GetOptionCryptKey() == "" {
				return errors.New("启用微信支付前必须配置环境变量 OPTION_CRYPT_KEY")
			}
			return service.ValidateWechatPayConfig(finalConfig)
		}
		return nil
	}
	if err := model.UpdateWechatPayOptionsWithValidation(updates, validateFinalConfig); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("WeChat Pay settings update failed: %v", err))
		common.ApiErrorMsg(c, "微信支付配置保存失败")
		return
	}
	respondWechatPayAdminSettings(c, pendingCount)
}

func applyWechatPayStringUpdate(input *string, target *string, key string, updates map[string]string, trim bool, keepWhenBlank bool) {
	if input == nil {
		return
	}
	value := *input
	if trim {
		value = strings.TrimSpace(value)
	}
	if keepWhenBlank && value == "" {
		return
	}
	*target = value
	updates[key] = value
}

func clearWechatPaySecrets(config *setting.WechatPayConfig, updates map[string]string, names []string) error {
	cleared := false
	for _, rawName := range names {
		switch strings.ToLower(strings.TrimSpace(rawName)) {
		case "merchant_private_key":
			config.MerchantPrivateKey = ""
			updates[setting.WechatPayMerchantPrivateKeyOptionKey] = ""
			cleared = true
		case "api_v3_key":
			config.APIV3Key = ""
			updates[setting.WechatPayAPIV3KeyOptionKey] = ""
			cleared = true
		case "public_key":
			config.PublicKey = ""
			updates[setting.WechatPayPublicKeyOptionKey] = ""
			cleared = true
		case "":
			continue
		default:
			return fmt.Errorf("不支持清除密钥 %q", rawName)
		}
	}
	if cleared {
		config.Enabled = false
		updates[setting.WechatPayEnabledOptionKey] = "false"
	}
	return nil
}
