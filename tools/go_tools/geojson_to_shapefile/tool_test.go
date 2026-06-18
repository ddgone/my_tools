package geojson_to_shapefile

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRunConvertOnlyWritesPointShapefile(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := filepath.Join(inputDir, "output")

	geojson := `{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": {"name": "p1"},
      "geometry": {"type": "Point", "coordinates": [114.0, 30.0]}
    },
    {
      "type": "Feature",
      "properties": {"name": "line1"},
      "geometry": {"type": "LineString", "coordinates": [[114.0, 30.0], [114.1, 30.1]]}
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(inputDir, "sample.geojson"), []byte(geojson), 0644); err != nil {
		t.Fatalf("failed to write geojson: %v", err)
	}

	var out bytes.Buffer
	if err := runConvert(context.Background(), inputDir, outputDir, &out); err != nil {
		t.Fatalf("expected runConvert success, got error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(outputDir, "merged_points.shp")); err != nil {
		t.Fatalf("expected point shapefile to exist, got error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "merged_lines.shp")); !os.IsNotExist(err) {
		t.Fatalf("expected line shapefile to be absent, got error: %v", err)
	}
}
