package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/aigc/service"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type catalogStub struct {
	group     string
	modelType string
	items     []service.PublicModel
}

func (stub *catalogStub) List(_ context.Context, group, modelType string) ([]service.PublicModel, error) {
	stub.group = group
	stub.modelType = modelType
	return stub.items, nil
}

func TestListModelsUsesAuthenticatedTokenGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	catalog := &catalogStub{items: []service.PublicModel{{
		ID: "writer-pro", Name: "Writer Pro", Type: "text", ConfigVersion: 2,
		Capabilities: json.RawMessage(`{"max_output_tokens":8192}`),
	}}}
	handler := NewModelHandler(catalog)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/aigc/models?type=text&group=forged", nil)
	c.Set(string(constant.ContextKeyTokenGroup), "vip")

	handler.List(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "vip", catalog.group)
	assert.Equal(t, "text", catalog.modelType)
	var response struct {
		Object string                `json:"object"`
		Data   []service.PublicModel `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "list", response.Object)
	require.Len(t, response.Data, 1)
	assert.Equal(t, "writer-pro", response.Data[0].ID)
}
