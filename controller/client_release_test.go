package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestValidateClientReleaseAssetChangesRejectsArbitraryManagedObject(t *testing.T) {
	release := &model.ClientRelease{
		Version:   "1.2.3",
		Platform:  "windows",
		Arch:      "x64",
		Channel:   "stable",
		ObjectKey: "client-releases/stable/windows/x64/1.2.3/attacker-controlled.exe",
	}

	err := validateClientReleaseAssetChanges(nil, release, &clientReleasePromotions{
		installer: &service.ClientReleasePromoteResult{},
	})
	require.ErrorContains(t, err, "completed direct upload")
}

func TestValidateClientReleaseAssetChangesRequiresFreshAssetsAfterTargetChange(t *testing.T) {
	existing := &model.ClientRelease{
		Version:          "1.2.3",
		Platform:         "macos",
		Arch:             "arm64",
		Channel:          "stable",
		ObjectKey:        "client-releases/stable/macos/arm64/1.2.3/installer.dmg",
		UpdaterObjectKey: "client-releases/stable/macos/arm64/1.2.3/updater.zip",
	}
	release := *existing
	release.Version = "1.2.4"

	err := validateClientReleaseAssetChanges(existing, &release, &clientReleasePromotions{
		installer: &service.ClientReleasePromoteResult{},
		updater:   &service.ClientReleasePromoteResult{},
	})
	require.ErrorContains(t, err, "installer must come from a completed direct upload")
}

func TestValidateClientReleaseAssetChangesRejectsMetadataRewriteWithoutUpload(t *testing.T) {
	existing := &model.ClientRelease{
		Version:   "1.2.3",
		Platform:  "windows",
		Arch:      "x64",
		Channel:   "stable",
		FileName:  "Z-UP-Setup-1.2.3-windows-x64-stable.exe",
		ObjectKey: "client-releases/stable/windows/x64/1.2.3/installer.exe",
		Size:      100,
		SHA256:    "sha256:original",
		SHA512:    "original",
	}
	release := *existing
	release.SHA512 = "attacker-controlled"

	err := validateClientReleaseAssetChanges(existing, &release, &clientReleasePromotions{
		installer: &service.ClientReleasePromoteResult{},
	})
	require.ErrorContains(t, err, "metadata cannot change without a completed direct upload")
}

func TestValidateClientReleaseAssetChangesAcceptsPromotedMacOSAssets(t *testing.T) {
	release := &model.ClientRelease{
		Version:          "1.2.3",
		Platform:         "macos",
		Arch:             "arm64",
		Channel:          "stable",
		ObjectKey:        "client-releases/stable/macos/arm64/1.2.3/installer.dmg",
		UpdaterObjectKey: "client-releases/stable/macos/arm64/1.2.3/updater.zip",
	}

	err := validateClientReleaseAssetChanges(nil, release, &clientReleasePromotions{
		installer: &service.ClientReleasePromoteResult{Promoted: true},
		updater:   &service.ClientReleasePromoteResult{Promoted: true},
	})
	require.NoError(t, err)
}

func TestGetClientReleaseLatestMacYAMLUsesUpdaterZIP(t *testing.T) {
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ClientRelease{}))
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })

	release := &model.ClientRelease{
		Version:          "1.2.3",
		Platform:         "macos",
		Arch:             "arm64",
		Channel:          "stable",
		FileName:         "Z-UP-Setup-1.2.3-macos-arm64-stable.dmg",
		ObjectKey:        "client-releases/stable/macos/arm64/1.2.3/installer.dmg",
		Size:             120,
		SHA512:           "installer-sha512",
		UpdaterFileName:  "Z-UP-Setup-1.2.3-macos-arm64-stable.zip",
		UpdaterObjectKey: "client-releases/stable/macos/arm64/1.2.3/updater.zip",
		UpdaterSize:      100,
		UpdaterSHA512:    "updater-sha512",
		Status:           model.ClientReleaseStatusPublished,
	}
	require.NoError(t, db.Create(release).Error)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{
		{Key: "platform", Value: "macos"},
		{Key: "arch", Value: "arm64"},
		{Key: "channel", Value: "stable"},
	}
	GetClientReleaseLatestMacYAML(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "no-cache, no-store, must-revalidate", recorder.Header().Get("Cache-Control"))
	require.Contains(t, recorder.Body.String(), "Z-UP-Setup-1.2.3-macos-arm64-stable.zip")
	require.Contains(t, recorder.Body.String(), "sha512: updater-sha512")
	require.NotContains(t, recorder.Body.String(), ".dmg")

	invalidDownloadRecorder := httptest.NewRecorder()
	invalidDownloadContext, _ := gin.CreateTestContext(invalidDownloadRecorder)
	invalidDownloadContext.Params = gin.Params{
		{Key: "platform", Value: "macos"},
		{Key: "arch", Value: "arm64"},
		{Key: "channel", Value: "stable"},
		{Key: "id", Value: "1"},
		{Key: "filename", Value: "another-object.zip"},
	}
	DownloadClientReleaseAsset(invalidDownloadContext)
	require.Empty(t, invalidDownloadRecorder.Header().Get("Location"))
	require.Contains(t, invalidDownloadRecorder.Body.String(), "client release asset not found")
}

func TestGetClientReleaseLatestMacYAMLRejectsOtherPlatforms(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "platform", Value: "windows"}}

	GetClientReleaseLatestMacYAML(context)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "only available for macos")
}

func TestRequiredAdminClientReleasePlatform(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		want      string
		wantError string
	}{
		{name: "required", wantError: "platform is required"},
		{name: "invalid", query: "?platform=android", wantError: "windows, macos, or linux"},
		{name: "Darwin alias normalizes to macos", query: "?platform=darwin", want: "macos"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodGet, "/api/admin/client-releases/"+test.query, nil)
			got, err := requiredAdminClientReleasePlatform(context)
			if test.wantError != "" {
				require.ErrorContains(t, err, test.wantError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestClientReleaseURLsExposeOnlyPublishedPublicLinks(t *testing.T) {
	previousServerAddress := system_setting.ServerAddress
	system_setting.ServerAddress = "https://downloads.example.com/"
	t.Cleanup(func() { system_setting.ServerAddress = previousServerAddress })

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "http://internal.invalid", nil)
	release := &model.ClientRelease{
		Id:               42,
		Platform:         "macos",
		Arch:             "arm64",
		Channel:          "stable",
		UpdaterFileName:  "Z-UP-Setup-1.2.3-macos-arm64-stable.zip",
		UpdaterObjectKey: "client-releases/updater.zip",
		Status:           model.ClientReleaseStatusDraft,
	}

	require.Equal(t, model.ClientReleaseURLs{}, clientReleaseURLs(context, release))
	release.Status = model.ClientReleaseStatusPublished
	urls := clientReleaseURLs(context, release)
	require.Equal(t, "https://downloads.example.com/api/client-releases/download/42", urls.DownloadURL)
	require.Equal(t, "https://downloads.example.com/api/client-releases/updates/macos/arm64/stable/download/42/Z-UP-Setup-1.2.3-macos-arm64-stable.zip", urls.UpdaterDownloadURL)
	require.Equal(t, "https://downloads.example.com/api/client-releases/updates/macos/arm64/stable/latest-mac.yml", urls.UpdateManifestURL)
}

func TestRequestBaseURLIgnoresForwardedHost(t *testing.T) {
	previousServerAddress := system_setting.ServerAddress
	system_setting.ServerAddress = "invalid://configured-address"
	t.Cleanup(func() { system_setting.ServerAddress = previousServerAddress })

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "http://admin.example.com/path", nil)
	context.Request.Host = "admin.example.com"
	context.Request.Header.Set("X-Forwarded-Proto", "https")
	context.Request.Header.Set("X-Forwarded-Host", "attacker.example")

	require.Equal(t, "https://admin.example.com", requestBaseURL(context))

	context.Request.Host = "admin.example.com@attacker.example"
	require.Empty(t, requestBaseURL(context))
}
