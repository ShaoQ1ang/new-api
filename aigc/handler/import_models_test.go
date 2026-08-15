package handler

import (
	"context"
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

type profileImporterStub struct {
	items []service.LegacyProfileInput
}

func (stub *profileImporterStub) Import(_ context.Context, items []service.LegacyProfileInput) (*service.ProfileImportReport, error) {
	stub.items = items
	return &service.ProfileImportReport{Total: len(items), Imported: len(items)}, nil
}

func TestProfileImportHandlerDecodesLegacyRows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	importer := &profileImporterStub{}
	handler := NewProfileImportHandler(importer)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/aigc/models/import", strings.NewReader(`{
		"items":[{"model_id":"writer-pro","model_type":"text","display_name":"Writer Pro","enabled":true,"config_json":"{\"upstream_model_id\":\"gpt-5\"}"}]
	}`))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Import(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Len(t, importer.items, 1)
	assert.Equal(t, "writer-pro", importer.items[0].ModelID)
	var response struct {
		Success bool                        `json:"success"`
		Data    service.ProfileImportReport `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, 1, response.Data.Imported)
}
