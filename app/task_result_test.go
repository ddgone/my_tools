package main

import (
	"testing"

	"my_tools/libs/core/toolspec"
)

func TestResolveRemoteResultHintForOutputDirectory(t *testing.T) {
	hint, err := resolveRemoteResultHint([]toolspec.ParameterSpec{
		{
			Key:    "input",
			ArgKey: "input",
			Type:   toolspec.FieldTypePath,
		},
		{
			Key:    "output",
			ArgKey: "output",
			Type:   toolspec.FieldTypePath,
		},
	}, `-input /data/source -output output`, "/tmp/fire-salamander-123")
	if err != nil {
		t.Fatalf("resolveRemoteResultHint failed: %v", err)
	}
	if hint.Path != "/tmp/fire-salamander-123/output" {
		t.Fatalf("unexpected path: %s", hint.Path)
	}
	if hint.Kind != "directory" {
		t.Fatalf("unexpected kind: %s", hint.Kind)
	}
}

func TestResolveRemoteResultHintForOutputFile(t *testing.T) {
	hint, err := resolveRemoteResultHint([]toolspec.ParameterSpec{
		{
			Key:      "output",
			ArgKey:   "output",
			Type:     toolspec.FieldTypePath,
			PathMode: "file",
		},
	}, `-output report.json`, "/tmp/fire-salamander-123")
	if err != nil {
		t.Fatalf("resolveRemoteResultHint failed: %v", err)
	}
	if hint.Path != "/tmp/fire-salamander-123/report.json" {
		t.Fatalf("unexpected path: %s", hint.Path)
	}
	if hint.Kind != "file" {
		t.Fatalf("unexpected kind: %s", hint.Kind)
	}
}

func TestResolveRemoteResultHintWithoutOutputValue(t *testing.T) {
	hint, err := resolveRemoteResultHint([]toolspec.ParameterSpec{
		{
			Key:    "output",
			ArgKey: "output",
			Type:   toolspec.FieldTypePath,
		},
	}, `-input /data/source`, "/tmp/fire-salamander-123")
	if err != nil {
		t.Fatalf("resolveRemoteResultHint failed: %v", err)
	}
	if hint.Path != "" {
		t.Fatalf("expected empty path, got %s", hint.Path)
	}
}

func TestPathWithinRemoteBase(t *testing.T) {
	if !pathWithinRemoteBase("/tmp/fire-salamander-123", "/tmp/fire-salamander-123/output/result.json") {
		t.Fatal("expected result path to be within remote base")
	}
	if pathWithinRemoteBase("/tmp/fire-salamander-123", "/tmp/fire-salamander-1234/output") {
		t.Fatal("unexpected prefix-only match")
	}
}

func TestFinalizeResultDownloadPath(t *testing.T) {
	if got := finalizeResultDownloadPath("C:/temp/result", "directory"); got != "C:/temp/result.tar.gz" {
		t.Fatalf("unexpected directory download path: %s", got)
	}
	if got := finalizeResultDownloadPath("C:/temp/result.json", "file"); got != "C:/temp/result.json" {
		t.Fatalf("unexpected file download path: %s", got)
	}
}
