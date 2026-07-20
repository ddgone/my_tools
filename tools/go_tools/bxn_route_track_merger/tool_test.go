package bxn_route_track_merger

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeRouteJSONFile(t *testing.T, dir, name string, tracks ...Track) {
	t.Helper()

	type inputFile struct {
		TrackList []Track `json:"trackList"`
	}
	data, err := json.MarshalIndent(inputFile{TrackList: tracks}, "", "  ")
	if err != nil {
		t.Fatalf("序列化测试数据失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
		t.Fatalf("写入测试输入文件失败: %v", err)
	}
}

func writeRouteJSONFormatB(t *testing.T, dir, name, trackID string, timestamps ...int64) {
	t.Helper()

	points := make([]Point, len(timestamps))
	for i, ts := range timestamps {
		points[i] = Point{
			X:         114.0 + float64(i)*0.1,
			Y:         30.0 + float64(i)*0.1,
			Z:         5.0 + float64(i),
			Timestamp: ts,
			Azimuth:   0.1,
			Pitch:     0.2,
			Roll:      0.3,
		}
	}

	type singleTrack struct {
		TrackID   string  `json:"trackId"`
		PointList []Point `json:"pointList"`
	}
	data, err := json.MarshalIndent(singleTrack{TrackID: trackID, PointList: points}, "", "  ")
	if err != nil {
		t.Fatalf("序列化测试数据失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
		t.Fatalf("写入测试输入文件失败: %v", err)
	}
}

func makeTrack(trackID string, timestamps ...int64) Track {
	points := make([]Point, len(timestamps))
	for i, ts := range timestamps {
		points[i] = Point{
			X:         114.0 + float64(i)*0.1,
			Y:         30.0 + float64(i)*0.1,
			Z:         5.0 + float64(i),
			Timestamp: ts,
			Azimuth:   0.1,
			Pitch:     0.2,
			Roll:      0.3,
		}
	}
	return Track{TrackID: trackID, PointList: points}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("预期文件 %s 存在，但报错: %v", path, err)
	}
}

func assertFileNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("预期文件 %s 不存在，但它存在", path)
	}
}

func TestParseFileFormatA(t *testing.T) {
	dir := t.TempDir()
	writeRouteJSONFile(t, dir, "route_001.json",
		makeTrack("track_1", 100, 200),
		makeTrack("track_2", 300, 400),
	)

	tracks, err := parseFile(filepath.Join(dir, "route_001.json"))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(tracks) != 2 {
		t.Fatalf("预期 2 条轨迹，得到 %d", len(tracks))
	}
	if tracks[0].TrackID != "track_1" {
		t.Fatalf("预期 trackId track_1，得到 %s", tracks[0].TrackID)
	}
	if tracks[1].TrackID != "track_2" {
		t.Fatalf("预期 trackId track_2，得到 %s", tracks[1].TrackID)
	}
}

func TestParseFileFormatB(t *testing.T) {
	dir := t.TempDir()
	writeRouteJSONFormatB(t, dir, "route_001.json", "single_track", 100, 200, 300)

	tracks, err := parseFile(filepath.Join(dir, "route_001.json"))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("预期 1 条轨迹，得到 %d", len(tracks))
	}
	if tracks[0].TrackID != "single_track" {
		t.Fatalf("预期 trackId single_track，得到 %s", tracks[0].TrackID)
	}
	if len(tracks[0].PointList) != 3 {
		t.Fatalf("预期 3 个点，得到 %d", len(tracks[0].PointList))
	}
}

func TestRunConvertAllArtifactsWithMerge(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := filepath.Join(inputDir, "output")

	writeRouteJSONFile(t, inputDir, "route_001.json",
		makeTrack("track_a", 100, 200, 300),
	)
	writeRouteJSONFile(t, inputDir, "route_002.json",
		makeTrack("track_b", 400, 500),
	)

	var out bytes.Buffer
	err := runConvert(context.Background(), inputDir, outputDir, artifactAll, 4, true, &out)
	if err != nil {
		t.Fatalf("runConvert 失败: %v", err)
	}

	// json/
	assertFileExists(t, filepath.Join(outputDir, "json", "track_a.json"))
	assertFileExists(t, filepath.Join(outputDir, "json", "track_b.json"))

	// shp/
	assertFileExists(t, filepath.Join(outputDir, "shp", "track_a.shp"))
	assertFileExists(t, filepath.Join(outputDir, "shp", "track_a.prj"))
	assertFileExists(t, filepath.Join(outputDir, "shp", "track_b.shp"))
	assertFileExists(t, filepath.Join(outputDir, "shp", "track_b.prj"))

	// geojson/
	assertFileExists(t, filepath.Join(outputDir, "geojson", "track_a.geojson"))
	assertFileExists(t, filepath.Join(outputDir, "geojson", "track_b.geojson"))

	// 合并产物
	assertFileExists(t, filepath.Join(outputDir, "geojson_merge", "merged.geojson"))
	assertFileExists(t, filepath.Join(outputDir, "shp_merge", "merged.shp"))
	assertFileExists(t, filepath.Join(outputDir, "shp_merge", "merged.prj"))
}

func TestRunConvertGeoJSONOnly(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := filepath.Join(inputDir, "output")

	writeRouteJSONFile(t, inputDir, "route_001.json",
		makeTrack("track_a", 100, 200),
	)

	var out bytes.Buffer
	err := runConvert(context.Background(), inputDir, outputDir, artifactGeoJSON, 4, false, &out)
	if err != nil {
		t.Fatalf("runConvert 失败: %v", err)
	}

	assertFileExists(t, filepath.Join(outputDir, "json", "track_a.json"))
	assertFileExists(t, filepath.Join(outputDir, "geojson", "track_a.geojson"))
	assertFileNotExists(t, filepath.Join(outputDir, "shp"))
}

func TestRunConvertSHPOnly(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := filepath.Join(inputDir, "output")

	writeRouteJSONFile(t, inputDir, "route_001.json",
		makeTrack("track_a", 100, 200),
	)

	var out bytes.Buffer
	err := runConvert(context.Background(), inputDir, outputDir, artifactSHP, 4, false, &out)
	if err != nil {
		t.Fatalf("runConvert 失败: %v", err)
	}

	assertFileExists(t, filepath.Join(outputDir, "json", "track_a.json"))
	assertFileExists(t, filepath.Join(outputDir, "shp", "track_a.shp"))
	assertFileExists(t, filepath.Join(outputDir, "shp", "track_a.prj"))
	assertFileNotExists(t, filepath.Join(outputDir, "geojson"))
}

func TestRunConvertGeoJSONContainsLineString(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := filepath.Join(inputDir, "output")

	writeRouteJSONFile(t, inputDir, "route_001.json",
		makeTrack("track_a", 100, 200, 300),
	)

	var out bytes.Buffer
	err := runConvert(context.Background(), inputDir, outputDir, artifactGeoJSON, 4, false, &out)
	if err != nil {
		t.Fatalf("runConvert 失败: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(outputDir, "geojson", "track_a.geojson"))
	if err != nil {
		t.Fatalf("读取 GeoJSON 失败: %v", err)
	}

	var fc geoJSONCollection
	if err := json.Unmarshal(data, &fc); err != nil {
		t.Fatalf("解析 GeoJSON 失败: %v", err)
	}

	if len(fc.Features) != 1 {
		t.Fatalf("预期 1 个 Feature，得到 %d", len(fc.Features))
	}

	f := fc.Features[0]
	if f.Type != "Feature" {
		t.Fatalf("预期 type=Feature，得到 %s", f.Type)
	}
	if f.Geometry.Type != "LineString" {
		t.Fatalf("预期 geometry type=LineString，得到 %s", f.Geometry.Type)
	}
	if len(f.Geometry.Coordinates) != 3 {
		t.Fatalf("预期 3 个坐标，得到 %d", len(f.Geometry.Coordinates))
	}
	if f.Properties["trackId"] != "track_a" {
		t.Fatalf("预期 trackId=track_a，得到 %v", f.Properties["trackId"])
	}

	for i, coord := range f.Geometry.Coordinates {
		if len(coord) != 3 {
			t.Fatalf("坐标点 %d 长度应为 3，得到 %d", i, len(coord))
		}
	}
}

func TestValidateArtifactSet(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{artifactGeoJSON, false},
		{artifactSHP, false},
		{artifactAll, false},
		{"invalid", true},
		{"", true},
	}
	for _, tt := range tests {
		err := validateArtifactSet(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateArtifactSet(%q) error=%v, wantErr=%v", tt.input, err, tt.wantErr)
		}
	}
}

func TestRunConvertNoMergeSingleTrack(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := filepath.Join(inputDir, "output")

	writeRouteJSONFile(t, inputDir, "route_001.json",
		makeTrack("track_a", 100, 200),
	)

	var out bytes.Buffer
	err := runConvert(context.Background(), inputDir, outputDir, artifactAll, 4, true, &out)
	if err != nil {
		t.Fatalf("runConvert 失败: %v", err)
	}

	// 只有 1 条轨迹时不生成 merge
	assertFileNotExists(t, filepath.Join(outputDir, "geojson_merge"))
	assertFileNotExists(t, filepath.Join(outputDir, "shp_merge"))

	// json 仍然生成
	assertFileExists(t, filepath.Join(outputDir, "json", "track_a.json"))
}

func TestRunConvertTrackIdMerge(t *testing.T) {
	// 同一 trackId 出现在两个文件中时，点应合并
	inputDir := t.TempDir()
	outputDir := filepath.Join(inputDir, "output")

	writeRouteJSONFile(t, inputDir, "route_001.json",
		makeTrack("track_a", 100, 200),
	)
	writeRouteJSONFile(t, inputDir, "route_002.json",
		makeTrack("track_a", 300, 400),
	)

	var out bytes.Buffer
	err := runConvert(context.Background(), inputDir, outputDir, artifactAll, 4, true, &out)
	if err != nil {
		t.Fatalf("runConvert 失败: %v", err)
	}

	// 应只有一条 track_a
	data, err := os.ReadFile(filepath.Join(outputDir, "json", "track_a.json"))
	if err != nil {
		t.Fatalf("读取 JSON 失败: %v", err)
	}

	var track Track
	if err := json.Unmarshal(data, &track); err != nil {
		t.Fatalf("解析 JSON 失败: %v", err)
	}
	if len(track.PointList) != 4 {
		t.Fatalf("预期 4 个点（合并），得到 %d", len(track.PointList))
	}

	// 排序后应按 timestamp 升序
	if track.PointList[0].Timestamp != 100 {
		t.Fatalf("预期第一个点 Timestamp=100，得到 %d", track.PointList[0].Timestamp)
	}
	if track.PointList[3].Timestamp != 400 {
		t.Fatalf("预期最后一个点 Timestamp=400，得到 %d", track.PointList[3].Timestamp)
	}
}
