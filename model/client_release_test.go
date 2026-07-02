package model

import "testing"

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
