package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCompareClientVersions(t *testing.T) {
	cases := []struct {
		name  string
		left  string
		right string
		want  int
	}{
		{name: "patch newer", left: "0.1.2", right: "0.1.1", want: 1},
		{name: "minor newer", left: "0.10.0", right: "0.2.0", want: 1},
		{name: "equal with v prefix", left: "v1.2.3", right: "1.2.3", want: 0},
		{name: "stable beats prerelease", left: "1.2.3", right: "1.2.3-beta.1", want: 1},
		{name: "older", left: "1.2.2", right: "1.2.3", want: -1},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareClientVersions(tt.left, tt.right)
			if got != tt.want {
				t.Fatalf("CompareClientVersions(%q, %q) = %d, want %d", tt.left, tt.right, got, tt.want)
			}
		})
	}
}

func TestClientReleaseTargetNormalization(t *testing.T) {
	got := ClientReleaseTarget("windows", "amd64", "")
	want := "windows/x64/stable"
	if got != want {
		t.Fatalf("ClientReleaseTarget() = %q, want %q", got, want)
	}
	if got := NormalizeClientReleasePlatform("win32"); got != "windows" {
		t.Fatalf("NormalizeClientReleasePlatform() = %q, want %q", got, "windows")
	}
	for _, alias := range []string{"darwin", "mac", "macos", "osx"} {
		require.Equal(t, "macos", NormalizeClientReleasePlatform(alias))
	}
	if !IsAllowedClientReleasePlatform("win32") {
		t.Fatal("IsAllowedClientReleasePlatform(\"win32\") = false, want true")
	}
	if !IsAllowedClientReleaseArch("amd64") {
		t.Fatal("IsAllowedClientReleaseArch(\"amd64\") = false, want true")
	}
	if IsAllowedClientReleasePlatform("android") {
		t.Fatal("IsAllowedClientReleasePlatform(\"android\") = true, want false")
	}
	if IsAllowedClientReleaseArch("mips") {
		t.Fatal("IsAllowedClientReleaseArch(\"mips\") = true, want false")
	}
}

func TestValidateMacOSClientReleaseRequiresDMGAndUpdaterZIPBeforePublishing(t *testing.T) {
	release := &ClientRelease{
		Version:   "1.2.3",
		Platform:  "macos",
		Arch:      "arm64",
		Channel:   "stable",
		FileName:  "Z-UP-Setup-1.2.3-macos-arm64-stable.dmg",
		ObjectKey: "client-releases/stable/macos/arm64/1.2.3/client.dmg",
		Size:      1024,
		SHA512:    "installer-sha512",
		Status:    ClientReleaseStatusPublished,
	}

	require.ErrorContains(t, ValidateClientRelease(release), "updater zip is required")

	release.UpdaterFileName = "Z-UP-Setup-1.2.3-macos-arm64-stable.zip"
	release.UpdaterObjectKey = "client-releases/stable/macos/arm64/1.2.3/client.zip"
	release.UpdaterSize = 900
	release.UpdaterSHA512 = "updater-sha512"
	require.NoError(t, ValidateClientRelease(release))

	release.UpdaterFileName = "Z-UP-Setup-1.2.3-macos-arm64-stable.dmg"
	require.ErrorContains(t, ValidateClientRelease(release), "must be a zip file")
}

func TestClientReleaseVersionValidation(t *testing.T) {
	if got := NormalizeClientReleaseVersion(" 1.2.3 "); got != "1.2.3" {
		t.Fatalf("NormalizeClientReleaseVersion() = %q, want %q", got, "1.2.3")
	}
	valid := []string{"1.2.3", "0.1.0", "10.20.30"}
	for _, version := range valid {
		if err := ValidateClientReleaseVersion(version); err != nil {
			t.Fatalf("ValidateClientReleaseVersion(%q) returned error: %v", version, err)
		}
	}
	invalid := []string{"1", "1.2", "1.2.x", "latest", "v1.2.3", "1.2.3-beta.1", "1.2.3+build.7"}
	for _, version := range invalid {
		if err := ValidateClientReleaseVersion(version); err == nil {
			t.Fatalf("ValidateClientReleaseVersion(%q) returned nil error", version)
		}
	}
}

func TestClientReleaseChannelValidation(t *testing.T) {
	if got := NormalizeClientReleaseChannel(" BETA "); got != "beta" {
		t.Fatalf("NormalizeClientReleaseChannel() = %q, want %q", got, "beta")
	}
	for _, channel := range []string{"stable", "beta", ""} {
		if !IsAllowedClientReleaseChannel(channel) {
			t.Fatalf("IsAllowedClientReleaseChannel(%q) = false, want true", channel)
		}
	}
	if IsAllowedClientReleaseChannel("dev") {
		t.Fatal("IsAllowedClientReleaseChannel(\"dev\") = true, want false")
	}
}

func TestValidateClientReleaseRequiresSHA512BeforePublishing(t *testing.T) {
	release := &ClientRelease{
		Version:   "1.2.3",
		Platform:  "windows",
		Arch:      "x64",
		Channel:   "stable",
		FileName:  "Z-UP-Setup-1.2.3-windows-x64-stable.exe",
		ObjectKey: "client-releases/stable/windows/x64/1.2.3/test.exe",
		Size:      1024,
		Status:    ClientReleaseStatusPublished,
	}

	if err := ValidateClientRelease(release); err == nil {
		t.Fatal("ValidateClientRelease() returned nil error for published release without sha512")
	}

	release.SHA512 = "base64-sha512"
	if err := ValidateClientRelease(release); err != nil {
		t.Fatalf("ValidateClientRelease() returned error with sha512: %v", err)
	}
}

func TestValidateClientReleaseEnforcesTargetFileNameAndPlatformType(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		fileName string
		wantErr  string
	}{
		{name: "windows exe", platform: "windows", fileName: "Z-UP-Setup-1.2.3-windows-x64-stable.exe"},
		{name: "windows rejects dmg", platform: "windows", fileName: "Z-UP-Setup-1.2.3-windows-x64-stable.dmg", wantErr: "not supported for windows"},
		{name: "macos dmg", platform: "macos", fileName: "Z-UP-Setup-1.2.3-macos-x64-stable.dmg"},
		{name: "linux AppImage", platform: "linux", fileName: "Z-UP-Setup-1.2.3-linux-x64-stable.AppImage"},
		{name: "target mismatch", platform: "linux", fileName: "Z-UP-Setup-1.2.3-windows-x64-stable.AppImage", wantErr: "does not match target"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			release := &ClientRelease{
				Version:   "1.2.3",
				Platform:  test.platform,
				Arch:      "x64",
				Channel:   "stable",
				FileName:  test.fileName,
				ObjectKey: "client-releases/test",
				Size:      1,
				Status:    ClientReleaseStatusDraft,
			}
			err := ValidateClientRelease(release)
			if test.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

func TestMigrateClientReleasePlatformToMacOS(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&ClientRelease{}))
	require.NoError(t, db.Exec(`
		INSERT INTO client_releases
		(version, platform, arch, channel, file_name, object_key, size, status, created_time, updated_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "1.2.3", "darwin", "arm64", "stable", "legacy.dmg", "client-releases/legacy.dmg", 100, 0, 1, 1).Error)

	require.NoError(t, migrateClientReleasePlatformToMacOS(db))
	var platform string
	require.NoError(t, db.Model(&ClientRelease{}).Select("platform").Where("version = ?", "1.2.3").Scan(&platform).Error)
	require.Equal(t, "macos", platform)
}

func TestMigrateClientReleasePlatformToMacOSRejectsConflictingTargets(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&ClientRelease{}))
	for _, platform := range []string{"darwin", "macos"} {
		require.NoError(t, db.Exec(`
			INSERT INTO client_releases
			(version, platform, arch, channel, file_name, object_key, size, status, created_time, updated_time)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, "1.2.3", platform, "arm64", "stable", platform+".dmg", "client-releases/"+platform+".dmg", 100, 0, 1, 1).Error)
	}

	err = migrateClientReleasePlatformToMacOS(db)
	require.ErrorContains(t, err, "same target")
}
