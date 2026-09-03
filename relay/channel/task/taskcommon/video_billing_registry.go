package taskcommon

import (
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

type VideoBillingConverter func(req relaycommon.TaskSubmitReq) (*types.VideoBillingParams, error)

type videoBillingRegistryItem struct {
	match     func(modelName string) bool
	converter VideoBillingConverter
}

var externalVideoBillingConverters []videoBillingRegistryItem

func RegisterVideoBillingConverter(match func(modelName string) bool, converter VideoBillingConverter) {
	if match == nil || converter == nil {
		return
	}
	externalVideoBillingConverters = append(externalVideoBillingConverters, videoBillingRegistryItem{
		match:     match,
		converter: converter,
	})
}

func getRegisteredVideoBillingConverter(modelName string) VideoBillingConverter {
	normalized := strings.ToLower(strings.TrimSpace(modelName))
	for _, item := range externalVideoBillingConverters {
		if item.match(normalized) {
			return item.converter
		}
	}
	return nil
}
