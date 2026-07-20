package service

import (
	"bytes"
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/require"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
)

func ptr[T any](value T) *T {
	return &value
}

func validWechatPayConfigForTest(t *testing.T) setting.WechatPayConfig {
	t.Helper()
	config, _ := validWechatPayConfigAndPlatformKeyForTest(t)
	return config
}

func validWechatPayConfigAndPlatformKeyForTest(t *testing.T) (setting.WechatPayConfig, *rsa.PrivateKey) {
	t.Helper()
	merchantPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	merchantPrivateDER, err := x509.MarshalPKCS8PrivateKey(merchantPrivateKey)
	require.NoError(t, err)
	platformPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	platformPublicDER, err := x509.MarshalPKIXPublicKey(&platformPrivateKey.PublicKey)
	require.NoError(t, err)

	return setting.WechatPayConfig{
		Enabled:              true,
		AppID:                "wx-test-app-id",
		MchID:                "1900000109",
		MerchantCertSerialNo: "merchant-serial",
		MerchantPrivateKey: string(pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: merchantPrivateDER,
		})),
		APIV3Key:    "12345678901234567890123456789012",
		PublicKeyID: "PUB_KEY_ID_test",
		PublicKey: string(pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: platformPublicDER,
		})),
		NotifyURL:          "https://pay.example.com/api/wechatpay/notify",
		MinTopUp:           1,
		MaxTopUp:           4000,
		OrderExpireMinutes: 10,
	}, platformPrivateKey
}

func validWechatPayTransactionForTest() (*model.TopUp, *payments.Transaction) {
	topUp := &model.TopUp{
		TradeNo: "wx-test-trade",
		Money:   12.34,
	}
	transaction := &payments.Transaction{
		Appid:         ptr("wx-test-app-id"),
		Mchid:         ptr("1900000109"),
		OutTradeNo:    ptr("wx-test-trade"),
		TradeType:     ptr("NATIVE"),
		TradeState:    ptr("SUCCESS"),
		TransactionId: ptr("4200000000000000001"),
		Amount: &payments.TransactionAmount{
			Total:    ptr[int64](1234),
			Currency: ptr("CNY"),
		},
	}
	return topUp, transaction
}

func TestValidateWechatPayConfig(t *testing.T) {
	config := validWechatPayConfigForTest(t)
	require.NoError(t, ValidateWechatPayConfig(config))

	config.APIV3Key = "too-short"
	require.ErrorContains(t, ValidateWechatPayConfig(config), "32 bytes")

	config = validWechatPayConfigForTest(t)
	config.NotifyURL = "http://pay.example.com/api/wechatpay/notify?token=secret"
	require.ErrorContains(t, ValidateWechatPayConfig(config), "HTTPS")

	config = validWechatPayConfigForTest(t)
	merchantPrivateKey, err := utils.LoadPrivateKey(config.MerchantPrivateKey)
	require.NoError(t, err)
	merchantPublicDER, err := x509.MarshalPKIXPublicKey(&merchantPrivateKey.PublicKey)
	require.NoError(t, err)
	config.PublicKey = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: merchantPublicDER}))
	require.ErrorContains(t, ValidateWechatPayConfig(config), "must not be the merchant private key")
}

func TestValidateWechatPayTransaction(t *testing.T) {
	config := validWechatPayConfigForTest(t)
	topUp, transaction := validWechatPayTransactionForTest()
	require.NoError(t, ValidateWechatPayTransaction(config, topUp, transaction))

	tests := []struct {
		name   string
		mutate func(*payments.Transaction)
		target error
	}{
		{name: "app id", mutate: func(tx *payments.Transaction) { tx.Appid = ptr("wrong") }},
		{name: "merchant id", mutate: func(tx *payments.Transaction) { tx.Mchid = ptr("wrong") }},
		{name: "trade number", mutate: func(tx *payments.Transaction) { tx.OutTradeNo = ptr("wrong") }},
		{name: "trade type", mutate: func(tx *payments.Transaction) { tx.TradeType = ptr("JSAPI") }},
		{name: "transaction id", mutate: func(tx *payments.Transaction) { tx.TransactionId = ptr("") }},
		{name: "amount", mutate: func(tx *payments.Transaction) { tx.Amount.Total = ptr[int64](1233) }, target: model.ErrWechatPayAmountMismatch},
		{name: "currency", mutate: func(tx *payments.Transaction) { tx.Amount.Currency = ptr("USD") }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, candidate := validWechatPayTransactionForTest()
			test.mutate(candidate)
			err := ValidateWechatPayTransaction(config, topUp, candidate)
			require.Error(t, err)
			if test.target != nil {
				require.ErrorIs(t, err, test.target)
			}
		})
	}
}

func TestNewWechatPayTradeNo(t *testing.T) {
	seen := make(map[string]struct{}, 100)
	for range 100 {
		tradeNo, err := NewWechatPayTradeNo()
		require.NoError(t, err)
		require.LessOrEqual(t, len(tradeNo), 32)
		require.Regexp(t, `^[a-z0-9]+$`, tradeNo)
		_, duplicate := seen[tradeNo]
		require.False(t, duplicate)
		seen[tradeNo] = struct{}{}
	}
}

func TestNextWechatPayPendingRetryTimeIsBounded(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	require.Equal(t, base.Add(30*time.Second).Unix(), NextWechatPayPendingRetryTime(base, -1))
	require.Equal(t, base.Add(30*time.Second).Unix(), NextWechatPayPendingRetryTime(base, 0))
	require.Equal(t, base.Add(time.Minute).Unix(), NextWechatPayPendingRetryTime(base, 1))
	require.Equal(t, base.Add(30*time.Minute).Unix(), NextWechatPayPendingRetryTime(base, 6))
	require.Equal(t, base.Add(30*time.Minute).Unix(), NextWechatPayPendingRetryTime(base, 100))
}

func buildWechatPayNotifyRequestForTest(t *testing.T, config setting.WechatPayConfig, signingKey *rsa.PrivateKey, tamperSignature bool, signedAt ...time.Time) *http.Request {
	t.Helper()
	_, transaction := validWechatPayTransactionForTest()
	plaintext, err := common.Marshal(transaction)
	require.NoError(t, err)

	block, err := aes.NewCipher([]byte(config.APIV3Key))
	require.NoError(t, err)
	aead, err := cipher.NewGCM(block)
	require.NoError(t, err)
	nonce := []byte("0123456789ab")
	associatedData := "transaction"
	ciphertext := aead.Seal(nil, nonce, plaintext, []byte(associatedData))
	body, err := common.Marshal(map[string]any{
		"id":            "notify-id",
		"create_time":   time.Now().UTC().Format(time.RFC3339),
		"event_type":    "TRANSACTION.SUCCESS",
		"resource_type": "encrypt-resource",
		"summary":       "payment successful",
		"resource": map[string]string{
			"algorithm":       "AEAD_AES_256_GCM",
			"ciphertext":      base64.StdEncoding.EncodeToString(ciphertext),
			"nonce":           string(nonce),
			"associated_data": associatedData,
		},
	})
	require.NoError(t, err)

	timestampTime := time.Now()
	if len(signedAt) > 0 {
		timestampTime = signedAt[0]
	}
	timestamp := fmt.Sprintf("%d", timestampTime.Unix())
	signNonce := "notify-sign-nonce"
	message := timestamp + "\n" + signNonce + "\n" + string(body) + "\n"
	digest := sha256.Sum256([]byte(message))
	signature, err := rsa.SignPKCS1v15(rand.Reader, signingKey, crypto.SHA256, digest[:])
	require.NoError(t, err)
	if tamperSignature {
		signature[len(signature)-1] ^= 0x01
	}

	request := httptest.NewRequest(http.MethodPost, "/api/wechatpay/notify", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Wechatpay-Serial", config.PublicKeyID)
	request.Header.Set("Wechatpay-Signature", base64.StdEncoding.EncodeToString(signature))
	request.Header.Set("Wechatpay-Timestamp", timestamp)
	request.Header.Set("Wechatpay-Nonce", signNonce)
	return request
}

func TestParseWechatPayNotifyVerifiesSignatureAndDecryptsTransaction(t *testing.T) {
	config, platformPrivateKey := validWechatPayConfigAndPlatformKeyForTest(t)
	request := buildWechatPayNotifyRequestForTest(t, config, platformPrivateKey, false)

	transaction, err := ParseWechatPayNotify(context.Background(), request, config)
	require.NoError(t, err)
	require.NotNil(t, transaction)
	require.Equal(t, "wx-test-trade", *transaction.OutTradeNo)
	require.Equal(t, int64(1234), *transaction.Amount.Total)
}

func TestParseWechatPayNotifyRejectsTamperedSignature(t *testing.T) {
	config, platformPrivateKey := validWechatPayConfigAndPlatformKeyForTest(t)
	request := buildWechatPayNotifyRequestForTest(t, config, platformPrivateKey, true)

	transaction, err := ParseWechatPayNotify(context.Background(), request, config)
	require.Error(t, err)
	require.Nil(t, transaction)
}

func TestParseWechatPayNotifyRejectsMerchantSignedRequest(t *testing.T) {
	config, _ := validWechatPayConfigAndPlatformKeyForTest(t)
	merchantPrivateKey, err := utils.LoadPrivateKey(config.MerchantPrivateKey)
	require.NoError(t, err)
	request := buildWechatPayNotifyRequestForTest(t, config, merchantPrivateKey, false)

	transaction, err := ParseWechatPayNotify(context.Background(), request, config)
	require.Error(t, err)
	require.Nil(t, transaction)
}

func TestParseWechatPayNotifyRejectsExpiredSignature(t *testing.T) {
	config, platformPrivateKey := validWechatPayConfigAndPlatformKeyForTest(t)
	request := buildWechatPayNotifyRequestForTest(t, config, platformPrivateKey, false, time.Now().Add(-10*time.Minute))

	transaction, err := ParseWechatPayNotify(context.Background(), request, config)
	require.Error(t, err)
	require.Nil(t, transaction)
}
