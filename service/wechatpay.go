package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base32"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

const wechatPayRequestTimeout = 15 * time.Second

type WechatPayNativeOrder struct {
	CodeURL   string
	ExpiresAt time.Time
}

func ResolveWechatPayNotifyURL(config setting.WechatPayConfig) string {
	if config.NotifyURL != "" {
		return config.NotifyURL
	}
	return strings.TrimRight(GetCallbackAddress(), "/") + "/api/wechatpay/notify"
}

func ValidateWechatPayConfig(config setting.WechatPayConfig) error {
	if err := ValidateWechatPayConfigFields(config); err != nil {
		return err
	}
	if config.AppID == "" {
		return errors.New("AppID is required")
	}
	if config.MchID == "" {
		return errors.New("merchant ID is required")
	}
	if config.MerchantCertSerialNo == "" {
		return errors.New("merchant certificate serial number is required")
	}
	if config.PublicKeyID == "" {
		return errors.New("WeChat Pay public key ID is required")
	}
	if config.MerchantPrivateKey == "" {
		return errors.New("merchant private key is required")
	}
	if config.APIV3Key == "" {
		return errors.New("APIv3 key is required")
	}
	if config.PublicKey == "" {
		return errors.New("WeChat Pay public key is required")
	}
	if err := validateWechatPayNotifyURL(ResolveWechatPayNotifyURL(config)); err != nil {
		return err
	}
	return nil
}

func ValidateWechatPayConfigFields(config setting.WechatPayConfig) error {
	var merchantPublicKey *rsa.PublicKey
	if config.APIV3Key != "" && len(config.APIV3Key) != 32 {
		return errors.New("APIv3 key must be exactly 32 bytes")
	}
	if config.MerchantPrivateKey != "" {
		privateKey, err := utils.LoadPrivateKey(config.MerchantPrivateKey)
		if err != nil {
			return errors.New("merchant private key is not a valid PKCS#8 RSA PEM key")
		}
		merchantPublicKey = &privateKey.PublicKey
	}
	var wechatPublicKey *rsa.PublicKey
	if config.PublicKey != "" {
		publicKey, err := utils.LoadPublicKey(config.PublicKey)
		if err != nil {
			return errors.New("WeChat Pay public key is not a valid RSA PEM public key")
		}
		wechatPublicKey = publicKey
	}
	if merchantPublicKey != nil && wechatPublicKey != nil &&
		merchantPublicKey.E == wechatPublicKey.E && merchantPublicKey.N.Cmp(wechatPublicKey.N) == 0 {
		return errors.New("WeChat Pay public key must not be the merchant private key's public key")
	}
	if config.NotifyURL != "" {
		if err := validateWechatPayNotifyURL(config.NotifyURL); err != nil {
			return err
		}
	}
	if config.MinTopUp <= 0 {
		return errors.New("minimum top-up must be greater than zero")
	}
	if config.MaxTopUp < config.MinTopUp || config.MaxTopUp > setting.WechatPayHardMaxTopUp {
		return errors.New("maximum top-up must be between the minimum and 4000")
	}
	if config.OrderExpireMinutes < 1 || config.OrderExpireMinutes > 120 {
		return errors.New("order expiration must be between 1 and 120 minutes")
	}
	return nil
}

func validateWechatPayNotifyURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return errors.New("notification URL must be a valid public HTTPS URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("notification URL must not contain credentials, query parameters, or fragments")
	}
	return nil
}

func newWechatPayClient(ctx context.Context, config setting.WechatPayConfig) (*core.Client, error) {
	if err := ValidateWechatPayConfig(config); err != nil {
		return nil, err
	}
	privateKey, err := utils.LoadPrivateKey(config.MerchantPrivateKey)
	if err != nil {
		return nil, err
	}
	publicKey, err := utils.LoadPublicKey(config.PublicKey)
	if err != nil {
		return nil, err
	}
	return core.NewClient(ctx, option.WithWechatPayPublicKeyAuthCipher(
		config.MchID,
		config.MerchantCertSerialNo,
		privateKey,
		config.PublicKeyID,
		publicKey,
	))
}

func NewWechatPayTradeNo() (string, error) {
	randomBytes := make([]byte, 12)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	randomPart := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes))
	tradeNo := "wx" + strconv.FormatInt(time.Now().UnixMilli(), 36) + randomPart
	if len(tradeNo) > 32 {
		tradeNo = tradeNo[:32]
	}
	return tradeNo, nil
}

func CreateWechatPayNativeOrder(ctx context.Context, config setting.WechatPayConfig, tradeNo string, totalFen int64, description string) (*WechatPayNativeOrder, error) {
	if totalFen <= 0 {
		return nil, errors.New("payment amount must be greater than zero")
	}
	client, err := newWechatPayClient(ctx, config)
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(time.Duration(config.OrderExpireMinutes) * time.Minute).Truncate(time.Second)
	currency := "CNY"
	notifyURL := ResolveWechatPayNotifyURL(config)
	service := native.NativeApiService{Client: client}
	requestCtx, cancel := context.WithTimeout(ctx, wechatPayRequestTimeout)
	defer cancel()
	response, _, err := service.Prepay(requestCtx, native.PrepayRequest{
		Appid:       &config.AppID,
		Mchid:       &config.MchID,
		Description: &description,
		OutTradeNo:  &tradeNo,
		TimeExpire:  &expiresAt,
		NotifyUrl:   &notifyURL,
		Amount: &native.Amount{
			Total:    &totalFen,
			Currency: &currency,
		},
	})
	if err != nil {
		return nil, err
	}
	if response == nil || response.CodeUrl == nil || strings.TrimSpace(*response.CodeUrl) == "" {
		return nil, errors.New("WeChat Pay returned an empty code_url")
	}
	return &WechatPayNativeOrder{CodeURL: *response.CodeUrl, ExpiresAt: expiresAt}, nil
}

func QueryWechatPayOrder(ctx context.Context, config setting.WechatPayConfig, tradeNo string) (*payments.Transaction, error) {
	client, err := newWechatPayClient(ctx, config)
	if err != nil {
		return nil, err
	}
	service := native.NativeApiService{Client: client}
	requestCtx, cancel := context.WithTimeout(ctx, wechatPayRequestTimeout)
	defer cancel()
	transaction, _, err := service.QueryOrderByOutTradeNo(requestCtx, native.QueryOrderByOutTradeNoRequest{
		OutTradeNo: &tradeNo,
		Mchid:      &config.MchID,
	})
	return transaction, err
}

func CloseWechatPayOrder(ctx context.Context, config setting.WechatPayConfig, tradeNo string) error {
	client, err := newWechatPayClient(ctx, config)
	if err != nil {
		return err
	}
	service := native.NativeApiService{Client: client}
	requestCtx, cancel := context.WithTimeout(ctx, wechatPayRequestTimeout)
	defer cancel()
	_, err = service.CloseOrder(requestCtx, native.CloseOrderRequest{OutTradeNo: &tradeNo, Mchid: &config.MchID})
	return err
}

func ParseWechatPayNotify(ctx context.Context, request *http.Request, config setting.WechatPayConfig) (*payments.Transaction, error) {
	if err := ValidateWechatPayConfig(config); err != nil {
		return nil, err
	}
	publicKey, err := utils.LoadPublicKey(config.PublicKey)
	if err != nil {
		return nil, err
	}
	handler, err := notify.NewRSANotifyHandler(
		config.APIV3Key,
		verifiers.NewSHA256WithRSAPubkeyVerifier(config.PublicKeyID, *publicKey),
	)
	if err != nil {
		return nil, err
	}
	transaction := new(payments.Transaction)
	if _, err := handler.ParseNotifyRequest(ctx, request, transaction); err != nil {
		return nil, err
	}
	return transaction, nil
}

func ValidateWechatPayTransaction(config setting.WechatPayConfig, topUp *model.TopUp, transaction *payments.Transaction) error {
	if topUp == nil || transaction == nil {
		return errors.New("missing local or provider transaction")
	}
	if transaction.Appid == nil || *transaction.Appid != config.AppID {
		return errors.New("AppID mismatch")
	}
	if transaction.Mchid == nil || *transaction.Mchid != config.MchID {
		return errors.New("merchant ID mismatch")
	}
	if transaction.OutTradeNo == nil || *transaction.OutTradeNo != topUp.TradeNo {
		return errors.New("trade number mismatch")
	}
	if transaction.TradeType == nil || *transaction.TradeType != "NATIVE" {
		return errors.New("trade type mismatch")
	}
	if transaction.TransactionId == nil || strings.TrimSpace(*transaction.TransactionId) == "" {
		return errors.New("provider transaction ID is missing")
	}
	expectedFen, err := model.WechatPayAmountFen(topUp.Money)
	if err != nil || transaction.Amount == nil || transaction.Amount.Total == nil || *transaction.Amount.Total != expectedFen {
		return model.ErrWechatPayAmountMismatch
	}
	if transaction.Amount.Currency == nil || *transaction.Amount.Currency != "CNY" {
		return errors.New("currency mismatch")
	}
	return nil
}

func SettleWechatPayTransaction(config setting.WechatPayConfig, transaction *payments.Transaction, callerIP string) (bool, error) {
	if transaction == nil || transaction.OutTradeNo == nil {
		return false, errors.New("provider trade number is missing")
	}
	if transaction.TradeState == nil || *transaction.TradeState != "SUCCESS" {
		return false, errors.New("transaction is not successful")
	}
	topUp := model.GetTopUpByTradeNo(*transaction.OutTradeNo)
	if topUp == nil {
		return false, model.ErrTopUpNotFound
	}
	if err := ValidateWechatPayTransaction(config, topUp, transaction); err != nil {
		return false, err
	}
	return model.RechargeWechatPay(
		topUp.TradeNo,
		*transaction.Amount.Total,
		*transaction.TransactionId,
		callerIP,
	)
}

func MapWechatPayTradeState(state string) string {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "SUCCESS":
		return common.TopUpStatusSuccess
	case "CLOSED", "REVOKED":
		return common.TopUpStatusExpired
	case "PAYERROR", "REFUND":
		return common.TopUpStatusFailed
	default:
		return common.TopUpStatusPending
	}
}
