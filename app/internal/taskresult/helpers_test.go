package taskresult

import "testing"

func TestFinalizeResultDownloadPath(t *testing.T) {
	if got := finalizeResultDownloadPath("C:/temp/result", "directory"); got != "C:/temp/result.tar.gz" {
		t.Fatalf("unexpected directory download path: %s", got)
	}
	if got := finalizeResultDownloadPath("C:/temp/result.json", "file"); got != "C:/temp/result.json" {
		t.Fatalf("unexpected file download path: %s", got)
	}
}
