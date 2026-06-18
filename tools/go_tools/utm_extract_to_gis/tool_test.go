package utm_extract_to_gis

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteTrackArtifactsOnlyWritesPointSHP(t *testing.T) {
	outputDir := t.TempDir()
	features := []GeoJSONFeature{
		{
			Type: "Feature",
			Properties: map[string]interface{}{
				"track_id":    "track_a",
				"point_index": 0,
				"timestamp":   "2026-06-18T10:00:00Z",
			},
			Geometry: GeoJSONGeometry{
				Type:        "Point",
				Coordinates: []float64{114.0, 30.0},
			},
		},
		{
			Type: "Feature",
			Properties: map[string]interface{}{
				"track_id":    "track_a",
				"point_index": 1,
				"timestamp":   "2026-06-18T10:00:01Z",
			},
			Geometry: GeoJSONGeometry{
				Type:        "Point",
				Coordinates: []float64{114.1, 30.1},
			},
		},
	}

	var out bytes.Buffer
	if err := writeTrackArtifacts(context.Background(), outputDir, "track_a", features, artifactSHP, &out); err != nil {
		t.Fatalf("expected writeTrackArtifacts success, got error: %v", err)
	}

	pointShp := filepath.Join(outputDir, "track_a_point.shp")
	if _, err := os.Stat(pointShp); err != nil {
		t.Fatalf("expected point shapefile to exist, got error: %v", err)
	}

	lineShp := filepath.Join(outputDir, "track_a_line.shp")
	if _, err := os.Stat(lineShp); !os.IsNotExist(err) {
		t.Fatalf("expected line shapefile to be absent, got error: %v", err)
	}
}
