package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/aigc/service"
	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type upstreamCatalogStub struct {
	items []service.UpstreamModel
	item  *service.UpstreamModel
	id    string
}

func (stub *upstreamCatalogStub) List(_ context.Context) ([]service.UpstreamModel, error) {
	return stub.items, nil
}

func (stub *upstreamCatalogStub) Get(_ context.Context, id string) (*service.UpstreamModel, error) {
	stub.id = id
	return stub.item, nil
}

func TestUpstreamModelsListUsesStandardAPIEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	catalog := &upstreamCatalogStub{items: []service.UpstreamModel{{ID: "video-pro", ChannelCount: 2}}}
	handler := NewUpstreamModelHandler(catalog)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/aigc/upstream-models", nil)

	handler.List(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool                    `json:"success"`
		Data    []service.UpstreamModel `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	require.Len(t, response.Data, 1)
	assert.Equal(t, "video-pro", response.Data[0].ID)
}

func TestUpstreamModelsGetDecodesEscapedModelID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	catalog := &upstreamCatalogStub{item: &service.UpstreamModel{ID: "openai/gpt-image-1"}}
	handler := NewUpstreamModelHandler(catalog)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "openai%2Fgpt-image-1"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/aigc/upstream-models/openai%2Fgpt-image-1", nil)

	handler.Get(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "openai/gpt-image-1", catalog.id)
}
