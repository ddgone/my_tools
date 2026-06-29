package runtime

import (
	"strings"
	"testing"
)

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

func TestFormatProbeDetail(t *testing.T) {
	detail := formatProbeDetail("uname -s && uname -m", "Linux\nx86_64", "warning", nil)
	if !strings.Contains(detail, `stdout: "Linux\nx86_64"`) {
		t.Fatalf("expected stdout in detail, got: %s", detail)
	}
	if !strings.Contains(detail, `stderr: "warning"`) {
		t.Fatalf("expected stderr in detail, got: %s", detail)
	}

	errDetail := formatProbeDetail("uname -s && uname -m", "", "", errTestProbe)
	if !strings.Contains(errDetail, errTestProbe.Error()) {
		t.Fatalf("expected error message in detail, got: %s", errDetail)
	}
}

var errTestProbe = testProbeError("probe failed")

type testProbeError string

func (e testProbeError) Error() string {
	return string(e)
}

func TestParseRemotePathEntries(t *testing.T) {
	entries := parseRemotePathEntries(strings.Join([]string{
		"beta.txt\t/tmp/beta.txt\tfile\tfalse",
		"z-dir\t/tmp/z-dir\tdirectory\tfalse",
		"alpha-dir\t/tmp/alpha-dir\tdirectory\ttrue",
		"",
	}, "\n"))

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Kind != "directory" || entries[0].Name != "alpha-dir" {
		t.Fatalf("expected first entry to be alpha-dir directory, got %+v", entries[0])
	}
	if !entries[0].IsSymlink {
		t.Fatalf("expected symlink flag to be preserved, got %+v", entries[0])
	}
	if entries[2].Kind != "file" || entries[2].Name != "beta.txt" {
		t.Fatalf("expected file entry last, got %+v", entries[2])
	}
}

func TestNormalizeRemotePath(t *testing.T) {
	if got := normalizeRemotePath(" /tmp/demo/../file.txt \n"); got != "/tmp/file.txt" {
		t.Fatalf("unexpected normalized path: %q", got)
	}
	if got := normalizeRemotePath("."); got != "" {
		t.Fatalf("expected dot to collapse to empty, got %q", got)
	}
}
