package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

type clientReleaseRequest struct {
	Revision         int64  `json:"revision"`
	Version          string `json:"version"`
	Platform         string `json:"platform"`
	Arch             string `json:"arch"`
	Channel          string `json:"channel"`
	FileName         string `json:"fileName"`
	ObjectKey        string `json:"objectKey"`
	Size             int64  `json:"size"`
	SHA256           string `json:"sha256"`
	SHA512           string `json:"sha512"`
	UpdaterFileName  string `json:"updaterFileName"`
	UpdaterObjectKey string `json:"updaterObjectKey"`
	UpdaterSize      int64  `json:"updaterSize"`
	UpdaterSHA256    string `json:"updaterSha256"`
	UpdaterSHA512    string `json:"updaterSha512"`
	ReleaseNotes     string `json:"releaseNotes"`
	MinVersion       string `json:"minVersion"`
	Forced           bool   `json:"forced"`
}

type clientReleasePromotions struct {
	installer *service.ClientReleasePromoteResult
	updater   *service.ClientReleasePromoteResult
}

type clientReleaseDirectUploadInitRequest struct {
	Version  string `json:"version"`
	Platform string `json:"platform"`
	Arch     string `json:"arch"`
	Channel  string `json:"channel"`
	FileName string `json:"fileName"`
	Size     int64  `json:"size"`
}

type clientReleaseDirectUploadCompleteRequest struct {
	UploadTicket string `json:"uploadTicket"`
}

type electronLatestYAML struct {
	Version     string                   `yaml:"version"`
	Files       []electronLatestYAMLFile `yaml:"files"`
	Path        string                   `yaml:"path"`
	SHA512      string                   `yaml:"sha512"`
	ReleaseDate string                   `yaml:"releaseDate"`
}

type electronLatestYAMLFile struct {
	URL    string `yaml:"url"`
	SHA512 string `yaml:"sha512"`
	Size   int64  `yaml:"size"`
}

func ListClientReleases(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	releases, total, err := model.SearchClientReleases(
		c.Query("keyword"),
		c.Query("platform"),
		c.Query("arch"),
		c.Query("channel"),
		false,
		pageInfo.GetStartIdx(),
		pageInfo.GetPageSize(),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, model.ClientReleaseListResponse{
		Items: model.ClientReleasesToResponses(releases, false, func(release *model.ClientRelease) model.ClientReleaseURLs {
			return clientReleaseURLs(c, release)
		}),
		Total: total,
	})
}

func GetLatestClientRelease(c *gin.Context) {
	latest, err := model.GetLatestClientRelease(
		c.Query("platform"),
		c.Query("arch"),
		c.Query("channel"),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	currentVersion := strings.TrimSpace(c.Query("current_version"))
	if currentVersion == "" {
		currentVersion = strings.TrimSpace(c.Query("version"))
	}
	if latest == nil {
		common.ApiSuccess(c, gin.H{
			"updateAvailable": false,
			"currentVersion":  currentVersion,
		})
		return
	}
	updateAvailable := currentVersion == "" || model.CompareClientVersions(latest.Version, currentVersion) > 0
	forceUpdate := false
	if latest.Forced && latest.MinVersion != "" && currentVersion != "" {
		forceUpdate = model.CompareClientVersions(currentVersion, latest.MinVersion) < 0
		if forceUpdate {
			updateAvailable = true
		}
	}
	common.ApiSuccess(c, gin.H{
		"updateAvailable": updateAvailable,
		"forceUpdate":     forceUpdate,
		"currentVersion":  currentVersion,
		"latest":          latest.ToResponse(false, clientReleaseURLs(c, latest)),
	})
}

func GetClientReleaseLatestYAML(c *gin.Context) {
	serveClientReleaseLatestYAML(c, false)
}

func GetClientReleaseLatestMacYAML(c *gin.Context) {
	serveClientReleaseLatestYAML(c, true)
}

func serveClientReleaseLatestYAML(c *gin.Context, requireMacOS bool) {
	platform := model.NormalizeClientReleasePlatform(c.Param("platform"))
	if requireMacOS && platform != "macos" {
		c.String(http.StatusBadRequest, "latest-mac.yml is only available for macos")
		return
	}
	latest, err := model.GetLatestClientRelease(
		platform,
		c.Param("arch"),
		c.Param("channel"),
	)
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	if latest == nil {
		c.String(http.StatusNotFound, "client release not found")
		return
	}
	fileName := latest.FileName
	sha512 := latest.SHA512
	size := latest.Size
	if platform == "macos" {
		fileName = latest.UpdaterFileName
		sha512 = latest.UpdaterSHA512
		size = latest.UpdaterSize
	}
	if strings.TrimSpace(fileName) == "" || strings.TrimSpace(sha512) == "" || size <= 0 {
		c.String(http.StatusInternalServerError, "client release updater asset is incomplete")
		return
	}
	assetPath := fmt.Sprintf(
		"download/%d/%s",
		latest.Id,
		url.PathEscape(fileName),
	)
	payload := electronLatestYAML{
		Version: latest.Version,
		Files: []electronLatestYAMLFile{
			{
				URL:    assetPath,
				SHA512: sha512,
				Size:   size,
			},
		},
		Path:        assetPath,
		SHA512:      sha512,
		ReleaseDate: clientReleaseTime(latest).Format(time.RFC3339),
	}
	body, err := yaml.Marshal(payload)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Content-Type", "text/yaml; charset=utf-8")
	c.String(http.StatusOK, string(body))
}

func DownloadClientRelease(c *gin.Context) {
	release, ok := publishedClientReleaseByParam(c, "id")
	if !ok {
		return
	}
	redirectClientReleaseAsset(c, release.ObjectKey, release.FileName)
}

func DownloadClientReleaseAsset(c *gin.Context) {
	release, ok := publishedClientReleaseByParam(c, "id")
	if !ok {
		return
	}
	if model.ClientReleaseTarget(release.Platform, release.Arch, release.Channel) != model.ClientReleaseTarget(c.Param("platform"), c.Param("arch"), c.Param("channel")) {
		common.ApiErrorMsg(c, "client release asset target does not match")
		return
	}
	fileName := strings.TrimSpace(c.Param("filename"))
	switch fileName {
	case release.FileName:
		redirectClientReleaseAsset(c, release.ObjectKey, release.FileName)
	case release.UpdaterFileName:
		if fileName == "" || release.UpdaterObjectKey == "" {
			common.ApiErrorMsg(c, "client release asset not found")
			return
		}
		redirectClientReleaseAsset(c, release.UpdaterObjectKey, release.UpdaterFileName)
	default:
		common.ApiErrorMsg(c, "client release asset not found")
	}
}

func AdminListClientReleases(c *gin.Context) {
	platform, err := requiredAdminClientReleasePlatform(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo := common.GetPageQuery(c)
	releases, total, err := model.SearchClientReleases(
		c.Query("keyword"),
		platform,
		c.Query("arch"),
		c.Query("channel"),
		true,
		pageInfo.GetStartIdx(),
		pageInfo.GetPageSize(),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, model.ClientReleaseListResponse{
		Items: model.ClientReleasesToResponses(releases, true, func(release *model.ClientRelease) model.ClientReleaseURLs {
			return clientReleaseURLs(c, release)
		}),
		Total: total,
	})
}

func AdminGetClientRelease(c *gin.Context) {
	release, err := adminClientReleaseByParam(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, release.ToResponse(true, clientReleaseURLs(c, release)))
}

func AdminInitClientReleaseDirectUpload(c *gin.Context) {
	var request clientReleaseDirectUploadInitRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.InitClientReleaseDirectUpload(service.ClientReleaseDirectUploadInput{
		ActorUserID: c.GetInt("id"),
		Version:     request.Version,
		Platform:    request.Platform,
		Arch:        request.Arch,
		Channel:     request.Channel,
		FileName:    request.FileName,
		Size:        request.Size,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func AdminCompleteClientReleaseDirectUpload(c *gin.Context) {
	var request clientReleaseDirectUploadCompleteRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.CompleteClientReleaseDirectUpload(request.UploadTicket, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func AdminDiscardClientReleaseDirectUpload(c *gin.Context) {
	var request clientReleaseDirectUploadCompleteRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := service.DiscardClientReleaseDirectUpload(request.UploadTicket, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func AdminCreateClientRelease(c *gin.Context) {
	var request clientReleaseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validateClientReleaseRequestTarget(request, nil); err != nil {
		common.ApiError(c, err)
		return
	}
	release := clientReleaseRequestToModel(request, nil)
	promotions, err := promoteClientReleaseAssets(release)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validateClientReleaseAssetChanges(nil, release, promotions); err != nil {
		cleanupPromotedClientReleaseFinals(promotions)
		common.ApiError(c, err)
		return
	}
	if err := release.Insert(); err != nil {
		cleanupPromotedClientReleaseFinals(promotions)
		common.ApiError(c, err)
		return
	}
	cleanupPromotedClientReleaseTemps(promotions)
	common.ApiSuccess(c, release.ToResponse(true, clientReleaseURLs(c, release)))
}

func AdminUpdateClientRelease(c *gin.Context) {
	release, err := adminClientReleaseByParam(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var request clientReleaseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validateClientReleaseRequestTarget(request, release); err != nil {
		common.ApiError(c, err)
		return
	}
	existing := release
	release = clientReleaseRequestToModel(request, existing)
	promotions, err := promoteClientReleaseAssets(release)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := validateClientReleaseAssetChanges(existing, release, promotions); err != nil {
		cleanupPromotedClientReleaseFinals(promotions)
		common.ApiError(c, err)
		return
	}
	oldObjectKeys, err := release.UpdateReturningPreviousObjectKeysFromRevision(c.GetInt("id"), model.ClientReleaseObjectKeys{
		Installer: existing.ObjectKey,
		Updater:   existing.UpdaterObjectKey,
	}, request.Revision)
	if err != nil {
		cleanupPromotedClientReleaseFinals(promotions)
		if errors.Is(err, model.ErrClientReleasePublishPermissionRequired) {
			common.ApiErrorI18n(c, i18n.MsgAuthInsufficientPrivilege)
			return
		}
		if errors.Is(err, model.ErrClientReleaseRevisionConflict) {
			common.ApiErrorMsg(c, "client release changed; refresh before saving again")
			return
		}
		common.ApiError(c, err)
		return
	}
	cleanupPromotedClientReleaseTemps(promotions)
	cleanupClientReleaseObjectIfChanged(oldObjectKeys.Installer, release.ObjectKey)
	cleanupClientReleaseObjectIfChanged(oldObjectKeys.Updater, release.UpdaterObjectKey)
	common.ApiSuccess(c, release.ToResponse(true, clientReleaseURLs(c, release)))
}

func AdminDeleteClientRelease(c *gin.Context) {
	release, err := adminClientReleaseByParam(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	objectKeys, err := model.DeleteClientReleaseReturningObjectKeysForPlatform(release.Id, release.Platform, c.GetInt("id"))
	if err != nil {
		if errors.Is(err, model.ErrClientReleasePublishPermissionRequired) {
			common.ApiErrorI18n(c, i18n.MsgAuthInsufficientPrivilege)
			return
		}
		common.ApiError(c, err)
		return
	}
	cleanupClientReleaseObject(objectKeys.Installer)
	cleanupClientReleaseObject(objectKeys.Updater)
	common.ApiSuccess(c, nil)
}

func AdminPublishClientRelease(c *gin.Context) {
	updateClientReleasePublishStatus(c, model.ClientReleaseStatusPublished)
}

func AdminUnpublishClientRelease(c *gin.Context) {
	updateClientReleasePublishStatus(c, model.ClientReleaseStatusDraft)
}

func updateClientReleasePublishStatus(c *gin.Context, status int) {
	existing, err := adminClientReleaseByParam(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	release, err := model.UpdateClientReleaseStatusForPlatform(existing.Id, existing.Platform, status, c.GetInt("id"))
	if err != nil {
		if errors.Is(err, model.ErrClientReleasePublishPermissionRequired) {
			common.ApiErrorI18n(c, i18n.MsgAuthInsufficientPrivilege)
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, release.ToResponse(true, clientReleaseURLs(c, release)))
}

func clientReleaseRequestToModel(request clientReleaseRequest, existing *model.ClientRelease) *model.ClientRelease {
	release := &model.ClientRelease{}
	if existing != nil {
		copy := *existing
		release = &copy
	}
	release.Revision = request.Revision
	release.Version = strings.TrimSpace(request.Version)
	release.Platform = strings.TrimSpace(request.Platform)
	release.Arch = strings.TrimSpace(request.Arch)
	release.Channel = strings.TrimSpace(request.Channel)
	release.FileName = strings.TrimSpace(request.FileName)
	release.ObjectKey = strings.TrimSpace(request.ObjectKey)
	release.Size = request.Size
	release.SHA256 = strings.TrimSpace(request.SHA256)
	release.SHA512 = strings.TrimSpace(request.SHA512)
	release.UpdaterFileName = strings.TrimSpace(request.UpdaterFileName)
	release.UpdaterObjectKey = strings.TrimSpace(request.UpdaterObjectKey)
	release.UpdaterSize = request.UpdaterSize
	release.UpdaterSHA256 = strings.TrimSpace(request.UpdaterSHA256)
	release.UpdaterSHA512 = strings.TrimSpace(request.UpdaterSHA512)
	release.ReleaseNotes = strings.TrimSpace(request.ReleaseNotes)
	release.MinVersion = strings.TrimSpace(request.MinVersion)
	release.Forced = request.Forced
	if existing == nil {
		release.Status = model.ClientReleaseStatusDraft
	}
	return release
}

func clientReleaseByParam(c *gin.Context) (*model.ClientRelease, error) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		return nil, fmt.Errorf("invalid client release id")
	}
	return model.GetClientReleaseByID(id)
}

func adminClientReleaseByParam(c *gin.Context) (*model.ClientRelease, error) {
	platform, err := requiredAdminClientReleasePlatform(c)
	if err != nil {
		return nil, err
	}
	release, err := clientReleaseByParam(c)
	if err != nil {
		return nil, err
	}
	if model.NormalizeClientReleasePlatform(release.Platform) != platform {
		return nil, errors.New("client release does not belong to the selected platform")
	}
	return release, nil
}

func requiredAdminClientReleasePlatform(c *gin.Context) (string, error) {
	platform := strings.TrimSpace(c.Query("platform"))
	if platform == "" {
		return "", errors.New("client release platform is required")
	}
	platform = model.NormalizeClientReleasePlatform(platform)
	if !model.IsAllowedClientReleasePlatform(platform) {
		return "", errors.New("client release platform must be windows, macos, or linux")
	}
	return platform, nil
}

func validateClientReleaseRequestTarget(request clientReleaseRequest, existing *model.ClientRelease) error {
	if strings.TrimSpace(request.Platform) == "" {
		return errors.New("client release platform is required")
	}
	if strings.TrimSpace(request.Arch) == "" {
		return errors.New("client release arch is required")
	}
	if strings.TrimSpace(request.Channel) == "" {
		return errors.New("client release channel is required")
	}
	if existing != nil && model.NormalizeClientReleasePlatform(request.Platform) != model.NormalizeClientReleasePlatform(existing.Platform) {
		return errors.New("client release platform cannot be changed")
	}
	if existing != nil && request.Revision != existing.Revision {
		return model.ErrClientReleaseRevisionConflict
	}
	return nil
}

func publishedClientReleaseByParam(c *gin.Context, param string) (*model.ClientRelease, bool) {
	id, err := strconv.Atoi(c.Param(param))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid client release id")
		return nil, false
	}
	release, err := model.GetClientReleaseByID(id)
	if err != nil || release.Status != model.ClientReleaseStatusPublished {
		common.ApiErrorMsg(c, "client release not found")
		return nil, false
	}
	return release, true
}

func redirectClientReleaseAsset(c *gin.Context, objectKey string, fileName string) {
	signedURL, err := service.SignClientReleaseURL(objectKey, fileName)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.Redirect(http.StatusFound, signedURL)
}

func clientReleaseURLs(c *gin.Context, release *model.ClientRelease) model.ClientReleaseURLs {
	if release == nil || release.Status != model.ClientReleaseStatusPublished {
		return model.ClientReleaseURLs{}
	}
	baseURL := requestBaseURL(c)
	urls := model.ClientReleaseURLs{
		DownloadURL: fmt.Sprintf("%s/api/client-releases/download/%d", baseURL, release.Id),
	}
	targetPath := fmt.Sprintf(
		"/api/client-releases/updates/%s/%s/%s",
		url.PathEscape(model.NormalizeClientReleasePlatform(release.Platform)),
		url.PathEscape(model.NormalizeClientReleaseArch(release.Arch)),
		url.PathEscape(model.NormalizeClientReleaseChannel(release.Channel)),
	)
	manifestName := "latest.yml"
	if model.NormalizeClientReleasePlatform(release.Platform) == "macos" {
		manifestName = "latest-mac.yml"
		if release.UpdaterFileName != "" && release.UpdaterObjectKey != "" {
			urls.UpdaterDownloadURL = fmt.Sprintf(
				"%s%s/download/%d/%s",
				baseURL,
				targetPath,
				release.Id,
				url.PathEscape(release.UpdaterFileName),
			)
		}
	}
	urls.UpdateManifestURL = fmt.Sprintf("%s%s/%s", baseURL, targetPath, manifestName)
	return urls
}

func clientReleaseTime(release *model.ClientRelease) time.Time {
	if release.UpdatedTime > 0 {
		return time.Unix(release.UpdatedTime, 0).UTC()
	}
	if release.CreatedTime > 0 {
		return time.Unix(release.CreatedTime, 0).UTC()
	}
	return time.Now().UTC()
}

func promoteClientReleaseAssets(release *model.ClientRelease) (*clientReleasePromotions, error) {
	installer, err := service.PromoteClientReleaseObject(release)
	if err != nil {
		return nil, err
	}
	updater, err := service.PromoteClientReleaseUpdaterObject(release)
	if err != nil {
		cleanupPromotedClientReleaseFinal(installer)
		return nil, err
	}
	return &clientReleasePromotions{installer: installer, updater: updater}, nil
}

func validateClientReleaseAssetChanges(existing *model.ClientRelease, release *model.ClientRelease, promotions *clientReleasePromotions) error {
	targetChanged := existing != nil && (existing.Version != release.Version || model.ClientReleaseTarget(existing.Platform, existing.Arch, existing.Channel) != model.ClientReleaseTarget(release.Platform, release.Arch, release.Channel))
	if promotions == nil || promotions.installer == nil || !promotions.installer.Promoted {
		if existing == nil || targetChanged || release.ObjectKey != existing.ObjectKey {
			return errors.New("client release installer must come from a completed direct upload")
		}
		if release.FileName != existing.FileName || release.Size != existing.Size || release.SHA256 != existing.SHA256 || release.SHA512 != existing.SHA512 {
			return errors.New("client release installer metadata cannot change without a completed direct upload")
		}
	}
	if release.UpdaterObjectKey == "" {
		return nil
	}
	if promotions.updater == nil || !promotions.updater.Promoted {
		if existing == nil || targetChanged || release.UpdaterObjectKey != existing.UpdaterObjectKey {
			return errors.New("client release updater must come from a completed direct upload")
		}
		if release.UpdaterFileName != existing.UpdaterFileName || release.UpdaterSize != existing.UpdaterSize || release.UpdaterSHA256 != existing.UpdaterSHA256 || release.UpdaterSHA512 != existing.UpdaterSHA512 {
			return errors.New("client release updater metadata cannot change without a completed direct upload")
		}
	}
	return nil
}

func cleanupClientReleaseObjectIfChanged(oldObjectKey string, newObjectKey string) {
	if strings.TrimSpace(oldObjectKey) == "" || strings.TrimSpace(oldObjectKey) == strings.TrimSpace(newObjectKey) {
		return
	}
	cleanupClientReleaseObject(oldObjectKey)
}

func cleanupClientReleaseObject(objectKey string) {
	if strings.TrimSpace(objectKey) == "" {
		return
	}
	if err := service.DeleteClientReleaseObject(objectKey); err != nil {
		common.SysLog(fmt.Sprintf("client release OSS object cleanup failed for %s: %v", objectKey, err))
	}
}

func cleanupPromotedClientReleaseFinal(result *service.ClientReleasePromoteResult) {
	if result == nil || !result.Promoted {
		return
	}
	cleanupClientReleaseObject(result.FinalObject)
}

func cleanupPromotedClientReleaseTemp(result *service.ClientReleasePromoteResult) {
	if result == nil || !result.Promoted {
		return
	}
	cleanupClientReleaseObject(result.TempObject)
}

func cleanupPromotedClientReleaseFinals(promotions *clientReleasePromotions) {
	if promotions == nil {
		return
	}
	cleanupPromotedClientReleaseFinal(promotions.installer)
	cleanupPromotedClientReleaseFinal(promotions.updater)
}

func cleanupPromotedClientReleaseTemps(promotions *clientReleasePromotions) {
	if promotions == nil {
		return
	}
	cleanupPromotedClientReleaseTemp(promotions.installer)
	cleanupPromotedClientReleaseTemp(promotions.updater)
}
