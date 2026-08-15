package openrouter

import (
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

var registeredHandlers = []ModelHandler{
	&Wan27Handler{BaseHandler: NewBaseHandler("wan27")},
	&HappyHorseHandler{BaseHandler: NewBaseHandler("happyhorse")},
	&KlingHandler{BaseHandler: NewBaseHandler("kling")},
	&MiniMaxHandler{BaseHandler: NewBaseHandler("minimax")},
	&SeedanceHandler{BaseHandler: NewBaseHandler("seedance")},
	&VeoHandler{BaseHandler: NewBaseHandler("veo")},
	// Sora is intentionally not registered until request mapping + video_seconds billing are implemented.
	&DefaultHandler{BaseHandler: NewBaseHandler("default")},
}

func SelectHandler(model string) ModelHandler {
	normalized := strings.TrimSpace(model)
	for _, handler := range registeredHandlers {
		if handler.Match(normalized) {
			return handler
		}
	}
	return &DefaultHandler{BaseHandler: NewBaseHandler("default")}
}

func selectRequestHandler(info *relaycommon.RelayInfo, model string) ModelHandler {
	if info != nil && info.ChannelMeta != nil && strings.TrimSpace(info.UpstreamModelName) != "" {
		return SelectHandler(info.UpstreamModelName)
	}
	return SelectHandler(model)
}

func requestForHandler(info *relaycommon.RelayInfo, req *relaycommon.TaskSubmitReq) relaycommon.TaskSubmitReq {
	resolved := *req
	if info != nil && info.ChannelMeta != nil && strings.TrimSpace(info.UpstreamModelName) != "" {
		resolved.Model = info.UpstreamModelName
	}
	return resolved
}

type DefaultHandler struct {
	BaseHandler
}

func (h *DefaultHandler) Match(string) bool {
	return true
}

type SeedanceHandler struct {
	BaseHandler
}

func (h *SeedanceHandler) Match(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(normalized, "bytedance/seedance")
}

type VeoHandler struct {
	BaseHandler
}

func (h *VeoHandler) Match(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(normalized, "google/veo")
}

type HappyHorseHandler struct {
	BaseHandler
}

func (h *HappyHorseHandler) Match(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(normalized, "alibaba/happyhorse-1.")
}

type KlingHandler struct {
	BaseHandler
}

func (h *KlingHandler) Match(model string) bool {
	return isSupportedKlingModel(model)
}

type MiniMaxHandler struct {
	BaseHandler
}

func (h *MiniMaxHandler) Match(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	return normalized == "minimax/hailuo-3" || normalized == "minimax/hailuo-2.3"
}
