package openrouter

import "strings"

var registeredHandlers = []ModelHandler{
	&SeedanceHandler{BaseHandler: NewBaseHandler("seedance")},
	&VeoHandler{BaseHandler: NewBaseHandler("veo")},
	&SoraHandler{BaseHandler: NewBaseHandler("sora")},
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

type DefaultHandler struct {
	BaseHandler
}

func (h *DefaultHandler) Match(string) bool {
	return true
}

type SoraHandler struct {
	BaseHandler
}

func (h *SoraHandler) Match(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(normalized, "openai/sora") || strings.HasPrefix(normalized, "sora")
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
