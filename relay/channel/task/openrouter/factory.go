package openrouter

import "strings"

var registeredHandlers = []ModelHandler{
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
