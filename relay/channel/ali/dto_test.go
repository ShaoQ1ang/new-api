package ali

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAliOutput_ChoicesToOpenAIImageDatePreservesMultipleImagesPerChoice(t *testing.T) {
	t.Parallel()

	var output AliOutput
	require.NoError(t, common.Unmarshal([]byte(`{
		"choices":[{"message":{"content":[
			{"image":"https://cdn.test/first.png"},
			{"image":"https://cdn.test/second.png"},
			{"text":"refined prompt"}
		]}}]
	}`), &output))

	actual := output.ChoicesToOpenAIImageDate(nil, "url")

	assert.Equal(t, []dto.ImageData{
		{Url: "https://cdn.test/first.png", RevisedPrompt: "refined prompt"},
		{Url: "https://cdn.test/second.png", RevisedPrompt: "refined prompt"},
	}, actual)
}

func TestAliOutput_ChoicesToOpenAIImageDatePreservesOrderAndSkipsEmptyChoices(t *testing.T) {
	t.Parallel()

	var output AliOutput
	require.NoError(t, common.Unmarshal([]byte(`{
		"choices":[
			{"message":{"content":[
				{"text":"first prompt"},
				{"image":"https://cdn.test/first.png"}
			]}},
			{"message":{"content":[{"text":"text only"}]}},
			{"message":{"content":[]}},
			{"message":{"content":[
				{"image":"inline-first"},
				{},
				{"text":"last prompt"},
				{"image":"inline-second"}
			]}}
		]
	}`), &output))

	actual := output.ChoicesToOpenAIImageDate(nil, "url")

	assert.Equal(t, []dto.ImageData{
		{Url: "https://cdn.test/first.png", RevisedPrompt: "first prompt"},
		{B64Json: "inline-first", RevisedPrompt: "last prompt"},
		{B64Json: "inline-second", RevisedPrompt: "last prompt"},
	}, actual)
}
