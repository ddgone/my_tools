package main

import (
	"testing"
)

func TestEnsureToolingLoadsPythonTools(t *testing.T) {
	app := NewApp()
	if err := ensureTooling(app.state); err != nil {
		t.Fatalf("ensureTooling failed: %v", err)
	}

	if len(app.state.PyTools) < 2 {
		t.Fatalf("expected at least 2 Python tools, got %d", len(app.state.PyTools))
	}

	if _, ok := app.state.Manifests["geojson_to_shp"]; !ok {
		t.Fatalf("expected geojson_to_shp manifest to be available")
	}
}
