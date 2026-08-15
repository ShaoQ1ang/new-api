package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/aigc/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type importUpstreamCatalogStub struct {
	items []UpstreamModel
	err   error
}

func (stub importUpstreamCatalogStub) List(_ context.Context) ([]UpstreamModel, error) {
	return stub.items, stub.err
}

func TestProfileImportTransformsLegacyTextAndPublishesEligibleModel(t *testing.T) {
	_, profiles := newAdminService(t)
	importer := NewProfileImportService(profiles, importUpstreamCatalogStub{items: []UpstreamModel{{
		ID: "gpt-5", ChannelCount: 2, Pricing: UpstreamPricing{Available: true},
	}}})

	report, err := importer.Import(context.Background(), []LegacyProfileInput{{
		ModelID: "writer-pro", DisplayName: "Writer Pro", ModelType: "text", Enabled: true,
		ConfigJSON: `{"upstream_model_id":"gpt-5","max_output_tokens":8192}`,
	}})

	require.NoError(t, err)
	assert.Equal(t, 1, report.Imported)
	assert.Equal(t, 1, report.Published)
	assert.Equal(t, "published", report.Items[0].Status)
	stored, err := profiles.GetProfileByPublicID(context.Background(), "writer-pro")
	require.NoError(t, err)
	assert.Equal(t, entity.ModelStatusPublished, stored.Status)
	assert.JSONEq(t, `{"text":{"upstream_model_id":"gpt-5","max_output_tokens":8192}}`, stored.ConfigJSON)
}

func TestProfileImportKeepsInvalidOrUnavailableModelsAsDrafts(t *testing.T) {
	_, profiles := newAdminService(t)
	importer := NewProfileImportService(profiles, importUpstreamCatalogStub{items: []UpstreamModel{
		{ID: "no-price", ChannelCount: 1, Pricing: UpstreamPricing{Available: false}},
	}})
	items := []LegacyProfileInput{
		{ModelID: "invalid-image", DisplayName: "Invalid", ModelType: "image", Enabled: true, Config: json.RawMessage(`{"image":{"adapter":"openai-image","modes":{}}}`)},
		{ModelID: "missing-model", DisplayName: "Missing", ModelType: "text", Enabled: true, ConfigJSON: `{"upstream_model_id":"absent"}`},
		{ModelID: "unpriced-model", DisplayName: "Unpriced", ModelType: "text", Enabled: true, ConfigJSON: `{"upstream_model_id":"no-price"}`},
	}

	report, err := importer.Import(context.Background(), items)

	require.NoError(t, err)
	assert.Equal(t, 3, report.Imported)
	assert.Equal(t, 3, report.Drafts)
	assert.Equal(t, "draft", report.Items[0].Status)
	assert.Equal(t, []string{"INVALID_CONFIG"}, importIssueCodes(report.Items[0]))
	assert.Equal(t, []string{"UPSTREAM_MODEL_NOT_FOUND"}, importIssueCodes(report.Items[1]))
	assert.Equal(t, []string{"PRICING_MISSING"}, importIssueCodes(report.Items[2]))
	for _, item := range items {
		stored, loadErr := profiles.GetProfileByPublicID(context.Background(), item.ModelID)
		require.NoError(t, loadErr)
		assert.Equal(t, entity.ModelStatusDraft, stored.Status)
	}
}

func TestProfileImportSkipsExistingProfilesAndReportsMalformedJSON(t *testing.T) {
	admin, profiles := newAdminService(t)
	_, err := admin.Create(context.Background(), ProfileInput{
		PublicModelID: "existing", DisplayName: "Existing", ModelType: "text",
		Config: json.RawMessage(`{"text":{"upstream_model_id":"gpt-5"}}`),
	})
	require.NoError(t, err)
	importer := NewProfileImportService(profiles, importUpstreamCatalogStub{})

	report, err := importer.Import(context.Background(), []LegacyProfileInput{
		{ModelID: "existing", DisplayName: "Replacement", ModelType: "text", ConfigJSON: `{"upstream_model_id":"other"}`},
		{ModelID: "broken", DisplayName: "Broken", ModelType: "text", ConfigJSON: `{not-json`},
	})

	require.NoError(t, err)
	assert.Equal(t, 1, report.Skipped)
	assert.Equal(t, 1, report.Failed)
	assert.Equal(t, "ALREADY_EXISTS", report.Items[0].Issues[0].Code)
	assert.Equal(t, "INVALID_JSON", report.Items[1].Issues[0].Code)
	_, err = profiles.GetProfileByPublicID(context.Background(), "broken")
	require.Error(t, err)
}

func TestProfileImportRequiresAnUpstreamInTheSelectedGroups(t *testing.T) {
	_, profiles := newAdminService(t)
	importer := NewProfileImportService(profiles, importUpstreamCatalogStub{items: []UpstreamModel{{
		ID: "grouped-model", Groups: []string{"default"}, ChannelCount: 1, Pricing: UpstreamPricing{Available: true},
	}}})

	report, err := importer.Import(context.Background(), []LegacyProfileInput{{
		ModelID: "vip-model", DisplayName: "VIP", ModelType: "text", Enabled: true, Groups: []string{"vip"},
		ConfigJSON: `{"upstream_model_id":"grouped-model"}`,
	}})

	require.NoError(t, err)
	assert.Equal(t, 1, report.Drafts)
	assert.Equal(t, []string{"UPSTREAM_MODEL_NOT_AVAILABLE_FOR_GROUP"}, importIssueCodes(report.Items[0]))
}

func importIssueCodes(item ProfileImportItem) []string {
	codes := make([]string, 0, len(item.Issues))
	for _, issue := range item.Issues {
		codes = append(codes, issue.Code)
	}
	return codes
}
