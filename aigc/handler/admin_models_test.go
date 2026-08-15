package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/aigc/service"
	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type adminCatalogStub struct {
	created service.ProfileInput
	item    *service.AdminProfile
}

func (stub *adminCatalogStub) Create(_ context.Context, input service.ProfileInput) (*service.AdminProfile, error) {
	stub.created = input
	return stub.item, nil
}

func (stub *adminCatalogStub) Get(_ context.Context, _ int64) (*service.AdminProfile, error) {
	return stub.item, nil
}

func (stub *adminCatalogStub) Update(_ context.Context, _ int64, input service.ProfileInput) (*service.AdminProfile, error) {
	return stub.item, nil
}

func (stub *adminCatalogStub) Validate(_ context.Context, _ int64) error       { return nil }
func (stub *adminCatalogStub) Disable(_ context.Context, _ int64, _ int) error { return nil }
func (stub *adminCatalogStub) List(_ context.Context, _ service.ProfileFilter) ([]service.AdminProfile, int64, error) {
	return []service.AdminProfile{*stub.item}, 1, nil
}

type publisherStub struct {
	publicModelID string
	version       int
}

func (stub *publisherStub) Publish(_ context.Context, publicModelID string, expectedVersion int) error {
	stub.publicModelID = publicModelID
	stub.version = expectedVersion
	return nil
}

func TestAdminCreateModelDecodesProfileInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	admin := &adminCatalogStub{item: &service.AdminProfile{ID: 10, PublicModelID: "writer-pro", ConfigVersion: 1}}
	handler := NewAdminModelHandler(admin, &publisherStub{})
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/aigc/models", strings.NewReader(`{
		"public_model_id":"writer-pro","display_name":"Writer Pro","model_type":"text",
		"groups":["vip"],"config":{"text":{"upstream_model_id":"gpt-5"}}
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Create(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "writer-pro", admin.created.PublicModelID)
	assert.Equal(t, []string{"vip"}, admin.created.Groups)
	var response struct {
		Success bool                 `json:"success"`
		Data    service.AdminProfile `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, int64(10), response.Data.ID)
}

func TestAdminPublishUsesStoredPublicIDAndRequestedVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	admin := &adminCatalogStub{item: &service.AdminProfile{ID: 10, PublicModelID: "writer-pro", ConfigVersion: 3}}
	publisher := &publisherStub{}
	handler := NewAdminModelHandler(admin, publisher)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "10"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/aigc/models/10/publish", strings.NewReader(`{"config_version":3}`))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Publish(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "writer-pro", publisher.publicModelID)
	assert.Equal(t, 3, publisher.version)
}

func TestAdminProfileJSONKeepsConfigAsObject(t *testing.T) {
	item := service.AdminProfile{Config: json.RawMessage(`{"text":{"upstream_model_id":"gpt-5"}}`)}
	contents, err := common.Marshal(item)
	require.NoError(t, err)
	assert.Contains(t, string(contents), `"config":{"text"`)
}
