package service

import "testing"

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

func TestClientReleaseUploadTicketRoundTrip(t *testing.T) {
	cfg := clientReleaseOSSConfig{AccessKeySecret: "secret"}
	ticket := clientReleaseUploadTicket{
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
	if got.FileName != ticket.FileName || got.Object != ticket.Object || got.Size != ticket.Size || got.ContentType != ticket.ContentType || got.ExpiresAt != ticket.ExpiresAt {
		t.Fatalf("parsed ticket = %+v, want %+v", got, ticket)
	}
	if _, err := parseClientReleaseUploadTicket(value+"x", cfg); err == nil {
		t.Fatal("parseClientReleaseUploadTicket() returned nil error for tampered ticket")
	}
}
