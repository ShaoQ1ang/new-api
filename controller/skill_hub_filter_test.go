package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSkillHubSkillSearchFilterAcceptsMultipleStatuses(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/?status=1,0&status=1", nil)

	filter, err := parseSkillHubSkillSearchFilter(c, true)
	require.NoError(t, err)
	assert.ElementsMatch(t, []int{model.SkillHubStatusPublished, model.SkillHubStatusDraft}, filter.Statuses)
}

func TestParseSkillHubSkillSearchFilterRejectsInvalidStatus(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/?status=1,2", nil)

	_, err := parseSkillHubSkillSearchFilter(c, true)
	require.EqualError(t, err, "invalid skill status: 2")
}
