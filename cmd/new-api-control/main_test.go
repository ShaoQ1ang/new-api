package main

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	aigcservice "github.com/QuantumNous/new-api/aigc/service"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pricingCatalogStub struct {
	document aigcservice.PublicPricingDocument
	err      error
	group    string
	calls    int
}

func (stub *pricingCatalogStub) List(_ context.Context, group string) (aigcservice.PublicPricingDocument, error) {
	stub.calls++
	stub.group = group
	return stub.document, stub.err
}

type controlPricingResponse struct {
	Success bool                   `json:"success"`
	Data    controlPricingDocument `json:"data"`
	Error   *apiError              `json:"error"`
}

func TestControlPricingRouteReturnsPricingForPersistedGroup(t *testing.T) {
	catalog := &pricingCatalogStub{document: aigcservice.PublicPricingDocument{
		PricingVersion: "sha256:source", Currency: "USD", QuotaPerUnit: "500000.000000",
		UserGroup: "vip", EffectiveGroupRatio: "1.500000",
		Models: []aigcservice.PublicModelPricing{{
			ModelID: "image-model", MediaType: "image", Routes: []aigcservice.PublicPricingRoute{{
				Mode: "text_to_image", BillingUnit: "generation", GenerationPrice: "0.150000",
			}},
		}},
	}}
	var resolvedIAMUserID, resolvedIdentityVersion int64
	var loadedUserID int
	var loadedFromDB bool
	handler := routesWithPricingDependencies(pricingRouteDependencies{
		resolveActiveIAMIdentity: func(iamUserID, minimumVersion int64) (model.IAMIdentityLink, error) {
			resolvedIAMUserID, resolvedIdentityVersion = iamUserID, minimumVersion
			return model.IAMIdentityLink{NewAPIUserID: 37}, nil
		},
		getUserGroup: func(userID int, fromDB bool) (string, error) {
			loadedUserID, loadedFromDB = userID, fromDB
			return " vip ", nil
		},
		catalog:         catalog,
		usdExchangeRate: func() float64 { return 7.25 },
	})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/internal/v1/aigc/pricing?iam_user_id=23&identity_version=9", nil))

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	assert.Empty(t, response.Header().Get("Authorization"))
	assert.Empty(t, response.Header().Get("New-Api-User"))
	var payload controlPricingResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Nil(t, payload.Error)
	assert.Equal(t, int64(23), resolvedIAMUserID)
	assert.Equal(t, int64(9), resolvedIdentityVersion)
	assert.Equal(t, 37, loadedUserID)
	assert.True(t, loadedFromDB)
	assert.Equal(t, "vip", catalog.group)
	assert.Equal(t, "7.25", payload.Data.USDToCNYRate)
	assert.Equal(t, "1.500000", payload.Data.EffectiveGroupRatio)
	require.Len(t, payload.Data.Models, 1)
	assert.Equal(t, "0.150000", payload.Data.Models[0].Routes[0].GenerationPrice)
}

func TestControlPricingRouteRejectsInactiveOrStaleIdentity(t *testing.T) {
	catalog := &pricingCatalogStub{}
	var resolvedIdentityVersion int64
	handler := routesWithPricingDependencies(pricingRouteDependencies{
		resolveActiveIAMIdentity: func(_ int64, minimumVersion int64) (model.IAMIdentityLink, error) {
			resolvedIdentityVersion = minimumVersion
			return model.IAMIdentityLink{}, model.ErrIAMIdentityNotActive
		},
		getUserGroup:    model.GetUserGroup,
		catalog:         catalog,
		usdExchangeRate: func() float64 { return 7.3 },
	})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/internal/v1/aigc/pricing?iam_user_id=23&identity_version=10", nil))

	require.Equal(t, http.StatusConflict, response.Code)
	var payload controlPricingResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	require.NotNil(t, payload.Error)
	assert.Equal(t, "IDENTITY_NOT_ACTIVE", payload.Error.Code)
	assert.Equal(t, int64(10), resolvedIdentityVersion)
	assert.Zero(t, catalog.calls)
}

func TestControlPricingRouteRejectsAutoGroup(t *testing.T) {
	catalog := &pricingCatalogStub{}
	handler := routesWithPricingDependencies(pricingRouteDependencies{
		resolveActiveIAMIdentity: func(_ int64, _ int64) (model.IAMIdentityLink, error) {
			return model.IAMIdentityLink{NewAPIUserID: 37}, nil
		},
		getUserGroup:    func(_ int, _ bool) (string, error) { return "auto", nil },
		catalog:         catalog,
		usdExchangeRate: func() float64 { return 7.3 },
	})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/internal/v1/aigc/pricing?iam_user_id=23&identity_version=9", nil))

	require.Equal(t, http.StatusConflict, response.Code)
	var payload controlPricingResponse
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	require.NotNil(t, payload.Error)
	assert.Equal(t, "AUTO_GROUP_UNSUPPORTED", payload.Error.Code)
	assert.Zero(t, catalog.calls)
}

func TestControlPricingRouteRejectsIdentityHeaders(t *testing.T) {
	for _, header := range []string{"Authorization", "Cookie", "New-Api-User", "Session"} {
		t.Run(header, func(t *testing.T) {
			catalog := &pricingCatalogStub{}
			handler := routesWithPricingDependencies(pricingRouteDependencies{
				resolveActiveIAMIdentity: func(_ int64, _ int64) (model.IAMIdentityLink, error) {
					return model.IAMIdentityLink{NewAPIUserID: 37}, nil
				},
				getUserGroup:    func(_ int, _ bool) (string, error) { return "default", nil },
				catalog:         catalog,
				usdExchangeRate: func() float64 { return 7.3 },
			})
			request := httptest.NewRequest(http.MethodGet,
				"/internal/v1/aigc/pricing?iam_user_id=23&identity_version=9", nil)
			request.Header.Set(header, "unexpected")
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			require.Equal(t, http.StatusBadRequest, response.Code)
			var payload controlPricingResponse
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
			require.NotNil(t, payload.Error)
			assert.Equal(t, "INVALID_ARGUMENT", payload.Error.Code)
			assert.Zero(t, catalog.calls)
		})
	}
}

func TestControlPricingRouteFailsClosedForInvalidExchangeRate(t *testing.T) {
	for name, rate := range map[string]float64{
		"zero": 0, "negative": -1, "nan": math.NaN(), "infinity": math.Inf(1),
	} {
		t.Run(name, func(t *testing.T) {
			catalog := &pricingCatalogStub{}
			handler := routesWithPricingDependencies(pricingRouteDependencies{
				resolveActiveIAMIdentity: func(_ int64, _ int64) (model.IAMIdentityLink, error) {
					return model.IAMIdentityLink{NewAPIUserID: 37}, nil
				},
				getUserGroup:    func(_ int, _ bool) (string, error) { return "default", nil },
				catalog:         catalog,
				usdExchangeRate: func() float64 { return rate },
			})
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet,
				"/internal/v1/aigc/pricing?iam_user_id=23&identity_version=9", nil))

			require.Equal(t, http.StatusInternalServerError, response.Code)
			var payload controlPricingResponse
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
			require.NotNil(t, payload.Error)
			assert.Equal(t, "PRICING_UNAVAILABLE", payload.Error.Code)
		})
	}
}

func TestControlOptionSyncUsesConfiguredFrequencyAndStops(t *testing.T) {
	originalFrequency := common.SyncFrequency
	common.SyncFrequency = 17
	t.Cleanup(func() { common.SyncFrequency = originalFrequency })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	started := make(chan int, 1)
	done := startControlOptionSync(ctx, func(ctx context.Context, frequency int) {
		started <- frequency
		<-ctx.Done()
	})

	select {
	case frequency := <-started:
		assert.Equal(t, 17, frequency)
	case <-time.After(time.Second):
		require.FailNow(t, "option sync did not start")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		require.FailNow(t, "option sync did not stop")
	}
}

func TestControlPricingRouteReturnsCatalogFailureWithoutDetails(t *testing.T) {
	catalog := &pricingCatalogStub{err: errors.New("private catalog failure")}
	handler := routesWithPricingDependencies(pricingRouteDependencies{
		resolveActiveIAMIdentity: func(_ int64, _ int64) (model.IAMIdentityLink, error) {
			return model.IAMIdentityLink{NewAPIUserID: 37}, nil
		},
		getUserGroup:    func(_ int, _ bool) (string, error) { return "default", nil },
		catalog:         catalog,
		usdExchangeRate: func() float64 { return 7.3 },
	})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		"/internal/v1/aigc/pricing?iam_user_id=23&identity_version=9", nil))

	require.Equal(t, http.StatusInternalServerError, response.Code)
	assert.NotContains(t, response.Body.String(), "private catalog failure")
}
