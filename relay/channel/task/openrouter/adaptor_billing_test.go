package openrouter

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestTaskAdaptorAdjustBillingOnCompleteUsesModelRatioAndOtherRatios(t *testing.T) {
	adaptor := &TaskAdaptor{}
	task := &model.Task{
		TaskID: "task_openrouter_ratio",
		PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{
				ModelRatio: 5,
				GroupRatio: 2,
				OtherRatios: map[string]float64{
					"seconds":         6,
					"resolution-720p": 1,
				},
			},
		},
	}

	actualQuota := adaptor.AdjustBillingOnComplete(task, &relaycommon.TaskInfo{
		Status:      model.TaskStatusSuccess,
		TotalTokens: 10,
	})

	if actualQuota != 600 {
		t.Fatalf("expected actual quota 600, got %d", actualQuota)
	}
}

func TestTaskAdaptorAdjustBillingOnCompleteUsesConditionalInputPrice(t *testing.T) {
	adaptor := &TaskAdaptor{}
	task := &model.Task{
		TaskID: "task_openrouter_conditional",
		PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{
				GroupRatio:            2,
				ConditionalInputPrice: 31,
			},
		},
	}

	actualQuota := adaptor.AdjustBillingOnComplete(task, &relaycommon.TaskInfo{
		Status:      model.TaskStatusSuccess,
		TotalTokens: 1_000_000,
	})

	expected := int(31 * 2 * common.QuotaPerUnit)
	if actualQuota != expected {
		t.Fatalf("expected actual quota %d, got %d", expected, actualQuota)
	}
}

func TestTaskAdaptorAdjustBillingOnCompleteUsesVideoSecondsUnitPrice(t *testing.T) {
	adaptor := &TaskAdaptor{}
	audioEnabled := false
	task := &model.Task{
		TaskID: "task_openrouter_video_seconds",
		PrivateData: model.TaskPrivateData{
			BillingContext: &model.TaskBillingContext{
				GroupRatio:            2,
				VideoSecondsUnitPrice: 0.6,
				VideoSecondsTier:      "720p",
				VideoDurationSeconds:  5,
				VideoAudioEnabled:     &audioEnabled,
			},
		},
	}

	actualQuota := adaptor.AdjustBillingOnComplete(task, &relaycommon.TaskInfo{
		Status:          model.TaskStatusSuccess,
		DurationSeconds: 8,
	})

	expected := int(0.6 * 8 * 2 * common.QuotaPerUnit)
	if actualQuota != expected {
		t.Fatalf("expected actual quota %d, got %d", expected, actualQuota)
	}
}
