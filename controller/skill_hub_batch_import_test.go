package controller

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
)

func TestValidateSkillHubBatchInitItemsRejectsDuplicates(t *testing.T) {
	items := []skillHubBatchUploadInitItemRequest{
		{
			Index:   0,
			ID:      "demo.skill",
			Version: "1.0.0",
			Zip:     skillHubBatchUploadFileRequest{FileName: "demo.zip", Size: 10},
		},
		{
			Index:   1,
			ID:      "demo.skill",
			Version: "2.0.0",
			Zip:     skillHubBatchUploadFileRequest{FileName: "demo-2.zip", Size: 10},
		},
	}
	if err := validateSkillHubBatchInitItems(items); err == nil || !strings.Contains(err.Error(), "duplicate skill id") {
		t.Fatalf("validateSkillHubBatchInitItems() error = %v, want duplicate skill id", err)
	}
}

func TestValidateSkillHubBatchCommitItemsRejectsDuplicateTickets(t *testing.T) {
	items := []skillHubBatchImportCommitItemRequest{
		{
			Index:           0,
			Skill:           skillHubSkillRequest{ID: "demo.one"},
			ZipUploadTicket: "same-ticket",
		},
		{
			Index:           1,
			Skill:           skillHubSkillRequest{ID: "demo.two"},
			ZipUploadTicket: "same-ticket",
		},
	}
	if err := validateSkillHubBatchCommitItems(items); err == nil || err.Error() != "duplicate upload ticket" {
		t.Fatalf("validateSkillHubBatchCommitItems() error = %v, want duplicate upload ticket", err)
	}
}

func TestSkillHubBatchSortValue(t *testing.T) {
	fixed, err := skillHubBatchSortValue(skillHubBatchImportOptionsRequest{
		SortMode:  skillHubBatchSortModeFixed,
		FixedSort: 123,
	}, 17)
	if err != nil || fixed != 123 {
		t.Fatalf("fixed sort = %d, error = %v", fixed, err)
	}

	sequential, err := skillHubBatchSortValue(skillHubBatchImportOptionsRequest{
		SortMode:  skillHubBatchSortModeSequence,
		SortStart: 1000,
		SortStep:  -5,
	}, 7)
	if err != nil || sequential != 965 {
		t.Fatalf("sequential sort = %d, error = %v", sequential, err)
	}

	if _, err := skillHubBatchSortValue(skillHubBatchImportOptionsRequest{
		SortMode:  skillHubBatchSortModeSequence,
		SortStart: 2147483647,
		SortStep:  1,
	}, 1); err == nil {
		t.Fatal("skillHubBatchSortValue() accepted a 32-bit overflow")
	}
}

func TestApplySkillHubBatchOptionsOverridesAndRetains(t *testing.T) {
	score := 4.5
	evaluationJSON, err := model.SkillHubEvaluationToJSON(&model.SkillHubEvaluation{
		OverallScore: &score,
		Dimensions: model.SkillHubEvaluationDimensions{
			Safety:   model.SkillHubEvaluationDimension{Score: &score},
			Access:   model.SkillHubEvaluationDimension{Score: &score},
			Frontier: model.SkillHubEvaluationDimension{Score: &score},
			Economy:  model.SkillHubEvaluationDimension{Score: &score},
		},
	})
	if err != nil {
		t.Fatalf("SkillHubEvaluationToJSON() error = %v", err)
	}
	testcasesJSON, err := model.SkillHubTestcasesToJSON(&model.SkillHubTestcases{
		Slug: "demo",
		Testcases: []model.SkillHubTestcase{
			{ID: 1, Question: "Q", Answer: "A"},
		},
	})
	if err != nil {
		t.Fatalf("SkillHubTestcasesToJSON() error = %v", err)
	}
	existing := &model.SkillHubSkill{
		EvaluationJSON: evaluationJSON,
		TestcasesJSON:  testcasesJSON,
	}
	status := model.SkillHubStatusPublished
	request := skillHubSkillRequest{
		Origin:      "manifest",
		Tags:        []string{"Manifest", "shared"},
		Verified:    false,
		Recommended: false,
		Published:   true,
		Status:      &status,
		Sort:        9,
	}
	options := skillHubBatchImportOptionsRequest{
		Published:         false,
		Recommended:       true,
		SortMode:          skillHubBatchSortModeSequence,
		SortStart:         500,
		SortStep:          10,
		VerifiedMode:      skillHubBatchVerifiedModeVerified,
		TagMode:           skillHubBatchTagModeAppend,
		CommonTags:        []string{"shared", "Common"},
		OverrideOrigin:    true,
		Origin:            "batch",
		MissingTestcases:  skillHubBatchMissingPolicyRetain,
		MissingEvaluation: skillHubBatchMissingPolicyRetain,
	}
	if err := applySkillHubBatchOptions(&request, options, 3, existing); err != nil {
		t.Fatalf("applySkillHubBatchOptions() error = %v", err)
	}
	if request.Published || request.Status != nil {
		t.Fatalf("published/status = %v/%v, want false/nil", request.Published, request.Status)
	}
	if !request.Recommended || !request.Verified {
		t.Fatalf("recommended/verified = %v/%v, want true/true", request.Recommended, request.Verified)
	}
	if request.Sort != 530 {
		t.Fatalf("sort = %d, want 530", request.Sort)
	}
	if request.Origin != "batch" {
		t.Fatalf("origin = %q, want batch", request.Origin)
	}
	if got := strings.Join(request.Tags, ","); got != "Manifest,shared,Common" {
		t.Fatalf("tags = %q, want Manifest,shared,Common", got)
	}
	if request.Evaluation == nil || request.Testcases == nil {
		t.Fatal("missing evaluation or testcases was not retained")
	}
}

func TestValidateSkillHubBatchOptionsRejectsSequenceOverflow(t *testing.T) {
	options := skillHubBatchImportOptionsRequest{
		SortMode:  skillHubBatchSortModeSequence,
		SortStart: 2147483600,
		SortStep:  1,
	}
	if err := validateSkillHubBatchOptions(&options); err == nil {
		t.Fatal("validateSkillHubBatchOptions() accepted an overflowing sequence")
	}
}
