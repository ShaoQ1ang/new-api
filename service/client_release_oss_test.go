package service

import (
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestClientReleaseGeneratedFileName(t *testing.T) {
	input := ClientReleaseUploadInput{
		Version:  "1.2.3",
		Platform: "windows",
		Arch:     "x64",
		Channel:  "stable",
	}
	got := clientReleaseGeneratedFileName(input, `C:\Users\demo\random setup.exe`)
	want := "Z-UP-Setup-1.2.3-windows-x64-stable.exe"
	if got != want {
		t.Fatalf("clientReleaseGeneratedFileName() = %q, want %q", got, want)
	}
}

func TestClientReleaseGeneratedFileNameKeepsAppImageExtension(t *testing.T) {
	input := ClientReleaseUploadInput{
		Version:  "1.2.4",
		Platform: "linux",
		Arch:     "arm64",
		Channel:  "beta",
	}
	got := clientReleaseGeneratedFileName(input, "z-up.AppImage")
	want := "Z-UP-Setup-1.2.4-linux-arm64-beta.AppImage"
	if got != want {
		t.Fatalf("clientReleaseGeneratedFileName() = %q, want %q", got, want)
	}
}

func TestClientReleaseGeneratedFileNameUsesMacOS(t *testing.T) {
	input := ClientReleaseUploadInput{
		Version:  "1.2.5",
		Platform: "darwin",
		Arch:     "arm64",
		Channel:  "stable",
	}
	require.NoError(t, normalizeClientReleaseUploadInput(&input))
	require.Equal(t, "macos", input.Platform)
	require.Equal(t,
		"Z-UP-Setup-1.2.5-macos-arm64-stable.zip",
		clientReleaseGeneratedFileName(input, "z-up.zip"),
	)
}

func TestNormalizeClientReleaseUploadInputRejectsInvalidTarget(t *testing.T) {
	input := ClientReleaseUploadInput{
		Version:  "1.2.3",
		Platform: "android",
		Arch:     "x64",
		Channel:  "stable",
	}
	if err := normalizeClientReleaseUploadInput(&input); err == nil {
		t.Fatal("normalizeClientReleaseUploadInput() returned nil error for invalid platform")
	}

	input = ClientReleaseUploadInput{
		Version:  "1.2.3",
		Platform: "windows",
		Arch:     "mips",
		Channel:  "stable",
	}
	if err := normalizeClientReleaseUploadInput(&input); err == nil {
		t.Fatal("normalizeClientReleaseUploadInput() returned nil error for invalid arch")
	}

	input = ClientReleaseUploadInput{
		Version:  "1.2.3",
		Platform: "windows",
		Arch:     "x64",
	}
	require.ErrorContains(t, normalizeClientReleaseUploadInput(&input), "channel is required")
}

func TestClientReleaseUploadURLExpiresDefault(t *testing.T) {
	t.Setenv("CLIENT_RELEASE_OSS_UPLOAD_URL_EXPIRES_SECONDS", "")
	if got := clientReleaseUploadURLExpires(); got != defaultClientReleaseUploadURLExpiresSeconds {
		t.Fatalf("clientReleaseUploadURLExpires() = %d, want %d", got, defaultClientReleaseUploadURLExpiresSeconds)
	}
}

func TestClientReleaseUploadURLExpiresCapsAtOneDay(t *testing.T) {
	t.Setenv("CLIENT_RELEASE_OSS_UPLOAD_URL_EXPIRES_SECONDS", "999999")
	if got := clientReleaseUploadURLExpires(); got != 86400 {
		t.Fatalf("clientReleaseUploadURLExpires() = %d, want 86400", got)
	}
}

func TestClientReleaseDirectUploadUsesHTTPSForBareEndpoint(t *testing.T) {
	t.Setenv("CLIENT_RELEASE_OSS_ENDPOINT", "oss-cn-hangzhou.aliyuncs.com")
	t.Setenv("CLIENT_RELEASE_OSS_BUCKET", "private")
	t.Setenv("CLIENT_RELEASE_OSS_ACCESS_KEY_ID", "ak")
	t.Setenv("CLIENT_RELEASE_OSS_ACCESS_KEY_SECRET", "secret")
	t.Setenv("CLIENT_RELEASE_OSS_PREFIX", "client-releases")

	result, err := InitClientReleaseDirectUpload(ClientReleaseDirectUploadInput{
		ActorUserID: 1,
		Version:     "1.2.3",
		Platform:    "windows",
		Arch:        "x64",
		Channel:     "stable",
		FileName:    "setup.exe",
		Size:        1,
	})
	if err != nil {
		t.Fatalf("InitClientReleaseDirectUpload() error = %v", err)
	}
	if !strings.HasPrefix(result.UploadURL, "https://") {
		t.Fatalf("UploadURL = %q, want https URL", result.UploadURL)
	}
}

func TestClientReleaseUploadTicketRoundTrip(t *testing.T) {
	cfg := clientReleaseOSSConfig{AccessKeySecret: "secret"}
	ticket := clientReleaseUploadTicket{
		ActorUserID: 7,
		FileName:    "Z-UP-Setup-1.2.3-windows-x64-stable.exe",
		Object:      "client-releases/stable/windows/x64/1.2.3/file.exe",
		Size:        123,
		ContentType: "application/vnd.microsoft.portable-executable",
		ExpiresAt:   4102444800,
	}
	value, err := signClientReleaseUploadTicket(ticket, cfg)
	if err != nil {
		t.Fatalf("signClientReleaseUploadTicket() error = %v", err)
	}
	got, err := parseClientReleaseUploadTicket(value, cfg)
	if err != nil {
		t.Fatalf("parseClientReleaseUploadTicket() error = %v", err)
	}
	if got.ActorUserID != ticket.ActorUserID || got.FileName != ticket.FileName || got.Object != ticket.Object || got.Size != ticket.Size || got.ContentType != ticket.ContentType || got.ExpiresAt != ticket.ExpiresAt {
		t.Fatalf("parsed ticket = %+v, want %+v", got, ticket)
	}
	if _, err := parseClientReleaseUploadTicket(value+"x", cfg); err == nil {
		t.Fatal("parseClientReleaseUploadTicket() returned nil error for tampered ticket")
	}
}

func TestClientReleaseUploadTicketIsBoundToActor(t *testing.T) {
	t.Setenv("CLIENT_RELEASE_OSS_ENDPOINT", "https://oss-cn-hangzhou.aliyuncs.com")
	t.Setenv("CLIENT_RELEASE_OSS_BUCKET", "private")
	t.Setenv("CLIENT_RELEASE_OSS_ACCESS_KEY_ID", "ak")
	t.Setenv("CLIENT_RELEASE_OSS_ACCESS_KEY_SECRET", "secret")
	t.Setenv("CLIENT_RELEASE_OSS_PREFIX", "client-releases")

	ticket, err := signClientReleaseUploadTicket(clientReleaseUploadTicket{
		ActorUserID: 7,
		FileName:    "Z-UP-Setup-1.2.3-windows-x64-stable.exe",
		Object:      "client-releases/_tmp/random/Z-UP-Setup-1.2.3-windows-x64-stable.exe",
		Size:        123,
		ContentType: "application/vnd.microsoft.portable-executable",
		ExpiresAt:   time.Now().Add(time.Hour).Unix(),
	}, clientReleaseOSSConfig{AccessKeySecret: "secret"})
	require.NoError(t, err)

	_, err = CompleteClientReleaseDirectUpload(ticket, 8)
	require.ErrorContains(t, err, "does not belong to the current user")
	err = DiscardClientReleaseDirectUpload(ticket, 8)
	require.ErrorContains(t, err, "does not belong to the current user")
}

func TestClientReleaseUploadFileTypesArePlatformSpecific(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		fileName string
		wantErr  bool
	}{
		{name: "windows exe", platform: "windows", fileName: "setup.exe"},
		{name: "windows dmg", platform: "windows", fileName: "setup.dmg", wantErr: true},
		{name: "macos dmg", platform: "macos", fileName: "setup.dmg"},
		{name: "macos updater zip", platform: "macos", fileName: "setup.zip"},
		{name: "linux AppImage", platform: "linux", fileName: "setup.AppImage"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			allowed := model.IsAllowedClientReleaseUploadFile(test.platform, test.fileName)
			require.Equal(t, !test.wantErr, allowed)
		})
	}
}

func TestClientReleaseTempObjectKey(t *testing.T) {
	cfg := clientReleaseOSSConfig{Prefix: "client-releases"}
	key, err := cfg.tempObjectKey("setup.exe")
	if err != nil {
		t.Fatalf("tempObjectKey() error = %v", err)
	}
	if !strings.HasPrefix(key, "client-releases/_tmp/") || !strings.HasSuffix(key, "/setup.exe") {
		t.Fatalf("temp object key = %q", key)
	}
	if !cfg.isTempObjectKey(key) {
		t.Fatalf("isTempObjectKey(%q) = false, want true", key)
	}
	final := cfg.objectKey(ClientReleaseUploadInput{
		Version:  "1.2.3",
		Platform: "windows",
		Arch:     "x64",
		Channel:  "stable",
	}, "setup.exe")
	if cfg.isTempObjectKey(final) {
		t.Fatalf("final object key %q must not be temporary", final)
	}
}

func TestPromoteClientReleaseObjectSkipsFinalManagedObjectWithoutOSSConfig(t *testing.T) {
	release := &model.ClientRelease{
		Version:   "1.2.3",
		Platform:  "windows",
		Arch:      "x64",
		Channel:   "stable",
		FileName:  "setup.exe",
		ObjectKey: "/client-releases/stable/windows/x64/1.2.3/setup.exe",
	}
	result, err := PromoteClientReleaseObject(release)
	if err != nil {
		t.Fatalf("PromoteClientReleaseObject() error = %v", err)
	}
	if result.Promoted {
		t.Fatal("PromoteClientReleaseObject() promoted a final object")
	}
	if release.ObjectKey != "client-releases/stable/windows/x64/1.2.3/setup.exe" {
		t.Fatalf("release.ObjectKey = %q", release.ObjectKey)
	}
}

func TestPromoteClientReleaseObjectRejectsUnmanagedObject(t *testing.T) {
	release := &model.ClientRelease{ObjectKey: "other-prefix/setup.exe"}
	if _, err := PromoteClientReleaseObject(release); err == nil {
		t.Fatal("PromoteClientReleaseObject() returned nil error for unmanaged object")
	}
}

func TestPromoteClientReleaseObjectRejectsTemporaryObjectWithDifferentFileName(t *testing.T) {
	t.Setenv("CLIENT_RELEASE_OSS_ENDPOINT", "https://oss-cn-hangzhou.aliyuncs.com")
	t.Setenv("CLIENT_RELEASE_OSS_BUCKET", "private")
	t.Setenv("CLIENT_RELEASE_OSS_ACCESS_KEY_ID", "ak")
	t.Setenv("CLIENT_RELEASE_OSS_ACCESS_KEY_SECRET", "secret")
	t.Setenv("CLIENT_RELEASE_OSS_PREFIX", "client-releases")

	release := &model.ClientRelease{
		Version:   "1.2.4",
		Platform:  "macos",
		Arch:      "arm64",
		Channel:   "stable",
		FileName:  "Z-UP-Setup-1.2.4-macos-arm64-stable.dmg",
		ObjectKey: "client-releases/_tmp/random/Z-UP-Setup-1.2.3-macos-arm64-stable.dmg",
	}
	_, err := PromoteClientReleaseObject(release)
	require.ErrorContains(t, err, "temporary object does not match uploaded file name")
}

func TestPromoteClientReleaseObjectRejectsUploadForDifferentTarget(t *testing.T) {
	t.Setenv("CLIENT_RELEASE_OSS_ENDPOINT", "https://oss-cn-hangzhou.aliyuncs.com")
	t.Setenv("CLIENT_RELEASE_OSS_BUCKET", "private")
	t.Setenv("CLIENT_RELEASE_OSS_ACCESS_KEY_ID", "ak")
	t.Setenv("CLIENT_RELEASE_OSS_ACCESS_KEY_SECRET", "secret")
	t.Setenv("CLIENT_RELEASE_OSS_PREFIX", "client-releases")

	release := &model.ClientRelease{
		Version:   "1.2.4",
		Platform:  "macos",
		Arch:      "arm64",
		Channel:   "stable",
		FileName:  "Z-UP-Setup-1.2.3-macos-arm64-stable.dmg",
		ObjectKey: "client-releases/_tmp/random/Z-UP-Setup-1.2.3-macos-arm64-stable.dmg",
	}
	_, err := PromoteClientReleaseObject(release)
	require.ErrorContains(t, err, "upload target does not match release metadata")
}
