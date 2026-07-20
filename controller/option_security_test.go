package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func initOptionTestDBForController(t *testing.T) {
	t.Helper()

	dsn := fmt.Sprintf("file:test_option_security_%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Option{}, &model.TopUp{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
	})
}

func TestUpdateOptionReturnsMissingCryptoKeyErrorForAlipaySecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initOptionTestDBForController(t)
	t.Setenv("OPTION_CRYPT_KEY", "")

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/option/",
		bytes.NewBufferString(`{"key":"AlipayPrivateKey","value":"private-value"}`),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	UpdateOption(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":false`)
	require.Contains(t, recorder.Body.String(), "OPTION_CRYPT_KEY is required")
	require.NotContains(t, recorder.Body.String(), "private-value")
}

func TestGetOptionsOmitsProtectedAlipayKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initOptionTestDBForController(t)

	common.OptionMapRWMutex.Lock()
	originalMap := common.OptionMap
	common.OptionMap = map[string]string{
		"AlipayPrivateKey": "private-value",
		"AlipayPublicKey":  "public-value",
		"AlipayGateway":    "https://openapi.alipay.com/gateway.do",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalMap
		common.OptionMapRWMutex.Unlock()
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	GetOptions(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "AlipayPrivateKey")
	require.NotContains(t, recorder.Body.String(), "AlipayPublicKey")
	require.Contains(t, recorder.Body.String(), "AlipayGateway")
}

func TestGetOptionsOmitsProtectedWechatPayKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initOptionTestDBForController(t)

	common.OptionMapRWMutex.Lock()
	originalMap := common.OptionMap
	common.OptionMap = map[string]string{
		"WechatPayMerchantPrivateKey": "private-value",
		"WechatPayAPIV3Key":           "api-v3-value",
		"WechatPayPublicKey":          "public-value",
		"WechatPayAppID":              "wx-visible-app-id",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalMap
		common.OptionMapRWMutex.Unlock()
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	GetOptions(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "WechatPayMerchantPrivateKey")
	require.NotContains(t, recorder.Body.String(), "WechatPayAPIV3Key")
	require.NotContains(t, recorder.Body.String(), "WechatPayPublicKey")
	require.NotContains(t, recorder.Body.String(), "private-value")
	require.NotContains(t, recorder.Body.String(), "api-v3-value")
	require.NotContains(t, recorder.Body.String(), "public-value")
	require.Contains(t, recorder.Body.String(), "WechatPayAppID")
	require.Contains(t, recorder.Body.String(), "wx-visible-app-id")
}

func TestClearWechatPaySecretsDisablesNewOrders(t *testing.T) {
	config := setting.WechatPayConfig{
		Enabled:            true,
		MerchantPrivateKey: "private-value",
		APIV3Key:           "api-v3-value",
		PublicKey:          "public-value",
	}
	updates := make(map[string]string)

	require.NoError(t, clearWechatPaySecrets(&config, updates, []string{"api_v3_key"}))
	require.False(t, config.Enabled)
	require.Empty(t, config.APIV3Key)
	require.Equal(t, "false", updates[setting.WechatPayEnabledOptionKey])
	require.Equal(t, "", updates[setting.WechatPayAPIV3KeyOptionKey])
}

func TestGetWechatPayAdminSettingsNeverReturnsCredentialPlaintext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initOptionTestDBForController(t)

	common.OptionMapRWMutex.Lock()
	originalMap := common.OptionMap
	common.OptionMap = map[string]string{
		setting.WechatPayEnabledOptionKey:            "true",
		setting.WechatPayAppIDOptionKey:              "wx-visible-app-id",
		setting.WechatPayMerchantPrivateKeyOptionKey: "private-credential-plaintext",
		setting.WechatPayAPIV3KeyOptionKey:           "api-v3-credential-plaintext-value",
		setting.WechatPayPublicKeyOptionKey:          "public-credential-plaintext",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalMap
		common.OptionMapRWMutex.Unlock()
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	GetWechatPayAdminSettings(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "wx-visible-app-id")
	require.Contains(t, recorder.Body.String(), `"merchant_private_key_configured":true`)
	require.Contains(t, recorder.Body.String(), `"api_v3_key_configured":true`)
	require.Contains(t, recorder.Body.String(), `"public_key_configured":true`)
	require.NotContains(t, recorder.Body.String(), "private-credential-plaintext")
	require.NotContains(t, recorder.Body.String(), "api-v3-credential-plaintext-value")
	require.NotContains(t, recorder.Body.String(), "public-credential-plaintext")
}
