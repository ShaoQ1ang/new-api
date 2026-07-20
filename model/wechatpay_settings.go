package model

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"gorm.io/gorm"
)

var wechatPayOptionUpdateMutex sync.Mutex

func isWechatPayOptionKey(key string) bool {
	for _, allowedKey := range setting.WechatPayOptionKeys {
		if key == allowedKey {
			return true
		}
	}
	return false
}

func publishWechatPayRuntimeOptions(values map[string]string) {
	common.OptionMapRWMutex.Lock()
	defer common.OptionMapRWMutex.Unlock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	for key, value := range values {
		common.OptionMap[key] = value
	}
}

func UpdateWechatPayOptions(values map[string]string) error {
	return updateWechatPayOptions(values, validateWechatPayOptionBounds)
}

func UpdateWechatPayOptionsWithValidation(values map[string]string, validate func(setting.WechatPayConfig) error) error {
	return updateWechatPayOptions(values, validate)
}

func updateWechatPayOptions(values map[string]string, validate func(setting.WechatPayConfig) error) error {
	if len(values) == 0 {
		return nil
	}
	wechatPayOptionUpdateMutex.Lock()
	defer wechatPayOptionUpdateMutex.Unlock()

	allowed := make(map[string]struct{}, len(setting.WechatPayOptionKeys))
	for _, key := range setting.WechatPayOptionKeys {
		allowed[key] = struct{}{}
	}
	for key := range values {
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("unsupported WeChat Pay option: %s", key)
		}
	}
	if err := validateWechatPayOptionValueSyntax(values); err != nil {
		return err
	}

	runtimeValues := make(map[string]string, len(setting.WechatPayOptionKeys))
	if err := DB.Transaction(func(tx *gorm.DB) error {
		// Every writer updates the same non-sensitive row first. This creates a
		// portable serialization point for SQLite, MySQL, and PostgreSQL before
		// reading the current configuration snapshot.
		lockOption := Option{Key: setting.WechatPayEnabledOptionKey, Value: "false"}
		keyColumn := "`key`"
		valueColumn := "`value`"
		if common.UsingPostgreSQL {
			keyColumn = `"key"`
			valueColumn = `"value"`
		}
		if err := tx.Where(keyColumn+" = ?", lockOption.Key).FirstOrCreate(&lockOption).Error; err != nil {
			return err
		}
		if err := tx.Model(&Option{}).
			Where(keyColumn+" = ?", lockOption.Key).
			UpdateColumn("value", gorm.Expr(valueColumn)).Error; err != nil {
			return err
		}

		for _, key := range setting.WechatPayOptionKeys {
			runtimeValues[key] = ""
		}
		var currentOptions []Option
		if err := tx.Where(keyColumn+" IN ?", setting.WechatPayOptionKeys).Find(&currentOptions).Error; err != nil {
			return err
		}
		for _, option := range currentOptions {
			value := option.Value
			if common.IsSensitiveOptionKey(option.Key) {
				plainText, err := common.DecryptOptionValue(option.Key, value)
				if err != nil {
					return err
				}
				value = plainText
			}
			runtimeValues[option.Key] = value
		}
		for key, value := range values {
			runtimeValues[key] = value
		}
		if validate != nil {
			if err := validate(setting.WechatPayConfigFromOptions(runtimeValues)); err != nil {
				return err
			}
		}

		valuesToPersist := make(map[string]string, len(values)+3)
		for key, value := range values {
			valuesToPersist[key] = value
		}
		// Opportunistically migrate manually seeded or legacy plaintext
		// credentials whenever the dedicated settings transaction succeeds.
		for _, key := range []string{
			setting.WechatPayMerchantPrivateKeyOptionKey,
			setting.WechatPayAPIV3KeyOptionKey,
			setting.WechatPayPublicKeyOptionKey,
		} {
			if _, explicitlyUpdated := valuesToPersist[key]; !explicitlyUpdated && runtimeValues[key] != "" {
				valuesToPersist[key] = runtimeValues[key]
			}
		}

		for key, value := range valuesToPersist {
			valueToPersist := value
			if common.IsSensitiveOptionKey(key) {
				encryptedValue, err := common.EncryptOptionValue(key, value)
				if err != nil {
					return err
				}
				valueToPersist = encryptedValue
			}

			option := Option{Key: key}
			if err := tx.FirstOrCreate(&option, Option{Key: key}).Error; err != nil {
				return err
			}
			option.Value = valueToPersist
			if err := tx.Save(&option).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}

	publishWechatPayRuntimeOptions(runtimeValues)
	return nil
}

func validateWechatPayOptionValueSyntax(values map[string]string) error {
	for key, raw := range values {
		var minValue, maxValue int
		switch key {
		case setting.WechatPayMinTopUpOptionKey:
			minValue = 1
		case setting.WechatPayMaxTopUpOptionKey:
			minValue, maxValue = 1, setting.WechatPayHardMaxTopUp
		case setting.WechatPayOrderExpireMinutesOptionKey:
			minValue, maxValue = 1, 120
		default:
			continue
		}
		value, err := strconv.Atoi(raw)
		if err != nil || value < minValue || (maxValue > 0 && value > maxValue) {
			return fmt.Errorf("invalid WeChat Pay numeric option %s", key)
		}
	}
	return nil
}

func validateWechatPayOptionBounds(config setting.WechatPayConfig) error {
	if config.MinTopUp <= 0 || config.MaxTopUp < config.MinTopUp || config.MaxTopUp > setting.WechatPayHardMaxTopUp {
		return fmt.Errorf("invalid WeChat Pay top-up bounds")
	}
	if config.OrderExpireMinutes < 1 || config.OrderExpireMinutes > 120 {
		return fmt.Errorf("invalid WeChat Pay order expiration")
	}
	return nil
}
