package echo_tool

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestShouldPrintArgsWhenRunNormally(t *testing.T) {
	var buf bytes.Buffer
	err := Run(context.Background(), []string{"-input", "/tmp/data", "-label", "test"}, &buf)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "label: test") {
		t.Fatalf("expected output to contain label, got: %s", output)
	}
	if !strings.Contains(output, "input: /tmp/data") {
		t.Fatalf("expected output to contain input, got: %s", output)
	}
}

func TestShouldReturnErrorWhenNoArgs(t *testing.T) {
	var buf bytes.Buffer
	err := Run(context.Background(), nil, &buf)
	if err != nil {
		t.Fatalf("Run with no args should not return error: %v", err)
	}
	if !strings.Contains(buf.String(), "echo_tool finished") {
		t.Fatalf("expected echo_tool to finish normally, got: %s", buf.String())
	}
}
