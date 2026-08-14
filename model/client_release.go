package model

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"gorm.io/gorm"
)

const (
	ClientReleaseStatusDraft     = 0
	ClientReleaseStatusPublished = 1
	clientReleaseKeywordMaxRunes = 128
	defaultClientReleaseChannel  = "stable"
)

var clientReleaseVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

var ErrClientReleasePublishPermissionRequired = errors.New("client release publish permission is required")
var ErrClientReleaseRevisionConflict = errors.New("client release was changed by another request")

type ClientRelease struct {
	Id               int            `json:"id" gorm:"primaryKey"`
	Version          string         `json:"version" gorm:"size:64;not null;uniqueIndex:uk_client_release_version_target_delete_at,priority:1"`
	Platform         string         `json:"platform" gorm:"size:32;not null;index;uniqueIndex:uk_client_release_version_target_delete_at,priority:2"`
	Arch             string         `json:"arch" gorm:"size:32;not null;index;uniqueIndex:uk_client_release_version_target_delete_at,priority:3"`
	Channel          string         `json:"channel" gorm:"size:32;not null;default:stable;index;uniqueIndex:uk_client_release_version_target_delete_at,priority:4"`
	FileName         string         `json:"fileName" gorm:"size:255;not null"`
	ObjectKey        string         `json:"-" gorm:"column:object_key;type:text;not null"`
	Size             int64          `json:"size" gorm:"bigint;not null;default:0"`
	SHA256           string         `json:"sha256,omitempty" gorm:"size:128"`
	SHA512           string         `json:"sha512,omitempty" gorm:"type:text"`
	UpdaterFileName  string         `json:"updaterFileName,omitempty" gorm:"size:255"`
	UpdaterObjectKey string         `json:"-" gorm:"column:updater_object_key;type:text"`
	UpdaterSize      int64          `json:"updaterSize,omitempty" gorm:"bigint;not null;default:0"`
	UpdaterSHA256    string         `json:"updaterSha256,omitempty" gorm:"size:128"`
	UpdaterSHA512    string         `json:"updaterSha512,omitempty" gorm:"type:text"`
	ReleaseNotes     string         `json:"releaseNotes,omitempty" gorm:"type:text"`
	MinVersion       string         `json:"minVersion,omitempty" gorm:"size:64"`
	Forced           bool           `json:"forced" gorm:"default:false"`
	Status           int            `json:"status" gorm:"default:0;index"`
	Revision         int64          `json:"revision" gorm:"not null;default:0"`
	CreatedTime      int64          `json:"createdTime" gorm:"bigint"`
	UpdatedTime      int64          `json:"updatedTime" gorm:"bigint"`
	DeletedAt        gorm.DeletedAt `json:"-" gorm:"index;uniqueIndex:uk_client_release_version_target_delete_at,priority:5"`
}

type ClientReleaseResponse struct {
	ID                 int    `json:"id"`
	Version            string `json:"version"`
	Platform           string `json:"platform"`
	Arch               string `json:"arch"`
	Channel            string `json:"channel"`
	FileName           string `json:"fileName"`
	ObjectKey          string `json:"objectKey,omitempty"`
	DownloadURL        string `json:"downloadUrl,omitempty"`
	UpdaterDownloadURL string `json:"updaterDownloadUrl,omitempty"`
	UpdateManifestURL  string `json:"updateManifestUrl,omitempty"`
	Size               int64  `json:"size"`
	SHA256             string `json:"sha256,omitempty"`
	SHA512             string `json:"sha512,omitempty"`
	UpdaterFileName    string `json:"updaterFileName,omitempty"`
	UpdaterObjectKey   string `json:"updaterObjectKey,omitempty"`
	UpdaterSize        int64  `json:"updaterSize,omitempty"`
	UpdaterSHA256      string `json:"updaterSha256,omitempty"`
	UpdaterSHA512      string `json:"updaterSha512,omitempty"`
	ReleaseNotes       string `json:"releaseNotes,omitempty"`
	MinVersion         string `json:"minVersion,omitempty"`
	Forced             bool   `json:"forced"`
	Published          bool   `json:"published,omitempty"`
	Status             int    `json:"status,omitempty"`
	Revision           int64  `json:"revision"`
	CreatedAt          string `json:"createdAt,omitempty"`
	UpdatedAt          string `json:"updatedAt,omitempty"`
}

type ClientReleaseObjectKeys struct {
	Installer string
	Updater   string
}

type ClientReleaseURLs struct {
	DownloadURL        string
	UpdaterDownloadURL string
	UpdateManifestURL  string
}

type ClientReleaseListResponse struct {
	Items []ClientReleaseResponse `json:"items"`
	Total int64                   `json:"total"`
}

func (r *ClientRelease) BeforeSave(tx *gorm.DB) error {
	if strings.TrimSpace(r.Channel) == "" {
		return errors.New("client release channel is required")
	}
	r.Version = NormalizeClientReleaseVersion(r.Version)
	r.Platform = NormalizeClientReleasePlatform(r.Platform)
	r.Arch = NormalizeClientReleaseArch(r.Arch)
	r.Channel = NormalizeClientReleaseChannel(r.Channel)
	r.FileName = cleanClientReleaseFileName(r.FileName)
	r.ObjectKey = strings.TrimLeft(strings.TrimSpace(r.ObjectKey), "/")
	r.SHA256 = strings.TrimSpace(r.SHA256)
	r.SHA512 = strings.TrimSpace(r.SHA512)
	r.UpdaterFileName = cleanClientReleaseFileName(r.UpdaterFileName)
	r.UpdaterObjectKey = strings.TrimLeft(strings.TrimSpace(r.UpdaterObjectKey), "/")
	r.UpdaterSHA256 = strings.TrimSpace(r.UpdaterSHA256)
	r.UpdaterSHA512 = strings.TrimSpace(r.UpdaterSHA512)
	r.MinVersion = NormalizeClientReleaseVersion(r.MinVersion)
	r.ReleaseNotes = strings.TrimSpace(r.ReleaseNotes)
	if err := ValidateClientRelease(r); err != nil {
		return err
	}
	if r.Revision <= 0 {
		r.Revision = 1
	}
	now := common.GetTimestamp()
	if r.CreatedTime == 0 {
		r.CreatedTime = now
	}
	r.UpdatedTime = now
	return nil
}

func ValidateClientRelease(r *ClientRelease) error {
	if r.Version == "" {
		return errors.New("client release version is required")
	}
	if err := ValidateClientReleaseVersion(r.Version); err != nil {
		return err
	}
	if !IsAllowedClientReleasePlatform(r.Platform) {
		return errors.New("client release platform must be windows, macos, or linux")
	}
	if !IsAllowedClientReleaseArch(r.Arch) {
		return errors.New("client release arch must be x64, arm64, ia32, or universal")
	}
	if !IsAllowedClientReleaseChannel(r.Channel) {
		return errors.New("client release channel must be stable or beta")
	}
	if r.MinVersion != "" {
		if err := ValidateClientReleaseVersion(r.MinVersion); err != nil {
			return fmt.Errorf("client release min version is invalid: %w", err)
		}
	}
	if r.FileName == "" {
		return errors.New("client release file name is required")
	}
	if expected := ClientReleaseExpectedFileName(r.Version, r.Platform, r.Arch, r.Channel, r.FileName); r.FileName != expected {
		return fmt.Errorf("client release installer file name does not match target; expected %s", expected)
	}
	if !IsAllowedClientReleaseInstallerFile(r.Platform, r.FileName) {
		return fmt.Errorf("client release installer file type is not supported for %s", NormalizeClientReleasePlatform(r.Platform))
	}
	if r.ObjectKey == "" {
		return errors.New("client release OSS object is required")
	}
	if r.Size <= 0 {
		return errors.New("client release package size is required")
	}
	switch r.Status {
	case ClientReleaseStatusDraft, ClientReleaseStatusPublished:
	default:
		return errors.New("client release status is invalid")
	}
	if r.Status == ClientReleaseStatusPublished && r.SHA512 == "" {
		return errors.New("client release sha512 is required before publishing")
	}
	updaterAssetPresent := r.UpdaterFileName != "" || r.UpdaterObjectKey != "" || r.UpdaterSize != 0 || r.UpdaterSHA256 != "" || r.UpdaterSHA512 != ""
	if updaterAssetPresent {
		if NormalizeClientReleasePlatform(r.Platform) != "macos" {
			return errors.New("client release updater asset is only supported for macos")
		}
		if !strings.HasSuffix(strings.ToLower(r.UpdaterFileName), ".zip") {
			return errors.New("macos client release updater package must be a zip file")
		}
		if r.UpdaterObjectKey == "" || r.UpdaterSize <= 0 || r.UpdaterSHA512 == "" {
			return errors.New("macos client release updater package metadata is incomplete")
		}
		if expected := ClientReleaseExpectedFileName(r.Version, r.Platform, r.Arch, r.Channel, r.UpdaterFileName); r.UpdaterFileName != expected {
			return fmt.Errorf("client release updater file name does not match target; expected %s", expected)
		}
	}
	if NormalizeClientReleasePlatform(r.Platform) == "macos" {
		if !strings.HasSuffix(strings.ToLower(r.FileName), ".dmg") {
			return errors.New("macos client release installer must be a dmg file")
		}
		if r.Status == ClientReleaseStatusPublished && !updaterAssetPresent {
			return errors.New("macos client release updater zip is required before publishing")
		}
	}
	return nil
}

func NormalizeClientReleaseVersion(value string) string {
	return strings.TrimSpace(value)
}

func ValidateClientReleaseVersion(value string) error {
	value = NormalizeClientReleaseVersion(value)
	if value == "" {
		return errors.New("client release version is required")
	}
	if !clientReleaseVersionPattern.MatchString(value) {
		return errors.New("client release version must use three numeric segments, such as 1.2.3")
	}
	return nil
}

func NormalizeClientReleasePlatform(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "win", "win32", "windows":
		return "windows"
	case "darwin", "mac", "macos", "osx":
		return "macos"
	default:
		return value
	}
}

func NormalizeClientReleaseArch(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "amd64":
		return "x64"
	case "aarch64":
		return "arm64"
	default:
		return value
	}
}

func NormalizeClientReleaseChannel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return defaultClientReleaseChannel
	}
	return value
}

func IsAllowedClientReleaseChannel(value string) bool {
	switch NormalizeClientReleaseChannel(value) {
	case "stable", "beta":
		return true
	default:
		return false
	}
}

func IsAllowedClientReleasePlatform(value string) bool {
	switch NormalizeClientReleasePlatform(value) {
	case "windows", "macos", "linux":
		return true
	default:
		return false
	}
}

func clientReleasePlatformAliases(platform string) []string {
	platform = NormalizeClientReleasePlatform(platform)
	if platform == "" {
		return nil
	}
	if platform == "windows" {
		return []string{"windows", "win32"}
	}
	if platform == "macos" {
		return []string{"macos", "darwin"}
	}
	return []string{platform}
}

func IsAllowedClientReleaseArch(value string) bool {
	switch NormalizeClientReleaseArch(value) {
	case "x64", "arm64", "ia32", "universal":
		return true
	default:
		return false
	}
}

func (r *ClientRelease) Insert() error {
	return DB.Create(r).Error
}

func (r *ClientRelease) Update() error {
	return DB.Save(r).Error
}

func (r *ClientRelease) UpdateReturningPreviousObjectKeys(actorUserId int) (ClientReleaseObjectKeys, error) {
	return r.updateReturningPreviousObjectKeys(actorUserId, nil, nil)
}

func (r *ClientRelease) UpdateReturningPreviousObjectKeysFrom(actorUserId int, expected ClientReleaseObjectKeys) (ClientReleaseObjectKeys, error) {
	return r.updateReturningPreviousObjectKeys(actorUserId, &expected, nil)
}

func (r *ClientRelease) UpdateReturningPreviousObjectKeysFromRevision(actorUserId int, expected ClientReleaseObjectKeys, expectedRevision int64) (ClientReleaseObjectKeys, error) {
	return r.updateReturningPreviousObjectKeys(actorUserId, &expected, &expectedRevision)
}

func (r *ClientRelease) updateReturningPreviousObjectKeys(actorUserId int, expected *ClientReleaseObjectKeys, expectedRevision *int64) (ClientReleaseObjectKeys, error) {
	var previousObjectKeys ClientReleaseObjectKeys
	err := DB.Transaction(func(tx *gorm.DB) error {
		allowPublished, err := hasAnyManagementPermissionTx(
			tx,
			actorUserId,
			true,
			constant.PermissionClientReleasesPublish,
		)
		if err != nil {
			return err
		}
		var current ClientRelease
		if err := lockForUpdate(tx).Where("id = ?", r.Id).First(&current).Error; err != nil {
			return err
		}
		if current.Status == ClientReleaseStatusPublished && !allowPublished {
			return ErrClientReleasePublishPermissionRequired
		}
		if expectedRevision != nil && current.Revision != *expectedRevision {
			return ErrClientReleaseRevisionConflict
		}
		if NormalizeClientReleasePlatform(current.Platform) != NormalizeClientReleasePlatform(r.Platform) {
			return errors.New("client release platform cannot be changed")
		}
		previousObjectKeys = ClientReleaseObjectKeys{
			Installer: current.ObjectKey,
			Updater:   current.UpdaterObjectKey,
		}
		if expected != nil && r.ObjectKey == expected.Installer && current.ObjectKey != expected.Installer {
			r.FileName = current.FileName
			r.ObjectKey = current.ObjectKey
			r.Size = current.Size
			r.SHA256 = current.SHA256
			r.SHA512 = current.SHA512
		}
		if expected != nil && r.UpdaterObjectKey == expected.Updater && current.UpdaterObjectKey != expected.Updater {
			r.UpdaterFileName = current.UpdaterFileName
			r.UpdaterObjectKey = current.UpdaterObjectKey
			r.UpdaterSize = current.UpdaterSize
			r.UpdaterSHA256 = current.UpdaterSHA256
			r.UpdaterSHA512 = current.UpdaterSHA512
		}
		r.CreatedTime = current.CreatedTime
		r.Revision = current.Revision + 1
		// Publishing is a separate capability. Preserve the status observed
		// under the row lock so metadata edits cannot publish, unpublish, or
		// overwrite a concurrent status transition.
		r.Status = current.Status
		return tx.Save(r).Error
	})
	return previousObjectKeys, err
}

func (r *ClientRelease) UpdateReturningPreviousObjectKey(actorUserId int) (string, error) {
	keys, err := r.UpdateReturningPreviousObjectKeys(actorUserId)
	return keys.Installer, err
}

func UpdateClientReleaseStatus(id int, status int, actorUserId int) (*ClientRelease, error) {
	return updateClientReleaseStatus(id, "", status, actorUserId)
}

func UpdateClientReleaseStatusForPlatform(id int, platform string, status int, actorUserId int) (*ClientRelease, error) {
	platform = NormalizeClientReleasePlatform(platform)
	if !IsAllowedClientReleasePlatform(platform) {
		return nil, errors.New("client release platform must be windows, macos, or linux")
	}
	return updateClientReleaseStatus(id, platform, status, actorUserId)
}

func updateClientReleaseStatus(id int, platform string, status int, actorUserId int) (*ClientRelease, error) {
	if status != ClientReleaseStatusDraft && status != ClientReleaseStatusPublished {
		return nil, errors.New("client release status is invalid")
	}
	var release ClientRelease
	err := DB.Transaction(func(tx *gorm.DB) error {
		allowed, err := hasAnyManagementPermissionTx(
			tx,
			actorUserId,
			true,
			constant.PermissionClientReleasesPublish,
		)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrClientReleasePublishPermissionRequired
		}
		query := lockForUpdate(tx).Where("id = ?", id)
		if platform != "" {
			query = query.Where("platform IN ?", clientReleasePlatformAliases(platform))
		}
		if err := query.First(&release).Error; err != nil {
			return err
		}
		release.Status = status
		if status == ClientReleaseStatusPublished {
			if err := ValidateClientRelease(&release); err != nil {
				return err
			}
		}
		release.Revision++
		release.UpdatedTime = common.GetTimestamp()
		return tx.Session(&gorm.Session{SkipHooks: true}).Model(&ClientRelease{}).
			Where("id = ?", release.Id).
			Updates(map[string]any{
				"status":       release.Status,
				"revision":     release.Revision,
				"updated_time": release.UpdatedTime,
			}).Error
	})
	if err != nil {
		return nil, err
	}
	return &release, nil
}

func DeleteClientRelease(id int, actorUserId int) (string, error) {
	keys, err := DeleteClientReleaseReturningObjectKeys(id, actorUserId)
	return keys.Installer, err
}

func DeleteClientReleaseReturningObjectKeys(id int, actorUserId int) (ClientReleaseObjectKeys, error) {
	return deleteClientReleaseReturningObjectKeys(id, "", actorUserId)
}

func DeleteClientReleaseReturningObjectKeysForPlatform(id int, platform string, actorUserId int) (ClientReleaseObjectKeys, error) {
	platform = NormalizeClientReleasePlatform(platform)
	if !IsAllowedClientReleasePlatform(platform) {
		return ClientReleaseObjectKeys{}, errors.New("client release platform must be windows, macos, or linux")
	}
	return deleteClientReleaseReturningObjectKeys(id, platform, actorUserId)
}

func deleteClientReleaseReturningObjectKeys(id int, platform string, actorUserId int) (ClientReleaseObjectKeys, error) {
	var objectKeys ClientReleaseObjectKeys
	err := DB.Transaction(func(tx *gorm.DB) error {
		allowPublished, err := hasAnyManagementPermissionTx(
			tx,
			actorUserId,
			true,
			constant.PermissionClientReleasesPublish,
		)
		if err != nil {
			return err
		}
		var release ClientRelease
		query := lockForUpdate(tx).Where("id = ?", id)
		if platform != "" {
			query = query.Where("platform IN ?", clientReleasePlatformAliases(platform))
		}
		if err := query.First(&release).Error; err != nil {
			return err
		}
		if release.Status == ClientReleaseStatusPublished && !allowPublished {
			return ErrClientReleasePublishPermissionRequired
		}
		objectKeys = ClientReleaseObjectKeys{
			Installer: release.ObjectKey,
			Updater:   release.UpdaterObjectKey,
		}
		return tx.Delete(&release).Error
	})
	return objectKeys, err
}

func GetClientReleaseByID(id int) (*ClientRelease, error) {
	var release ClientRelease
	err := DB.Where("id = ?", id).First(&release).Error
	if err != nil {
		return nil, err
	}
	return &release, nil
}

func SearchClientReleases(keyword string, platform string, arch string, channel string, admin bool, offset int, limit int) ([]*ClientRelease, int64, error) {
	db := DB.Model(&ClientRelease{})
	if !admin {
		db = db.Where("status = ?", ClientReleaseStatusPublished)
	}
	if platform = NormalizeClientReleasePlatform(platform); platform != "" {
		db = db.Where("platform IN ?", clientReleasePlatformAliases(platform))
	}
	if arch = NormalizeClientReleaseArch(arch); arch != "" {
		db = db.Where("arch = ?", arch)
	}
	if channel = strings.TrimSpace(channel); channel != "" {
		db = db.Where("channel = ?", NormalizeClientReleaseChannel(channel))
	}
	like, err := clientReleaseContainsLikePattern(keyword)
	if err != nil {
		return nil, 0, err
	}
	if like != "" {
		db = db.Where(
			"(version LIKE ? ESCAPE '!' OR file_name LIKE ? ESCAPE '!' OR channel LIKE ? ESCAPE '!' OR release_notes LIKE ? ESCAPE '!')",
			like,
			like,
			like,
			like,
		)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var releases []*ClientRelease
	err = db.Order("id DESC").Offset(offset).Limit(limit).Find(&releases).Error
	return releases, total, err
}

func GetLatestClientRelease(platform string, arch string, channel string) (*ClientRelease, error) {
	platform = NormalizeClientReleasePlatform(platform)
	arch = NormalizeClientReleaseArch(arch)
	channel = NormalizeClientReleaseChannel(channel)
	if !IsAllowedClientReleasePlatform(platform) || !IsAllowedClientReleaseArch(arch) {
		return nil, errors.New("client release platform or arch is invalid")
	}
	if !IsAllowedClientReleaseChannel(channel) {
		return nil, errors.New("client release channel must be stable or beta")
	}
	var release ClientRelease
	err := DB.Where(
		"platform IN ? AND arch = ? AND channel = ? AND status = ?",
		clientReleasePlatformAliases(platform),
		arch,
		channel,
		ClientReleaseStatusPublished,
	).Order("id DESC").First(&release).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &release, nil
}

func ClientReleasesToResponses(releases []*ClientRelease, admin bool, releaseURLs func(*ClientRelease) ClientReleaseURLs) []ClientReleaseResponse {
	responses := make([]ClientReleaseResponse, 0, len(releases))
	for _, release := range releases {
		urls := ClientReleaseURLs{}
		if releaseURLs != nil {
			urls = releaseURLs(release)
		}
		responses = append(responses, release.ToResponse(admin, urls))
	}
	return responses
}

func (r *ClientRelease) ToResponse(admin bool, urls ClientReleaseURLs) ClientReleaseResponse {
	response := ClientReleaseResponse{
		ID:                 r.Id,
		Version:            r.Version,
		Platform:           NormalizeClientReleasePlatform(r.Platform),
		Arch:               r.Arch,
		Channel:            r.Channel,
		FileName:           r.FileName,
		ObjectKey:          r.ObjectKey,
		DownloadURL:        urls.DownloadURL,
		UpdaterDownloadURL: urls.UpdaterDownloadURL,
		UpdateManifestURL:  urls.UpdateManifestURL,
		Size:               r.Size,
		SHA256:             r.SHA256,
		SHA512:             r.SHA512,
		UpdaterFileName:    r.UpdaterFileName,
		UpdaterObjectKey:   r.UpdaterObjectKey,
		UpdaterSize:        r.UpdaterSize,
		UpdaterSHA256:      r.UpdaterSHA256,
		UpdaterSHA512:      r.UpdaterSHA512,
		ReleaseNotes:       r.ReleaseNotes,
		MinVersion:         r.MinVersion,
		Forced:             r.Forced,
		Published:          r.Status == ClientReleaseStatusPublished,
		Status:             r.Status,
		Revision:           r.Revision,
	}
	if r.CreatedTime > 0 {
		response.CreatedAt = time.Unix(r.CreatedTime, 0).UTC().Format(time.RFC3339)
	}
	if r.UpdatedTime > 0 {
		response.UpdatedAt = time.Unix(r.UpdatedTime, 0).UTC().Format(time.RFC3339)
	}
	if !admin {
		response.ObjectKey = ""
		response.UpdaterFileName = ""
		response.UpdaterObjectKey = ""
		response.UpdaterSize = 0
		response.UpdaterSHA256 = ""
		response.UpdaterSHA512 = ""
		response.Status = 0
		response.Published = false
	}
	return response
}

func CompareClientVersions(left string, right string) int {
	leftVersion := parseClientVersion(left)
	rightVersion := parseClientVersion(right)
	for i := 0; i < 3; i++ {
		if leftVersion.parts[i] > rightVersion.parts[i] {
			return 1
		}
		if leftVersion.parts[i] < rightVersion.parts[i] {
			return -1
		}
	}
	if leftVersion.preRelease == rightVersion.preRelease {
		return 0
	}
	if leftVersion.preRelease == "" {
		return 1
	}
	if rightVersion.preRelease == "" {
		return -1
	}
	if leftVersion.preRelease > rightVersion.preRelease {
		return 1
	}
	return -1
}

type parsedClientVersion struct {
	parts      [3]int
	preRelease string
}

func parseClientVersion(value string) parsedClientVersion {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "v")
	value = strings.TrimPrefix(value, "V")
	if idx := strings.Index(value, "+"); idx >= 0 {
		value = value[:idx]
	}
	preRelease := ""
	if idx := strings.Index(value, "-"); idx >= 0 {
		preRelease = value[idx+1:]
		value = value[:idx]
	}
	var result parsedClientVersion
	result.preRelease = preRelease
	segments := strings.Split(value, ".")
	for i := 0; i < len(segments) && i < 3; i++ {
		part := numericPrefix(segments[i])
		if part == "" {
			continue
		}
		num, err := strconv.Atoi(part)
		if err == nil {
			result.parts[i] = num
		}
	}
	return result
}

func numericPrefix(value string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	for _, r := range value {
		if r < '0' || r > '9' {
			break
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

func clientReleaseContainsLikePattern(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if len([]rune(value)) > clientReleaseKeywordMaxRunes {
		return "", errors.New("keyword is too long")
	}
	value = strings.ReplaceAll(value, "!", "!!")
	value = strings.ReplaceAll(value, "%", "!%")
	value = strings.ReplaceAll(value, "_", "!_")
	return "%" + value + "%", nil
}

func cleanClientReleaseFileName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\\", "/")
	parts := strings.Split(value, "/")
	if len(parts) > 0 {
		value = parts[len(parts)-1]
	}
	value = strings.Trim(value, ". ")
	if len(value) > 255 {
		return value[len(value)-255:]
	}
	return value
}

func ClientReleaseExpectedFileName(version string, platform string, arch string, channel string, fileName string) string {
	ext := strings.ToLower(path.Ext(strings.ReplaceAll(fileName, "\\", "/")))
	if ext == ".appimage" {
		ext = ".AppImage"
	}
	return fmt.Sprintf(
		"Z-UP-Setup-%s-%s-%s-%s%s",
		NormalizeClientReleaseVersion(version),
		NormalizeClientReleasePlatform(platform),
		NormalizeClientReleaseArch(arch),
		NormalizeClientReleaseChannel(channel),
		ext,
	)
}

func IsAllowedClientReleaseInstallerFile(platform string, fileName string) bool {
	ext := strings.ToLower(path.Ext(strings.ReplaceAll(fileName, "\\", "/")))
	switch NormalizeClientReleasePlatform(platform) {
	case "windows":
		return ext == ".exe" || ext == ".msi" || ext == ".zip"
	case "macos":
		return ext == ".dmg"
	case "linux":
		return ext == ".appimage" || ext == ".deb" || ext == ".rpm" || ext == ".zip"
	default:
		return false
	}
}

func IsAllowedClientReleaseUploadFile(platform string, fileName string) bool {
	if IsAllowedClientReleaseInstallerFile(platform, fileName) {
		return true
	}
	return NormalizeClientReleasePlatform(platform) == "macos" && strings.EqualFold(path.Ext(fileName), ".zip")
}

func ClientReleaseTarget(platform string, arch string, channel string) string {
	return fmt.Sprintf(
		"%s/%s/%s",
		NormalizeClientReleasePlatform(platform),
		NormalizeClientReleaseArch(arch),
		NormalizeClientReleaseChannel(channel),
	)
}

func migrateClientReleasePlatformToMacOS(db *gorm.DB) error {
	if !db.Migrator().HasTable(&ClientRelease{}) {
		return nil
	}

	var releases []ClientRelease
	if err := db.Unscoped().Where("platform IN ?", []string{"darwin", "macos"}).Find(&releases).Error; err != nil {
		return err
	}
	targets := make(map[string]int, len(releases))
	for _, release := range releases {
		deletedAt := "active"
		if release.DeletedAt.Valid {
			deletedAt = release.DeletedAt.Time.UTC().Format(time.RFC3339Nano)
		}
		key := fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s", release.Version, release.Arch, release.Channel, deletedAt, NormalizeClientReleasePlatform(release.Platform))
		if previousID, exists := targets[key]; exists {
			return fmt.Errorf("cannot migrate client release platform to macos: releases %d and %d have the same target", previousID, release.Id)
		}
		targets[key] = release.Id
	}

	return db.Unscoped().Model(&ClientRelease{}).Where("platform = ?", "darwin").UpdateColumn("platform", "macos").Error
}
