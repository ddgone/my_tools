package main

import (
	"reflect"
	"testing"
)

func TestNormalizeRustCLIArgs(t *testing.T) {
	input := []string{"-input", "/tmp/source.laz", "-output", "/tmp/out.laz", "-raster-only", "-h", "--threads", "4"}
	got := normalizeRustCLIArgs(input)
	want := []string{"--input", "/tmp/source.laz", "--output", "/tmp/out.laz", "--raster-only", "-h", "--threads", "4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected normalized args:\nwant: %#v\ngot:  %#v", want, got)
	}
}
