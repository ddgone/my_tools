package runtime

import "testing"

func TestParseRemotePlatformOutput(t *testing.T) {
	t.Run("linux amd64", func(t *testing.T) {
		platform, err := parseRemotePlatformOutput("Linux\nx86_64\n")
		if err != nil {
			t.Fatalf("expected parse success, got error: %v", err)
		}
		if platform.OS != "linux" || platform.Arch != "amd64" {
			t.Fatalf("unexpected platform: %+v", platform)
		}
	})

	t.Run("ignores empty lines", func(t *testing.T) {
		platform, err := parseRemotePlatformOutput("\nLinux\n\nx86_64\n")
		if err != nil {
			t.Fatalf("expected parse success, got error: %v", err)
		}
		if platform.OS != "linux" || platform.Arch != "amd64" {
			t.Fatalf("unexpected platform: %+v", platform)
		}
	})

	t.Run("fails on empty output", func(t *testing.T) {
		if _, err := parseRemotePlatformOutput(""); err == nil {
			t.Fatal("expected parse error for empty output")
		}
	})

	t.Run("fails on unsupported platform", func(t *testing.T) {
		if _, err := parseRemotePlatformOutput("Windows\nAMD64\n"); err == nil {
			t.Fatal("expected parse error for unsupported platform")
		}
	})
}
