package setting

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	WechatPayEnabledOptionKey              = "WechatPayEnabled"
	WechatPayAppIDOptionKey                = "WechatPayAppID"
	WechatPayMchIDOptionKey                = "WechatPayMchID"
	WechatPayMerchantCertSerialNoOptionKey = "WechatPayMerchantCertSerialNo"
	WechatPayMerchantPrivateKeyOptionKey   = "WechatPayMerchantPrivateKey"
	WechatPayAPIV3KeyOptionKey             = "WechatPayAPIV3Key"
	WechatPayPublicKeyIDOptionKey          = "WechatPayPublicKeyID"
	WechatPayPublicKeyOptionKey            = "WechatPayPublicKey"
	WechatPayNotifyURLOptionKey            = "WechatPayNotifyURL"
	WechatPayMinTopUpOptionKey             = "WechatPayMinTopUp"
	WechatPayMaxTopUpOptionKey             = "WechatPayMaxTopUp"
	WechatPayOrderExpireMinutesOptionKey   = "WechatPayOrderExpireMinutes"
	WechatPayHardMaxTopUp                  = 4000
)

var WechatPayOptionKeys = []string{
	WechatPayEnabledOptionKey,
	WechatPayAppIDOptionKey,
	WechatPayMchIDOptionKey,
	WechatPayMerchantCertSerialNoOptionKey,
	WechatPayMerchantPrivateKeyOptionKey,
	WechatPayAPIV3KeyOptionKey,
	WechatPayPublicKeyIDOptionKey,
	WechatPayPublicKeyOptionKey,
	WechatPayNotifyURLOptionKey,
	WechatPayMinTopUpOptionKey,
	WechatPayMaxTopUpOptionKey,
	WechatPayOrderExpireMinutesOptionKey,
}

type WechatPayConfig struct {
	Enabled              bool
	AppID                string
	MchID                string
	MerchantCertSerialNo string
	MerchantPrivateKey   string
	APIV3Key             string
	PublicKeyID          string
	PublicKey            string
	NotifyURL            string
	MinTopUp             int
	MaxTopUp             int
	OrderExpireMinutes   int
}

func GetWechatPayConfig() WechatPayConfig {
	common.OptionMapRWMutex.RLock()
	values := make(map[string]string, len(WechatPayOptionKeys))
	for _, key := range WechatPayOptionKeys {
		values[key] = common.OptionMap[key]
	}
	common.OptionMapRWMutex.RUnlock()
	return WechatPayConfigFromOptions(values)
}

func WechatPayConfigFromOptions(values map[string]string) WechatPayConfig {
	minTopUp := parseWechatPayPositiveInt(values[WechatPayMinTopUpOptionKey], 1)
	maxTopUp := parseWechatPayPositiveInt(values[WechatPayMaxTopUpOptionKey], WechatPayHardMaxTopUp)
	if maxTopUp > WechatPayHardMaxTopUp {
		maxTopUp = WechatPayHardMaxTopUp
	}
	expireMinutes := parseWechatPayPositiveInt(values[WechatPayOrderExpireMinutesOptionKey], 10)
	return WechatPayConfig{
		Enabled:              strings.EqualFold(strings.TrimSpace(values[WechatPayEnabledOptionKey]), "true"),
		AppID:                strings.TrimSpace(values[WechatPayAppIDOptionKey]),
		MchID:                strings.TrimSpace(values[WechatPayMchIDOptionKey]),
		MerchantCertSerialNo: strings.TrimSpace(values[WechatPayMerchantCertSerialNoOptionKey]),
		MerchantPrivateKey:   strings.TrimSpace(values[WechatPayMerchantPrivateKeyOptionKey]),
		APIV3Key:             values[WechatPayAPIV3KeyOptionKey],
		PublicKeyID:          strings.TrimSpace(values[WechatPayPublicKeyIDOptionKey]),
		PublicKey:            strings.TrimSpace(values[WechatPayPublicKeyOptionKey]),
		NotifyURL:            strings.TrimSpace(values[WechatPayNotifyURLOptionKey]),
		MinTopUp:             minTopUp,
		MaxTopUp:             maxTopUp,
		OrderExpireMinutes:   expireMinutes,
	}
}

func parseWechatPayPositiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func WechatPayConfigComplete(config WechatPayConfig) bool {
	return config.AppID != "" &&
		config.MchID != "" &&
		config.MerchantCertSerialNo != "" &&
		config.MerchantPrivateKey != "" &&
		len(config.APIV3Key) == 32 &&
		config.PublicKeyID != "" &&
		config.PublicKey != ""
}
