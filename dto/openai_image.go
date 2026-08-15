package dto

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// MaxImageN caps the image generation count. Without this bound a huge or
// wrapped-negative n overflows quota calculation into a negative charge.
const MaxImageN = 128

type ImageRequest struct {
	Model             string          `json:"model"`
	Prompt            string          `json:"prompt" binding:"required"`
	N                 *uint           `json:"n,omitempty"`
	Size              string          `json:"size,omitempty"`
	Resolution        string          `json:"resolution,omitempty"`
	AspectRatio       string          `json:"aspect_ratio,omitempty"`
	Quality           string          `json:"quality,omitempty"`
	ResponseFormat    string          `json:"response_format,omitempty"`
	Seed              *int64          `json:"seed,omitempty"`
	Style             json.RawMessage `json:"style,omitempty"`
	User              json.RawMessage `json:"user,omitempty"`
	ExtraFields       json.RawMessage `json:"extra_fields,omitempty"`
	Background        json.RawMessage `json:"background,omitempty"`
	Moderation        json.RawMessage `json:"moderation,omitempty"`
	OutputFormat      json.RawMessage `json:"output_format,omitempty"`
	OutputCompression json.RawMessage `json:"output_compression,omitempty"`
	PartialImages     json.RawMessage `json:"partial_images,omitempty"`
	Stream            *bool           `json:"stream,omitempty"`
	Images            json.RawMessage `json:"images,omitempty"`
	InputReferences   json.RawMessage `json:"input_references,omitempty"`
	Mask              json.RawMessage `json:"mask,omitempty"`
	InputFidelity     json.RawMessage `json:"input_fidelity,omitempty"`
	Watermark         *bool           `json:"watermark,omitempty"`
	Provider          json.RawMessage `json:"provider,omitempty"`
	// zhipu 4v
	WatermarkEnabled      json.RawMessage `json:"watermark_enabled,omitempty"`
	UserId                json.RawMessage `json:"user_id,omitempty"`
	Image                 json.RawMessage `json:"image,omitempty"`
	ParsedInputImageTiers []string        `json:"-"`
	// 用匿名参数接收额外参数
	Extra map[string]json.RawMessage `json:"-"`
}

func (i *ImageRequest) UnmarshalJSON(data []byte) error {
	// 先解析成 map[string]interface{}
	var rawMap map[string]json.RawMessage
	if err := common.Unmarshal(data, &rawMap); err != nil {
		return err
	}

	// 用 struct tag 获取所有已定义字段名
	knownFields := GetJSONFieldNames(reflect.TypeOf(*i))

	// 再正常解析已定义字段
	type Alias ImageRequest
	var known Alias
	if err := common.Unmarshal(data, &known); err != nil {
		return err
	}
	*i = ImageRequest(known)

	// 提取多余字段
	i.Extra = make(map[string]json.RawMessage)
	for k, v := range rawMap {
		if _, ok := knownFields[k]; !ok {
			i.Extra[k] = v
		}
	}
	return nil
}

// 序列化时需要重新把字段平铺
func (r ImageRequest) MarshalJSON() ([]byte, error) {
	// 将已定义字段转为 map
	type Alias ImageRequest
	alias := Alias(r)
	base, err := common.Marshal(alias)
	if err != nil {
		return nil, err
	}

	var baseMap map[string]json.RawMessage
	if err := common.Unmarshal(base, &baseMap); err != nil {
		return nil, err
	}

	// 不能合并ExtraFields！！！！！！！！
	// 合并 ExtraFields
	//for k, v := range r.Extra {
	//	if _, exists := baseMap[k]; !exists {
	//		baseMap[k] = v
	//	}
	//}

	return common.Marshal(baseMap)
}

func GetJSONFieldNames(t reflect.Type) map[string]struct{} {
	fields := make(map[string]struct{})
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// 跳过匿名字段（例如 ExtraFields）
		if field.Anonymous {
			continue
		}

		tag := field.Tag.Get("json")
		if tag == "-" || tag == "" {
			continue
		}

		// 取逗号前字段名（排除 omitempty 等）
		name := tag
		if commaIdx := indexComma(tag); commaIdx != -1 {
			name = tag[:commaIdx]
		}
		fields[name] = struct{}{}
	}
	return fields
}

func indexComma(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			return i
		}
	}
	return -1
}

func (i *ImageRequest) GetTokenCountMeta() *types.TokenCountMeta {
	var sizeRatio = 1.0
	var qualityRatio = 1.0

	if strings.HasPrefix(i.Model, "dall-e") {
		// Size
		if i.Size == "256x256" {
			sizeRatio = 0.4
		} else if i.Size == "512x512" {
			sizeRatio = 0.45
		} else if i.Size == "1024x1024" {
			sizeRatio = 1
		} else if i.Size == "1024x1792" || i.Size == "1792x1024" {
			sizeRatio = 2
		}

		if i.Model == "dall-e-3" && i.Quality == "hd" {
			qualityRatio = 2.0
			if i.Size == "1024x1792" || i.Size == "1792x1024" {
				qualityRatio = 1.5
			}
		}
	}

	imageN := uint(1)
	if i.N != nil && *i.N > 0 {
		imageN = *i.N
	}

	// Keep n separate from ImagePriceRatio so size/quality and count remain
	// independent billing dimensions. Fixed-price pre-consume stores this on
	// PriceData, and image settlement reuses or replaces the same "n" ratio.
	return &types.TokenCountMeta{
		CombineText:     i.Prompt,
		MaxTokens:       1584,
		ImagePriceRatio: sizeRatio * qualityRatio,
		ImageSize:       i.Size,
		BillingRatios:   map[string]float64{"n": float64(imageN)},
		InputImageTiers: i.GetInputImageTiers(),
	}
}

func (i *ImageRequest) GetInputImageTiers() []string {
	if i == nil {
		return nil
	}
	if i.ParsedInputImageTiers != nil {
		return append([]string(nil), i.ParsedInputImageTiers...)
	}
	tiers := imageTiersFromRawMessage(i.Image)
	tiers = append(tiers, imageTiersFromRawMessage(i.Images)...)
	tiers = append(tiers, imageReferenceTiersFromRawMessage(i.InputReferences)...)
	return tiers
}

func imageTiersFromRawMessage(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var value any
	if err := common.Unmarshal(raw, &value); err != nil {
		return nil
	}
	return imageTiersFromValue(value)
}

func imageReferenceTiersFromRawMessage(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var references []map[string]any
	if err := common.Unmarshal(raw, &references); err != nil {
		return nil
	}
	tiers := make([]string, 0, len(references))
	for _, reference := range references {
		referenceType, _ := reference["type"].(string)
		if referenceType != "" && referenceType != "image" && referenceType != "image_url" && referenceType != "input_image" {
			continue
		}
		tiers = append(tiers, imageTierFromMap(reference))
	}
	return tiers
}

func imageTiersFromValue(value any) []string {
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) != "" {
			return []string{"default"}
		}
	case []any:
		tiers := make([]string, 0, len(typed))
		for _, item := range typed {
			tiers = append(tiers, imageTiersFromValue(item)...)
		}
		return tiers
	case map[string]any:
		return []string{imageTierFromMap(typed)}
	}
	return nil
}

func imageTierFromMap(value map[string]any) string {
	for _, key := range []string{"resolution", "size"} {
		if tier, ok := value[key].(string); ok && strings.TrimSpace(tier) != "" {
			return tier
		}
	}
	width, widthOK := imageDimension(value["width"])
	height, heightOK := imageDimension(value["height"])
	if widthOK && heightOK {
		return fmt.Sprintf("%dx%d", width, height)
	}
	for _, key := range []string{"image_url", "image"} {
		if nested, ok := value[key].(map[string]any); ok {
			return imageTierFromMap(nested)
		}
	}
	return "default"
}

func imageDimension(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		if typed > 0 && typed <= math.MaxInt32 && typed == math.Trunc(typed) {
			return int(typed), true
		}
	case int:
		return typed, typed > 0
	}
	return 0, false
}

func (i *ImageRequest) IsStream(c *gin.Context) bool {
	return i.Stream != nil && *i.Stream
}

func (i *ImageRequest) SetModelName(modelName string) {
	if modelName != "" {
		i.Model = modelName
	}
}

type ImageResponse struct {
	Data     []ImageData     `json:"data"`
	Created  int64           `json:"created"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}
type ImageData struct {
	Url           string `json:"url"`
	B64Json       string `json:"b64_json"`
	RevisedPrompt string `json:"revised_prompt"`
}
