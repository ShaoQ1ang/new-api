package service

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// AuthenticateBusinessBilling consumes workload-only headers before any provider
// adapter or header template sees them. User authentication is still mandatory.
func AuthenticateBusinessBilling(c *gin.Context) error {
	var orders, credentials []string
	for name, values := range c.Request.Header {
		switch {
		case strings.EqualFold(name, common.BusinessBillingOrderHeader):
			orders = append(orders, values...)
			delete(c.Request.Header, name)
		case strings.EqualFold(name, common.BusinessBillingAuthorizationHeader):
			credentials = append(credentials, values...)
			delete(c.Request.Header, name)
		case strings.HasPrefix(strings.ToLower(name), "x-business-billing-"):
			delete(c.Request.Header, name)
		}
	}
	if len(orders) == 0 && len(credentials) == 0 {
		return nil
	}
	if len(orders) != 1 || len(credentials) != 1 || !validBusinessOrderNumber(orders[0]) {
		return fmt.Errorf("invalid business billing context")
	}
	secret := os.Getenv("BUSINESS_BILLING_CALLER_TOKEN")
	if len(secret) < 32 || secret != strings.TrimSpace(secret) || len(credentials[0]) > 4096 {
		return fmt.Errorf("business billing is unavailable")
	}
	want := sha256.Sum256([]byte("Bearer " + secret))
	got := sha256.Sum256([]byte(credentials[0]))
	if subtle.ConstantTimeCompare(want[:], got[:]) != 1 {
		return fmt.Errorf("business billing caller authentication failed")
	}
	if err := validateCoveredWalletConfig(loadWalletCallbackConfig()); err != nil {
		return err
	}
	// Only synchronous model workflows with a complete reserve/confirm/cancel
	// lifecycle may use a parent order. Async and realtime paths fail closed.
	if c.Request.Method != http.MethodPost {
		return fmt.Errorf("business billing is unsupported for this endpoint")
	}
	switch c.Request.URL.Path {
	case "/v1/chat/completions", "/v1/responses", "/v1/messages", "/v1/embeddings":
	default:
		return fmt.Errorf("business billing is unsupported for this endpoint")
	}
	c.Set(common.BusinessBillingOrderContextKey, orders[0])
	c.Set(common.BusinessBillingAuthenticatedContextKey, true)
	return nil
}

func validBusinessOrderNumber(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, ch := range value {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_' || ch == '.' || ch == ':') {
			return false
		}
	}
	return true
}

func validateCoveredWalletConfig(config walletCallbackConfig) error {
	if !config.Enabled || !config.FailClosed || len(config.Token) < 32 {
		return fmt.Errorf("business billing requires authenticated fail-closed wallet callbacks")
	}
	if config.AutoRetryEnabled {
		return fmt.Errorf("business billing requires manual callback reconciliation")
	}
	endpoint, err := url.Parse(config.BaseURL)
	if err != nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || (endpoint.Path != "" && endpoint.Path != "/") {
		return fmt.Errorf("business billing requires an HTTPS wallet callback origin")
	}
	return nil
}
