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

func TestResolveRemoteResultHintWithDefaultOutputForFileInput(t *testing.T) {
	hint, err := resolveRemoteResultHint([]toolspec.ParameterSpec{
		{
			Key:      "input",
			ArgKey:   "input",
			Type:     toolspec.FieldTypePath,
			PathMode: "file",
		},
		{
			Key:         "output",
			ArgKey:      "output",
			Type:        toolspec.FieldTypePath,
			PathMode:    "directory",
			Placeholder: "留空则自动在输入位置生成 output",
		},
	}, `-input /data/source/report.geojson`, "/tmp/fire-salamander-123")
	if err != nil {
		t.Fatalf("resolveRemoteResultHint failed: %v", err)
	}
	if hint.Path != "/data/source/output" {
		t.Fatalf("unexpected path: %s", hint.Path)
	}
	if hint.Kind != "directory" {
		t.Fatalf("unexpected kind: %s", hint.Kind)
	}
}

func TestResolveRemoteResultHintWithDefaultOutputForDirectoryInput(t *testing.T) {
	hint, err := resolveRemoteResultHint([]toolspec.ParameterSpec{
		{
			Key:      "input",
			ArgKey:   "input",
			Type:     toolspec.FieldTypePath,
			PathMode: "directory",
		},
		{
			Key:      "output",
			ArgKey:   "output",
			Type:     toolspec.FieldTypePath,
			PathMode: "directory",
			Help:     "不填写时会沿用旧版默认行为，在输入目录下创建 output 目录。",
		},
	}, `-input /data/source-dir`, "/tmp/fire-salamander-123")
	if err != nil {
		t.Fatalf("resolveRemoteResultHint failed: %v", err)
	}
	if hint.Path != "/data/source-dir/output" {
		t.Fatalf("unexpected path: %s", hint.Path)
	}
}

func TestResolveRemoteResultHintWithDefaultOutputForFileOrDirectoryInput(t *testing.T) {
	hint, err := resolveRemoteResultHint([]toolspec.ParameterSpec{
		{
			Key:      "convertPath",
			ArgKey:   "convert",
			Type:     toolspec.FieldTypePath,
			PathMode: "fileOrDirectory",
		},
		{
			Key:      "output",
			ArgKey:   "output",
			Type:     toolspec.FieldTypePath,
			PathMode: "directory",
			Help:     "可选。默认在输入路径旁边创建 output 目录。",
		},
	}, `-convert /data/tracks/utm.txt`, "/tmp/fire-salamander-123")
	if err != nil {
		t.Fatalf("resolveRemoteResultHint failed: %v", err)
	}
	if hint.Path != "/data/tracks/output" {
		t.Fatalf("unexpected path: %s", hint.Path)
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
