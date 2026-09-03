package service

import (
	"context"

	"github.com/QuantumNous/new-api/model"
)

type ModelAvailability struct{}

func NewModelAvailability() ModelAvailability {
	return ModelAvailability{}
}

func (ModelAvailability) Available(_ context.Context, group string, upstreamModelIDs []string) (map[string]bool, error) {
	var enabledModels []string
	if group == "" {
		enabledModels = model.GetEnabledModels()
	} else {
		enabledModels = model.GetGroupEnabledModels(group)
	}
	enabled := make(map[string]bool, len(enabledModels))
	for _, modelID := range enabledModels {
		enabled[modelID] = true
	}
	result := make(map[string]bool, len(upstreamModelIDs))
	for _, modelID := range upstreamModelIDs {
		result[modelID] = enabled[modelID]
	}
	return result, nil
}
