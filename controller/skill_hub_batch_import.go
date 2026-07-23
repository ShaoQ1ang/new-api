package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	skillHubBatchImportMaxItems       = 200
	skillHubBatchImportMaxTickets     = skillHubBatchImportMaxItems * 2
	skillHubBatchImportMaxBodyBytes   = 32 << 20
	skillHubBatchValidationWorkers    = 2
	skillHubBatchItemStatusReady      = "ready"
	skillHubBatchItemStatusSuccess    = "success"
	skillHubBatchItemStatusSkipped    = "skipped"
	skillHubBatchItemStatusFailed     = "failed"
	skillHubBatchMissingPolicyRetain  = "retain"
	skillHubBatchMissingPolicyClear   = "clear"
	skillHubBatchVerifiedModeManifest = "manifest"
	skillHubBatchVerifiedModeVerified = "verified"
	skillHubBatchVerifiedModeDisabled = "unverified"
	skillHubBatchTagModeManifest      = "manifest"
	skillHubBatchTagModeAppend        = "append"
	skillHubBatchTagModeReplace       = "replace"
	skillHubBatchSortModeFixed        = "fixed"
	skillHubBatchSortModeSequence     = "sequence"
)

type skillHubBatchUploadFileRequest struct {
	FileName string `json:"fileName"`
	Size     int64  `json:"size"`
}

type skillHubBatchUploadInitItemRequest struct {
	Index   int                             `json:"index"`
	ID      string                          `json:"id"`
	Version string                          `json:"version"`
	Zip     skillHubBatchUploadFileRequest  `json:"zip"`
	Icon    *skillHubBatchUploadFileRequest `json:"icon,omitempty"`
}

type skillHubBatchUploadInitRequest struct {
	Mode  string                               `json:"mode"`
	Items []skillHubBatchUploadInitItemRequest `json:"items"`
}

type skillHubBatchUploadInitItemResponse struct {
	Index   int                                     `json:"index"`
	ID      string                                  `json:"id"`
	Status  string                                  `json:"status"`
	Action  string                                  `json:"action,omitempty"`
	Message string                                  `json:"message,omitempty"`
	Zip     *service.SkillHubDirectUploadInitResult `json:"zip,omitempty"`
	Icon    *service.SkillHubDirectUploadInitResult `json:"icon,omitempty"`
}

type skillHubBatchImportOptionsRequest struct {
	Published         bool     `json:"published"`
	Recommended       bool     `json:"recommended"`
	SortMode          string   `json:"sortMode"`
	FixedSort         int64    `json:"fixedSort"`
	SortStart         int64    `json:"sortStart"`
	SortStep          int64    `json:"sortStep"`
	VerifiedMode      string   `json:"verifiedMode"`
	TagMode           string   `json:"tagMode"`
	CommonTags        []string `json:"commonTags"`
	OverrideOrigin    bool     `json:"overrideOrigin"`
	Origin            string   `json:"origin"`
	MissingIcon       string   `json:"missingIcon"`
	MissingTestcases  string   `json:"missingTestcases"`
	MissingEvaluation string   `json:"missingEvaluation"`
}

type skillHubBatchImportCommitItemRequest struct {
	Index            int                  `json:"index"`
	Skill            skillHubSkillRequest `json:"skill"`
	ZipUploadTicket  string               `json:"zipUploadTicket"`
	IconUploadTicket string               `json:"iconUploadTicket,omitempty"`
}

type skillHubBatchImportCommitRequest struct {
	Mode    string                                 `json:"mode"`
	Options skillHubBatchImportOptionsRequest      `json:"options"`
	Items   []skillHubBatchImportCommitItemRequest `json:"items"`
}

type skillHubBatchImportCommitItemResponse struct {
	Index   int                          `json:"index"`
	ID      string                       `json:"id"`
	Status  string                       `json:"status"`
	Action  string                       `json:"action,omitempty"`
	Message string                       `json:"message,omitempty"`
	Skill   *model.SkillHubSkillResponse `json:"skill,omitempty"`
}

type skillHubBatchDiscardRequest struct {
	UploadTickets []string `json:"uploadTickets"`
}

type skillHubBatchCompletedUploads struct {
	Zip  *service.SkillHubDirectUploadCompleteResult
	Icon *service.SkillHubDirectUploadCompleteResult
	Err  error
}

func AdminInitSkillHubBatchUpload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
	var request skillHubBatchUploadInitRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	mode, err := normalizeSkillHubBatchMode(request.Mode)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validateSkillHubBatchInitItems(request.Items); err != nil {
		common.ApiError(c, err)
		return
	}

	ids := make([]string, 0, len(request.Items))
	for _, item := range request.Items {
		ids = append(ids, strings.TrimSpace(item.ID))
	}
	existingSkills, err := model.GetSkillHubSkillsBySkillIDs(ids)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	existingByID := make(map[string]*model.SkillHubSkill, len(existingSkills))
	for _, skill := range existingSkills {
		existingByID[skill.SkillID] = skill
	}

	results := make([]skillHubBatchUploadInitItemResponse, len(request.Items))
	for index, item := range request.Items {
		item.ID = strings.TrimSpace(item.ID)
		item.Version = strings.TrimSpace(item.Version)
		result := skillHubBatchUploadInitItemResponse{
			Index:  item.Index,
			ID:     item.ID,
			Status: skillHubBatchItemStatusReady,
			Action: "create",
		}
		if existingByID[item.ID] != nil {
			switch mode {
			case "skip":
				result.Status = skillHubBatchItemStatusSkipped
				result.Action = "exists"
				result.Message = "skill already exists"
				results[index] = result
				continue
			case "fail":
				result.Status = skillHubBatchItemStatusFailed
				result.Action = "exists"
				result.Message = "skill already exists"
				results[index] = result
				continue
			default:
				result.Action = "update"
			}
		}

		zipUpload, initErr := service.InitSkillHubDirectUpload(service.SkillHubDirectUploadInput{
			Kind:     service.SkillHubUploadKindZip,
			SkillID:  item.ID,
			Version:  item.Version,
			FileName: item.Zip.FileName,
			Size:     item.Zip.Size,
		})
		if initErr != nil {
			result.Status = skillHubBatchItemStatusFailed
			result.Message = initErr.Error()
			results[index] = result
			continue
		}
		result.Zip = zipUpload

		if item.Icon != nil {
			iconUpload, iconErr := service.InitSkillHubDirectUpload(service.SkillHubDirectUploadInput{
				Kind:     service.SkillHubUploadKindIcon,
				SkillID:  item.ID,
				FileName: item.Icon.FileName,
				Size:     item.Icon.Size,
			})
			if iconErr != nil {
				result.Status = skillHubBatchItemStatusFailed
				result.Message = iconErr.Error()
				result.Zip = nil
				results[index] = result
				continue
			}
			result.Icon = iconUpload
		}
		results[index] = result
	}
	common.ApiSuccess(c, gin.H{"items": results})
}

func AdminCommitSkillHubBatchUpload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, skillHubBatchImportMaxBodyBytes)
	var request skillHubBatchImportCommitRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	mode, err := normalizeSkillHubBatchMode(request.Mode)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validateSkillHubBatchOptions(&request.Options); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validateSkillHubBatchCommitItems(request.Items); err != nil {
		common.ApiError(c, err)
		return
	}

	results := make([]skillHubBatchImportCommitItemResponse, len(request.Items))
	eligible := make([]bool, len(request.Items))
	ids := make([]string, 0, len(request.Items))
	for _, item := range request.Items {
		ids = append(ids, strings.TrimSpace(item.Skill.ID))
	}
	existingSkills, err := model.GetSkillHubSkillsBySkillIDs(ids)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	existingByID := make(map[string]*model.SkillHubSkill, len(existingSkills))
	for _, skill := range existingSkills {
		existingByID[skill.SkillID] = skill
	}
	for index, item := range request.Items {
		id := strings.TrimSpace(item.Skill.ID)
		results[index] = skillHubBatchImportCommitItemResponse{Index: item.Index, ID: id}
		if existingByID[id] != nil && mode != "update" {
			if mode == "skip" {
				results[index].Status = skillHubBatchItemStatusSkipped
			} else {
				results[index].Status = skillHubBatchItemStatusFailed
			}
			results[index].Action = "exists"
			results[index].Message = "skill already exists"
			discardSkillHubBatchItemTickets(item)
			continue
		}
		eligible[index] = true
	}
	for index, item := range request.Items {
		if !eligible[index] {
			continue
		}
		id := strings.TrimSpace(item.Skill.ID)
		if preflightErr := preflightSkillHubBatchItem(c, item, request.Options, existingByID[id]); preflightErr != nil {
			eligible[index] = false
			results[index].Status = skillHubBatchItemStatusFailed
			results[index].Message = preflightErr.Error()
			discardSkillHubBatchItemTickets(item)
		}
	}

	completed := completeSkillHubBatchUploads(c.Request.Context(), request.Items, eligible)
	for index, item := range request.Items {
		if !eligible[index] {
			continue
		}
		state := completed[index]
		if state.Err != nil {
			results[index].Status = skillHubBatchItemStatusFailed
			results[index].Message = state.Err.Error()
			discardSkillHubBatchItemTickets(item)
			continue
		}

		current, getErr := model.GetSkillHubSkillBySkillID(strings.TrimSpace(item.Skill.ID))
		if getErr != nil && !errors.Is(getErr, gorm.ErrRecordNotFound) {
			results[index].Status = skillHubBatchItemStatusFailed
			results[index].Message = getErr.Error()
			discardSkillHubBatchItemTickets(item)
			continue
		}
		if errors.Is(getErr, gorm.ErrRecordNotFound) {
			current = nil
		}
		if current != nil && mode != "update" {
			if mode == "skip" {
				results[index].Status = skillHubBatchItemStatusSkipped
			} else {
				results[index].Status = skillHubBatchItemStatusFailed
			}
			results[index].Action = "exists"
			results[index].Message = "skill already exists"
			discardSkillHubBatchItemTickets(item)
			continue
		}

		skillRequest := item.Skill
		if applyErr := applySkillHubBatchOptions(&skillRequest, request.Options, item.Index, current); applyErr != nil {
			results[index].Status = skillHubBatchItemStatusFailed
			results[index].Message = applyErr.Error()
			discardSkillHubBatchItemTickets(item)
			continue
		}
		skillRequest.Source = model.SkillHubSource{
			Type:     "zip",
			URL:      skillHubDownloadURL(c, strings.TrimSpace(item.Skill.ID)),
			Ref:      state.Zip.Upload.Object,
			Checksum: state.Zip.Upload.Checksum,
		}
		if state.Icon != nil {
			skillRequest.Icon = state.Icon.Upload.URL
		} else if current != nil && request.Options.MissingIcon == skillHubBatchMissingPolicyRetain {
			skillRequest.Icon = current.Icon
		} else {
			skillRequest.Icon = ""
		}

		response, action, saveErr := saveSkillHubBatchItem(skillRequest, current)
		if saveErr != nil {
			results[index].Status = skillHubBatchItemStatusFailed
			results[index].Action = action
			results[index].Message = saveErr.Error()
			discardSkillHubBatchItemTickets(item)
			continue
		}
		results[index].Status = skillHubBatchItemStatusSuccess
		results[index].Action = action
		results[index].Skill = response
	}
	common.ApiSuccess(c, gin.H{"items": results})
}

func AdminDiscardSkillHubBatchUpload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
	var request skillHubBatchDiscardRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if len(request.UploadTickets) == 0 || len(request.UploadTickets) > skillHubBatchImportMaxTickets {
		common.ApiErrorMsg(c, fmt.Sprintf("select between 1 and %d upload tickets", skillHubBatchImportMaxTickets))
		return
	}
	seen := make(map[string]struct{}, len(request.UploadTickets))
	discarded := 0
	failed := 0
	for _, rawTicket := range request.UploadTickets {
		ticket := strings.TrimSpace(rawTicket)
		if ticket == "" {
			failed++
			continue
		}
		if _, ok := seen[ticket]; ok {
			continue
		}
		seen[ticket] = struct{}{}
		if err := service.DiscardSkillHubDirectUpload(ticket); err != nil {
			failed++
		} else {
			discarded++
		}
	}
	common.ApiSuccess(c, gin.H{"discarded": discarded, "failed": failed})
}

func validateSkillHubBatchInitItems(items []skillHubBatchUploadInitItemRequest) error {
	if len(items) == 0 || len(items) > skillHubBatchImportMaxItems {
		return fmt.Errorf("select between 1 and %d skills", skillHubBatchImportMaxItems)
	}
	seenIDs := make(map[string]struct{}, len(items))
	seenIndexes := make(map[int]struct{}, len(items))
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if err := model.ValidateSkillHubSkillID(id); err != nil {
			return err
		}
		if strings.TrimSpace(item.Version) == "" {
			return errors.New("skill version is required")
		}
		if _, ok := seenIDs[id]; ok {
			return fmt.Errorf("duplicate skill id: %s", id)
		}
		if _, ok := seenIndexes[item.Index]; ok {
			return fmt.Errorf("duplicate batch item index: %d", item.Index)
		}
		if item.Index < 0 || item.Index >= skillHubBatchImportMaxItems {
			return fmt.Errorf("batch item index is out of range: %d", item.Index)
		}
		seenIDs[id] = struct{}{}
		seenIndexes[item.Index] = struct{}{}
	}
	return nil
}

func validateSkillHubBatchCommitItems(items []skillHubBatchImportCommitItemRequest) error {
	if len(items) == 0 || len(items) > skillHubBatchImportMaxItems {
		return fmt.Errorf("select between 1 and %d skills", skillHubBatchImportMaxItems)
	}
	seenIDs := make(map[string]struct{}, len(items))
	seenIndexes := make(map[int]struct{}, len(items))
	seenTickets := make(map[string]struct{}, len(items)*2)
	for _, item := range items {
		id := strings.TrimSpace(item.Skill.ID)
		if err := model.ValidateSkillHubSkillID(id); err != nil {
			return err
		}
		if _, ok := seenIDs[id]; ok {
			return fmt.Errorf("duplicate skill id: %s", id)
		}
		if _, ok := seenIndexes[item.Index]; ok {
			return fmt.Errorf("duplicate batch item index: %d", item.Index)
		}
		if item.Index < 0 || item.Index >= skillHubBatchImportMaxItems {
			return fmt.Errorf("batch item index is out of range: %d", item.Index)
		}
		seenIDs[id] = struct{}{}
		seenIndexes[item.Index] = struct{}{}
		for _, rawTicket := range []string{item.ZipUploadTicket, item.IconUploadTicket} {
			ticket := strings.TrimSpace(rawTicket)
			if ticket == "" {
				if rawTicket == item.ZipUploadTicket {
					return fmt.Errorf("zip upload ticket is required for %s", id)
				}
				continue
			}
			if _, ok := seenTickets[ticket]; ok {
				return errors.New("duplicate upload ticket")
			}
			seenTickets[ticket] = struct{}{}
		}
	}
	return nil
}

func normalizeSkillHubBatchMode(mode string) (string, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "skip"
	}
	switch mode {
	case "skip", "update", "fail":
		return mode, nil
	default:
		return "", errors.New("mode must be skip, update, or fail")
	}
}

func validateSkillHubBatchOptions(options *skillHubBatchImportOptionsRequest) error {
	options.SortMode = strings.ToLower(strings.TrimSpace(options.SortMode))
	if options.SortMode == "" {
		options.SortMode = skillHubBatchSortModeFixed
	}
	if options.SortMode != skillHubBatchSortModeFixed && options.SortMode != skillHubBatchSortModeSequence {
		return errors.New("sort mode must be fixed or sequence")
	}
	options.VerifiedMode = strings.ToLower(strings.TrimSpace(options.VerifiedMode))
	if options.VerifiedMode == "" {
		options.VerifiedMode = skillHubBatchVerifiedModeManifest
	}
	switch options.VerifiedMode {
	case skillHubBatchVerifiedModeManifest, skillHubBatchVerifiedModeVerified, skillHubBatchVerifiedModeDisabled:
	default:
		return errors.New("verified mode is invalid")
	}
	options.TagMode = strings.ToLower(strings.TrimSpace(options.TagMode))
	if options.TagMode == "" {
		options.TagMode = skillHubBatchTagModeManifest
	}
	switch options.TagMode {
	case skillHubBatchTagModeManifest, skillHubBatchTagModeAppend, skillHubBatchTagModeReplace:
	default:
		return errors.New("tag mode is invalid")
	}
	for _, tag := range cleanSkillHubBatchTags(options.CommonTags) {
		if err := model.ValidateSkillHubTag(&model.SkillHubTag{Name: tag}); err != nil {
			return err
		}
	}
	options.CommonTags = cleanSkillHubBatchTags(options.CommonTags)
	options.Origin = strings.TrimSpace(options.Origin)
	if len([]rune(options.Origin)) > 64 {
		return errors.New("skill origin must be 64 characters or fewer")
	}
	for _, policy := range []*string{&options.MissingIcon, &options.MissingTestcases, &options.MissingEvaluation} {
		*policy = strings.ToLower(strings.TrimSpace(*policy))
		if *policy == "" {
			*policy = skillHubBatchMissingPolicyRetain
		}
		if *policy != skillHubBatchMissingPolicyRetain && *policy != skillHubBatchMissingPolicyClear {
			return errors.New("missing resource policy must be retain or clear")
		}
	}
	if _, err := skillHubBatchSortValue(*options, skillHubBatchImportMaxItems-1); err != nil {
		return err
	}
	return nil
}

func applySkillHubBatchOptions(request *skillHubSkillRequest, options skillHubBatchImportOptionsRequest, index int, existing *model.SkillHubSkill) error {
	request.Published = options.Published
	request.Status = nil
	request.Recommended = options.Recommended
	sort, err := skillHubBatchSortValue(options, index)
	if err != nil {
		return err
	}
	request.Sort = sort
	switch options.VerifiedMode {
	case skillHubBatchVerifiedModeVerified:
		request.Verified = true
	case skillHubBatchVerifiedModeDisabled:
		request.Verified = false
	}
	switch options.TagMode {
	case skillHubBatchTagModeAppend:
		request.Tags = mergeSkillHubBatchTags(request.Tags, options.CommonTags)
	case skillHubBatchTagModeReplace:
		request.Tags = append([]string(nil), options.CommonTags...)
	default:
		request.Tags = cleanSkillHubBatchTags(request.Tags)
	}
	if options.OverrideOrigin {
		request.Origin = options.Origin
	}
	if existing != nil && request.Testcases == nil && options.MissingTestcases == skillHubBatchMissingPolicyRetain {
		value, parseErr := model.SkillHubTestcasesFromJSON(existing.TestcasesJSON)
		if parseErr != nil {
			return parseErr
		}
		request.Testcases = value
	}
	if existing != nil && request.Evaluation == nil && options.MissingEvaluation == skillHubBatchMissingPolicyRetain {
		value, parseErr := model.SkillHubEvaluationFromJSON(existing.EvaluationJSON)
		if parseErr != nil {
			return parseErr
		}
		request.Evaluation = value
	}
	return nil
}

func skillHubBatchSortValue(options skillHubBatchImportOptionsRequest, index int) (int, error) {
	const (
		minSort = int64(-2147483648)
		maxSort = int64(2147483647)
	)
	for _, value := range []int64{options.FixedSort, options.SortStart, options.SortStep} {
		if value < minSort || value > maxSort {
			return 0, errors.New("batch sort values must fit in a 32-bit integer")
		}
	}
	value := options.FixedSort
	if options.SortMode == skillHubBatchSortModeSequence {
		if index < 0 || index >= skillHubBatchImportMaxItems {
			return 0, errors.New("batch item index is out of range")
		}
		value = options.SortStart + int64(index)*options.SortStep
	}
	if value < minSort || value > maxSort {
		return 0, errors.New("batch sort result must fit in a 32-bit integer")
	}
	return int(value), nil
}

func preflightSkillHubBatchItem(
	c *gin.Context,
	item skillHubBatchImportCommitItemRequest,
	options skillHubBatchImportOptionsRequest,
	existing *model.SkillHubSkill,
) error {
	request := item.Skill
	if err := applySkillHubBatchOptions(&request, options, item.Index, existing); err != nil {
		return err
	}
	request.Source = model.SkillHubSource{
		Type: "zip",
		URL:  skillHubDownloadURL(c, strings.TrimSpace(request.ID)),
	}
	if strings.TrimSpace(item.IconUploadTicket) != "" {
		request.Icon = ""
	} else if existing != nil && options.MissingIcon == skillHubBatchMissingPolicyRetain {
		request.Icon = existing.Icon
	} else {
		request.Icon = ""
	}
	request.Tags = cleanSkillHubBatchTags(request.Tags)
	for _, tag := range request.Tags {
		if err := model.ValidateSkillHubTag(&model.SkillHubTag{Name: tag}); err != nil {
			return err
		}
	}
	skill, err := skillHubRequestToModel(request, existing)
	if err != nil {
		return err
	}
	return model.ValidateSkillHubSkill(skill)
}

func completeSkillHubBatchUploads(ctx context.Context, items []skillHubBatchImportCommitItemRequest, eligible []bool) []skillHubBatchCompletedUploads {
	results := make([]skillHubBatchCompletedUploads, len(items))
	jobs := make(chan int)
	var workers sync.WaitGroup
	workerCount := skillHubBatchValidationWorkers
	if len(items) < workerCount {
		workerCount = len(items)
	}
	workers.Add(workerCount)
	for worker := 0; worker < workerCount; worker++ {
		go func() {
			defer workers.Done()
			for index := range jobs {
				if !eligible[index] {
					continue
				}
				select {
				case <-ctx.Done():
					results[index].Err = ctx.Err()
					continue
				default:
				}
				item := items[index]
				zipResult, err := service.CompleteSkillHubDirectUpload(item.ZipUploadTicket)
				if err != nil {
					results[index].Err = err
					continue
				}
				if zipResult.Kind != service.SkillHubUploadKindZip || zipResult.SkillID != strings.TrimSpace(item.Skill.ID) {
					results[index].Err = errors.New("zip upload ticket does not match the skill")
					continue
				}
				results[index].Zip = zipResult
				if strings.TrimSpace(item.IconUploadTicket) == "" {
					continue
				}
				iconResult, iconErr := service.CompleteSkillHubDirectUpload(item.IconUploadTicket)
				if iconErr != nil {
					results[index].Err = iconErr
					continue
				}
				if iconResult.Kind != service.SkillHubUploadKindIcon || iconResult.SkillID != strings.TrimSpace(item.Skill.ID) {
					results[index].Err = errors.New("icon upload ticket does not match the skill")
					continue
				}
				results[index].Icon = iconResult
			}
		}()
	}
	for index := range items {
		if eligible[index] {
			jobs <- index
		}
	}
	close(jobs)
	workers.Wait()
	return results
}

func saveSkillHubBatchItem(request skillHubSkillRequest, existing *model.SkillHubSkill) (*model.SkillHubSkillResponse, string, error) {
	action := "create"
	if existing != nil {
		action = "update"
	}
	skill, err := skillHubRequestToModel(request, existing)
	if err != nil {
		return nil, action, err
	}
	if err := model.ValidateSkillHubSkill(skill); err != nil {
		return nil, action, err
	}
	duplicated, err := model.IsSkillHubSkillIDDuplicated(skill.Id, skill.SkillID)
	if err != nil {
		return nil, action, err
	}
	if duplicated {
		return nil, action, errors.New("skill id already exists")
	}
	promotion, err := service.PromoteSkillHubObjects(skill)
	if err != nil {
		return nil, action, err
	}
	if existing == nil {
		if err := skill.Insert(); err != nil {
			cleanupPromotedSkillHubFinalObjects(promotion)
			return nil, action, err
		}
		cleanupPromotedSkillHubTempObjects(promotion)
		response := skill.ToResponse(true)
		return &response, action, nil
	}
	oldSourceRef, oldIcon, err := skill.UpdateReturningPreviousObjects()
	if err != nil {
		cleanupPromotedSkillHubFinalObjects(promotion)
		return nil, action, err
	}
	cleanupPromotedSkillHubTempObjects(promotion)
	cleanupSkillHubObjectsIfChanged(oldSourceRef, oldIcon, skill.SourceRef, skill.Icon)
	response := skill.ToResponse(true)
	return &response, action, nil
}

func discardSkillHubBatchItemTickets(item skillHubBatchImportCommitItemRequest) {
	for index, ticket := range []string{item.ZipUploadTicket, item.IconUploadTicket} {
		if strings.TrimSpace(ticket) != "" {
			if err := service.DiscardSkillHubDirectUpload(ticket); err != nil {
				kind := service.SkillHubUploadKindZip
				if index == 1 {
					kind = service.SkillHubUploadKindIcon
				}
				common.SysLog(fmt.Sprintf(
					"skill hub batch %s temporary object cleanup failed for %s: %v",
					kind,
					strings.TrimSpace(item.Skill.ID),
					err,
				))
			}
		}
	}
}

func cleanSkillHubBatchTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	result := make([]string, 0, len(tags))
	for _, rawTag := range tags {
		tag := strings.TrimSpace(rawTag)
		key := strings.ToLower(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, tag)
	}
	return result
}

func mergeSkillHubBatchTags(first []string, second []string) []string {
	return cleanSkillHubBatchTags(append(append([]string(nil), first...), second...))
}
