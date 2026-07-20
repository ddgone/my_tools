package pos_trajectory_to_gis

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func writePOSInput(t *testing.T, dir, name, trackID string, timestamps ...int) {
	t.Helper()

	content := `{
  "trackId": "` + trackID + `",
  "pointList": [
    {"x": 114.0, "y": 30.0, "z": 5.0, "timestamp": ` + intToString(timestamps[0]) + `, "azimuth": 0.1, "pitch": 0.2, "roll": 0.3, "videoFile": "a.mp4", "videoFrameIndex": "1"}`
	for i := 1; i < len(timestamps); i++ {
		content += `,
    {"x": 114.1, "y": 30.1, "z": 5.1, "timestamp": ` + intToString(timestamps[i]) + `, "azimuth": 0.2, "pitch": 0.3, "roll": 0.4, "videoFile": "a.mp4", "videoFrameIndex": "2"}`
	}
	content += `
  ]
}`

	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write input json: %v", err)
	}
}

func intToString(v int) string {
	return strconv.Itoa(v)
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist, got error: %v", path, err)
	}
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be absent, got error: %v", path, err)
	}
}

func TestRunConvertWritesPerTrackAndMergedArtifacts(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := filepath.Join(inputDir, "output")

	writePOSInput(t, inputDir, "track_a.json", "track_1", 1, 2)
	writePOSInput(t, inputDir, "track_b.json", "track_2", 3, 4)

	var out bytes.Buffer
	if err := runConvert(context.Background(), inputDir, outputDir, artifactAll, 4, &out); err != nil {
		t.Fatalf("expected runConvert success, got error: %v", err)
	}

	assertExists(t, filepath.Join(outputDir, "track_a.geojson"))
	assertExists(t, filepath.Join(outputDir, "track_a_point.shp"))
	assertExists(t, filepath.Join(outputDir, "track_b.geojson"))
	assertExists(t, filepath.Join(outputDir, "track_b_point.shp"))
	assertExists(t, filepath.Join(outputDir, "merged_pos.geojson"))
	assertExists(t, filepath.Join(outputDir, "merged_pos_point.shp"))
	assertNotExists(t, filepath.Join(outputDir, "merged_pos_line.shp"))
}

func TestRunConvertRespectsArtifactSetSHP(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := filepath.Join(inputDir, "output")
	writePOSInput(t, inputDir, "track.json", "track_1", 1, 2)

	var out bytes.Buffer
	if err := runConvert(context.Background(), inputDir, outputDir, artifactSHP, 4, &out); err != nil {
		t.Fatalf("expected runConvert success, got error: %v", err)
	}

	assertExists(t, filepath.Join(outputDir, "track_point.shp"))
	assertNotExists(t, filepath.Join(outputDir, "track.geojson"))
	assertNotExists(t, filepath.Join(outputDir, "merged_pos_point.shp"))
}

func TestRunConvertRespectsArtifactSetGeoJSON(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := filepath.Join(inputDir, "output")
	writePOSInput(t, inputDir, "track.json", "track_1", 1, 2)

	var out bytes.Buffer
	if err := runConvert(context.Background(), inputDir, outputDir, artifactGeoJSON, 4, &out); err != nil {
		t.Fatalf("expected runConvert success, got error: %v", err)
	}

	assertExists(t, filepath.Join(outputDir, "track.geojson"))
	assertNotExists(t, filepath.Join(outputDir, "track_point.shp"))
	assertNotExists(t, filepath.Join(outputDir, "merged_pos.geojson"))
}
