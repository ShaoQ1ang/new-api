package model

import (
	"fmt"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/require"
)

func resetWechatPayOptionState(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Option{}))
	require.NoError(t, DB.Where("key IN ?", setting.WechatPayOptionKeys).Delete(&Option{}).Error)
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	for _, key := range setting.WechatPayOptionKeys {
		common.OptionMap[key] = ""
	}
	common.OptionMapRWMutex.Unlock()
}

func TestWechatPayOptionCryptoRoundTrip(t *testing.T) {
	t.Setenv("OPTION_CRYPT_KEY", "test-wechatpay-option-crypt-key")

	for _, key := range []string{
		setting.WechatPayMerchantPrivateKeyOptionKey,
		setting.WechatPayAPIV3KeyOptionKey,
		setting.WechatPayPublicKeyOptionKey,
	} {
		encrypted, err := common.EncryptOptionValue(key, "secret-value")
		require.NoError(t, err)
		require.True(t, common.IsEncryptedOptionValue(encrypted))

		decrypted, err := common.DecryptOptionValue(key, encrypted)
		require.NoError(t, err)
		require.Equal(t, "secret-value", decrypted)
	}
}

func TestUpdateWechatPayOptionsEncryptsSecretsAndKeepsRuntimePlaintext(t *testing.T) {
	resetWechatPayOptionState(t)
	t.Setenv("OPTION_CRYPT_KEY", "test-wechatpay-option-crypt-key")

	updates := map[string]string{
		setting.WechatPayAppIDOptionKey:              "wx-test-app-id",
		setting.WechatPayMerchantPrivateKeyOptionKey: "private-key-plaintext",
		setting.WechatPayAPIV3KeyOptionKey:           "12345678901234567890123456789012",
	}
	require.NoError(t, UpdateWechatPayOptions(updates))

	for key, expected := range updates {
		var persisted Option
		require.NoError(t, DB.Where("key = ?", key).First(&persisted).Error)
		if common.IsSensitiveOptionKey(key) {
			require.True(t, common.IsEncryptedOptionValue(persisted.Value))
			require.NotEqual(t, expected, persisted.Value)
		} else {
			require.Equal(t, expected, persisted.Value)
		}

		common.OptionMapRWMutex.RLock()
		runtimeValue := common.OptionMap[key]
		common.OptionMapRWMutex.RUnlock()
		require.Equal(t, expected, runtimeValue)
	}
}

func TestUpdateWechatPayOptionsMigratesExistingPlaintextSecret(t *testing.T) {
	resetWechatPayOptionState(t)
	t.Setenv("OPTION_CRYPT_KEY", "test-wechatpay-option-crypt-key")
	require.NoError(t, DB.Create(&Option{
		Key:   setting.WechatPayAPIV3KeyOptionKey,
		Value: "12345678901234567890123456789012",
	}).Error)

	require.NoError(t, UpdateWechatPayOptions(map[string]string{
		setting.WechatPayAppIDOptionKey: "wx-migrate-plaintext",
	}))

	var persisted Option
	require.NoError(t, DB.Where("key = ?", setting.WechatPayAPIV3KeyOptionKey).First(&persisted).Error)
	require.True(t, common.IsEncryptedOptionValue(persisted.Value))
}

func TestUpdateWechatPayOptionsRollsBackWhenSecretCannotBeEncrypted(t *testing.T) {
	resetWechatPayOptionState(t)
	t.Setenv("OPTION_CRYPT_KEY", "")

	err := UpdateWechatPayOptions(map[string]string{
		setting.WechatPayAppIDOptionKey:    "must-not-be-persisted",
		setting.WechatPayAPIV3KeyOptionKey: "12345678901234567890123456789012",
		setting.WechatPayMinTopUpOptionKey: "9",
		setting.WechatPayEnabledOptionKey:  "true",
	})
	require.ErrorContains(t, err, "OPTION_CRYPT_KEY is required")

	var count int64
	require.NoError(t, DB.Model(&Option{}).Where("key IN ?", setting.WechatPayOptionKeys).Count(&count).Error)
	require.Zero(t, count)
}

func TestWechatPayRuntimeOptionsPublishAsOneSnapshot(t *testing.T) {
	resetWechatPayOptionState(t)
	first := map[string]string{
		setting.WechatPayAppIDOptionKey: "app-a",
		setting.WechatPayMchIDOptionKey: "merchant-a",
	}
	second := map[string]string{
		setting.WechatPayAppIDOptionKey: "app-b",
		setting.WechatPayMchIDOptionKey: "merchant-b",
	}
	require.NoError(t, UpdateWechatPayOptions(first))

	start := make(chan struct{})
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 50; i++ {
			values := first
			if i%2 == 0 {
				values = second
			}
			if err := UpdateWechatPayOptions(values); err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 5000; i++ {
			config := setting.GetWechatPayConfig()
			if (config.AppID == "app-a" && config.MchID == "merchant-a") ||
				(config.AppID == "app-b" && config.MchID == "merchant-b") {
				continue
			}
			select {
			case errCh <- fmt.Errorf("torn runtime snapshot: app_id=%q mch_id=%q", config.AppID, config.MchID):
			default:
			}
			return
		}
	}()
	close(start)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}
}

func TestGenericOptionUpdateRejectsWechatPayKeys(t *testing.T) {
	require.ErrorContains(t, UpdateOption(setting.WechatPayAppIDOptionKey, "bypass"), "dedicated settings API")
}

func TestWechatPayOptionUpdateRejectsInvalidNumericValues(t *testing.T) {
	for key, value := range map[string]string{
		setting.WechatPayMinTopUpOptionKey:           "0",
		setting.WechatPayMaxTopUpOptionKey:           "4001",
		setting.WechatPayOrderExpireMinutesOptionKey: "121",
	} {
		require.Error(t, UpdateWechatPayOptions(map[string]string{key: value}))
	}
}

func TestConcurrentPartialWechatPayUpdatesCannotCommitInvalidCombination(t *testing.T) {
	resetWechatPayOptionState(t)
	require.NoError(t, UpdateWechatPayOptions(map[string]string{
		setting.WechatPayMinTopUpOptionKey: "1",
		setting.WechatPayMaxTopUpOptionKey: "4000",
	}))
	validate := func(config setting.WechatPayConfig) error {
		if config.MaxTopUp < config.MinTopUp {
			return fmt.Errorf("maximum is below minimum")
		}
		return nil
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	for _, update := range []map[string]string{
		{setting.WechatPayMinTopUpOptionKey: "3000"},
		{setting.WechatPayMaxTopUpOptionKey: "1000"},
	} {
		update := update
		go func() {
			<-start
			results <- UpdateWechatPayOptionsWithValidation(update, validate)
		}()
	}
	close(start)

	successes := 0
	failures := 0
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			failures++
		} else {
			successes++
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, failures)
	config := setting.GetWechatPayConfig()
	require.GreaterOrEqual(t, config.MaxTopUp, config.MinTopUp)
}
