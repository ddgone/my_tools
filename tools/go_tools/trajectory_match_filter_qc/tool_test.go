package trajectory_match_filter_qc

import (
	"bytes"
	"testing"
	"time"
)

func TestParseCLIArgsShouldSupportMergeFlag(t *testing.T) {
	var out bytes.Buffer
	opts, err := parseCLIArgs([]string{
		"-input", "target.shp",
		"-base", "base.shp",
		"-merge=false",
		"-timeout", "2m",
	}, &out)
	if err != nil {
		t.Fatalf("expected parse success, got error: %v", err)
	}

	if opts.Merge {
		t.Fatalf("expected merge to be false")
	}
	if opts.Timeout != 2*time.Minute {
		t.Fatalf("expected timeout 2m, got %v", opts.Timeout)
	}
}

func TestParseCLIArgsShouldSupportLegacyMeargeFlag(t *testing.T) {
	var out bytes.Buffer
	opts, err := parseCLIArgs([]string{
		"-input", "target.shp",
		"-base", "base.shp",
		"-mearge=false",
	}, &out)
	if err != nil {
		t.Fatalf("expected parse success, got error: %v", err)
	}

	if opts.Merge {
		t.Fatalf("expected legacy mearge flag to disable merge output")
	}
}
