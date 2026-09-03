package execution

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	relaydto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
)

func BuildSyncRequest(spec Spec) (relaydto.Request, string, types.RelayFormat, error) {
	upstreamModelID := strings.TrimSpace(spec.UpstreamModelID)
	if upstreamModelID == "" {
		return nil, "", "", fmt.Errorf("AIGC synchronous execution requires an upstream model")
	}
	switch strings.TrimSpace(spec.ModelType) {
	case "text":
		if strings.TrimSpace(spec.Mode) != "text" {
			return nil, "", "", fmt.Errorf("AIGC text execution does not support mode %q", spec.Mode)
		}
		stream := false
		message := relaydto.Message{Role: "user"}
		message.SetStringContent(strings.TrimSpace(spec.Request.Prompt))
		return &relaydto.GeneralOpenAIRequest{
			Model: upstreamModelID, Messages: []relaydto.Message{message}, Stream: &stream,
		}, "/v1/chat/completions", types.RelayFormatOpenAI, nil
	case "image":
		mode := strings.TrimSpace(spec.Mode)
		if mode != "text_to_image" && mode != "image_edit" {
			return nil, "", "", fmt.Errorf("AIGC image execution does not support mode %q", spec.Mode)
		}
		count := spec.Request.Output.Count
		if count <= 0 {
			count = 1
		}
		image := &relaydto.ImageRequest{
			Model: upstreamModelID, Prompt: strings.TrimSpace(spec.Request.Prompt), Size: strings.TrimSpace(spec.Request.Output.Size),
			N: common.GetPointer(uint(count)), ResponseFormat: "url",
		}
		path := "/v1/images/generations"
		if mode == "image_edit" {
			inputs := make([]struct {
				ImageURL string `json:"image_url"`
			}, len(spec.Request.Inputs.Images))
			for index, input := range spec.Request.Inputs.Images {
				inputs[index].ImageURL = strings.TrimSpace(input.URL)
			}
			contents, err := common.Marshal(inputs)
			if err != nil {
				return nil, "", "", fmt.Errorf("encode AIGC image inputs: %w", err)
			}
			image.Images = contents
			path = "/v1/images/edits"
		}
		return image, path, types.RelayFormatOpenAIImage, nil
	default:
		return nil, "", "", fmt.Errorf("AIGC synchronous execution does not support model type %q", spec.ModelType)
	}
}
