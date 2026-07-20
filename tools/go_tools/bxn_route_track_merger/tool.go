package bxn_route_track_merger

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/jonas-p/go-shp"
)

// Point 单轨迹点
type Point struct {
	X               float64 `json:"x"`
	Y               float64 `json:"y"`
	Z               float64 `json:"z"`
	Timestamp       int64   `json:"timestamp"`
	Azimuth         float64 `json:"azimuth"`
	Pitch           float64 `json:"pitch"`
	Roll            float64 `json:"roll"`
	VideoFile       string  `json:"videoFile"`
	VideoFrameIndex string  `json:"videoFrameIndex"`
	SourceFile      string  `json:"sourceFile"`
}

// Track 单条轨迹
type Track struct {
	TrackID   string  `json:"trackId"`
	PointList []Point `json:"pointList"`
}

// inputFile 支持两种JSON格式：
// Format A: {"trackList": [{"trackId": "...", "pointList": [...]}]}
// Format B: {"trackId": "...", "pointList": [...]}
type inputFile struct {
	TrackList []Track `json:"trackList"`
	TrackID   string  `json:"trackId"`
	PointList []Point `json:"pointList"`
}

// GeoJSON types
type geoJSONFeature struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Geometry   geoJSONLineGeometry    `json:"geometry"`
}

type geoJSONLineGeometry struct {
	Type        string      `json:"type"`
	Coordinates [][]float64 `json:"coordinates"`
}

type geoJSONCollection struct {
	Type     string           `json:"type"`
	Features []geoJSONFeature `json:"features"`
}

var pointFields = []shp.Field{
	shp.StringField("trackId", 50),
	shp.FloatField("longitude", 16, 10),
	shp.FloatField("latitude", 16, 10),
	shp.FloatField("altitude", 12, 4),
	shp.StringField("timestamp", 20),
	shp.FloatField("azimuth", 10, 4),
	shp.FloatField("pitch", 10, 4),
	shp.FloatField("roll", 10, 4),
	shp.StringField("videoFile", 100),
	shp.StringField("videoIdx", 20),
	shp.StringField("sourceFile", 80),
}

const (
	artifactGeoJSON = "geojson"
	artifactSHP     = "shp"
	artifactAll     = "all"
)

func shouldWriteGeoJSON(artifactSet string) bool {
	return artifactSet == artifactGeoJSON || artifactSet == artifactAll
}

func shouldWriteSHP(artifactSet string) bool {
	return artifactSet == artifactSHP || artifactSet == artifactAll
}

func validateArtifactSet(s string) error {
	switch s {
	case artifactGeoJSON, artifactSHP, artifactAll:
		return nil
	default:
		return fmt.Errorf("-artifact-set 仅支持 geojson、shp 或 all")
	}
}

// parseFile 解析单个 route JSON 文件
func parseFile(path string) ([]Track, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var file inputFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}

	sourceName := filepath.Base(path)

	var tracks []Track

	// Format A: trackList
	for _, t := range file.TrackList {
		if t.TrackID == "" || len(t.PointList) == 0 {
			continue
		}
		for i := range t.PointList {
			t.PointList[i].SourceFile = sourceName
		}
		tracks = append(tracks, t)
	}

	// Format B: root-level trackId
	if file.TrackID != "" && len(file.PointList) > 0 {
		for i := range file.PointList {
			file.PointList[i].SourceFile = sourceName
		}
		tracks = append(tracks, Track{TrackID: file.TrackID, PointList: file.PointList})
	}

	return tracks, nil
}

// trackToGeoJSONFeature 将轨迹转为 LineString GeoJSON Feature
func trackToGeoJSONFeature(track Track) geoJSONFeature {
	coords := make([][]float64, len(track.PointList))
	sourceSet := make(map[string]struct{}, 1)
	for i, p := range track.PointList {
		coords[i] = []float64{p.X, p.Y, p.Z}
		sourceSet[p.SourceFile] = struct{}{}
	}
	sources := make([]string, 0, len(sourceSet))
	for s := range sourceSet {
		sources = append(sources, s)
	}
	sort.Strings(sources)
	return geoJSONFeature{
		Type: "Feature",
		Properties: map[string]interface{}{
			"trackId":     track.TrackID,
			"pointCount":  len(track.PointList),
			"sourceFiles": sources,
		},
		Geometry: geoJSONLineGeometry{
			Type:        "LineString",
			Coordinates: coords,
		},
	}
}

func writeSingleGeoJSON(path string, track Track) error {
	fc := geoJSONCollection{
		Type:     "FeatureCollection",
		Features: []geoJSONFeature{trackToGeoJSONFeature(track)},
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(fc)
}

func writeMergedGeoJSON(path string, tracks []Track) error {
	features := make([]geoJSONFeature, 0, len(tracks))
	for _, t := range tracks {
		features = append(features, trackToGeoJSONFeature(t))
	}

	fc := geoJSONCollection{Type: "FeatureCollection", Features: features}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(fc)
}

func writeTrackJSON(path string, track Track) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(track)
}

func writeSingleSHP(base string, track Track) error {
	shape, err := shp.Create(base, shp.POINT)
	if err != nil {
		return err
	}

	if err := shape.SetFields(pointFields); err != nil {
		shape.Close()
		return err
	}

	for i, p := range track.PointList {
		pt := shp.Point{X: p.X, Y: p.Y}
		shape.Write(&pt)

		idx := i
		_ = shape.WriteAttribute(idx, 0, track.TrackID)
		_ = shape.WriteAttribute(idx, 1, p.X)
		_ = shape.WriteAttribute(idx, 2, p.Y)
		_ = shape.WriteAttribute(idx, 3, p.Z)
		_ = shape.WriteAttribute(idx, 4, fmt.Sprintf("%d", p.Timestamp))
		_ = shape.WriteAttribute(idx, 5, p.Azimuth)
		_ = shape.WriteAttribute(idx, 6, p.Pitch)
		_ = shape.WriteAttribute(idx, 7, p.Roll)
		_ = shape.WriteAttribute(idx, 8, p.VideoFile)
		_ = shape.WriteAttribute(idx, 9, p.VideoFrameIndex)
		_ = shape.WriteAttribute(idx, 10, p.SourceFile)
	}
	shape.Close()

	fixDbfExt(base)
	return writeWGS84Prj(base + ".prj")
}

func writeMergedSHP(base string, tracks []Track) error {
	shape, err := shp.Create(base, shp.POINT)
	if err != nil {
		return err
	}

	if err := shape.SetFields(pointFields); err != nil {
		shape.Close()
		return err
	}

	globalIdx := 0
	for _, t := range tracks {
		for _, p := range t.PointList {
			pt := shp.Point{X: p.X, Y: p.Y}
			shape.Write(&pt)

			_ = shape.WriteAttribute(globalIdx, 0, t.TrackID)
			_ = shape.WriteAttribute(globalIdx, 1, p.X)
			_ = shape.WriteAttribute(globalIdx, 2, p.Y)
			_ = shape.WriteAttribute(globalIdx, 3, p.Z)
			_ = shape.WriteAttribute(globalIdx, 4, fmt.Sprintf("%d", p.Timestamp))
			_ = shape.WriteAttribute(globalIdx, 5, p.Azimuth)
			_ = shape.WriteAttribute(globalIdx, 6, p.Pitch)
			_ = shape.WriteAttribute(globalIdx, 7, p.Roll)
			_ = shape.WriteAttribute(globalIdx, 8, p.VideoFile)
			_ = shape.WriteAttribute(globalIdx, 9, p.VideoFrameIndex)
			_ = shape.WriteAttribute(globalIdx, 10, p.SourceFile)
			globalIdx++
		}
	}
	shape.Close()

	fixDbfExt(base)
	return writeWGS84Prj(base + ".prj")
}

func fixDbfExt(base string) {
	bad := base + "dbf"
	good := base + ".dbf"
	if _, err := os.Stat(bad); err == nil {
		os.Rename(bad, good)
	}
}

func writeWGS84Prj(prjPath string) error {
	wgs84WKT := `GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9122"]],AUTHORITY["EPSG","4326"]]`
	return os.WriteFile(prjPath, []byte(wgs84WKT), 0644)
}

type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

func runConvert(ctx context.Context, inputDir, outputDir, artifactSet string, workers int, merge bool, out io.Writer) error {
	if workers <= 0 {
		return fmt.Errorf("-workers 必须大于 0")
	}

	info, err := os.Stat(inputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("目录 '%s' 不存在", inputDir)
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("'%s' 不是一个目录", inputDir)
	}

	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return err
	}

	var jsonFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".json") {
			jsonFiles = append(jsonFiles, filepath.Join(inputDir, e.Name()))
		}
	}

	if len(jsonFiles) == 0 {
		return fmt.Errorf("未在 '%s' 中找到任何 JSON 文件", inputDir)
	}

	sw := &syncWriter{w: out}

	// 阶段1：并发解析所有文件，按 trackId 归并
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		trackMap = make(map[string]*Track)
		firstErr error
		sem      = make(chan struct{}, workers)
	)

	for _, jp := range jsonFiles {
		select {
		case <-ctx.Done():
			return fmt.Errorf("任务已被取消")
		default:
		}

		wg.Add(1)
		go func(jp string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			tracks, err := parseFile(jp)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("解析 %s: %w", filepath.Base(jp), err)
				}
				mu.Unlock()
				return
			}

			mu.Lock()
			for _, t := range tracks {
				if existing, ok := trackMap[t.TrackID]; ok {
					existing.PointList = append(existing.PointList, t.PointList...)
				} else {
					trackMap[t.TrackID] = &Track{TrackID: t.TrackID, PointList: t.PointList}
				}
			}
			fmt.Fprintf(sw, "加载: %s (%d 条轨迹)\n", filepath.Base(jp), len(tracks))
			mu.Unlock()
		}(jp)
	}
	wg.Wait()

	if firstErr != nil {
		return firstErr
	}

	if len(trackMap) == 0 {
		return fmt.Errorf("%d 个 JSON 文件中未找到有效轨迹数据", len(jsonFiles))
	}

	fmt.Fprintf(out, "\n解析完成！%d 个文件，%d 条轨迹\n", len(jsonFiles), len(trackMap))

	// 排序 trackId，按时间戳排序点
	trackIDs := make([]string, 0, len(trackMap))
	for tid := range trackMap {
		trackIDs = append(trackIDs, tid)
	}
	sort.Strings(trackIDs)

	var totalPoints int
	for _, tid := range trackIDs {
		t := trackMap[tid]
		sort.Slice(t.PointList, func(i, j int) bool {
			return t.PointList[i].Timestamp < t.PointList[j].Timestamp
		})
		totalPoints += len(t.PointList)
	}

	fmt.Fprintf(out, "总点数: %d\n", totalPoints)

	// 阶段2：按 trackId 输出到子目录
	jsonDir := filepath.Join(outputDir, "json")
	os.MkdirAll(jsonDir, 0755)

	var shpDir, geoDir string
	if shouldWriteSHP(artifactSet) {
		shpDir = filepath.Join(outputDir, "shp")
		os.MkdirAll(shpDir, 0755)
	}
	if shouldWriteGeoJSON(artifactSet) {
		geoDir = filepath.Join(outputDir, "geojson")
		os.MkdirAll(geoDir, 0755)
	}

	for _, tid := range trackIDs {
		select {
		case <-ctx.Done():
			return fmt.Errorf("任务已被取消")
		default:
		}

		track := trackMap[tid]

		// JSON（始终输出）
		jsonPath := filepath.Join(jsonDir, tid+".json")
		if err := writeTrackJSON(jsonPath, *track); err != nil {
			return fmt.Errorf("写入 JSON %s: %w", tid, err)
		}

		// GeoJSON
		if shouldWriteGeoJSON(artifactSet) {
			geoPath := filepath.Join(geoDir, tid+".geojson")
			if err := writeSingleGeoJSON(geoPath, *track); err != nil {
				return fmt.Errorf("写入 GeoJSON %s: %w", tid, err)
			}
		}

		// SHP
		if shouldWriteSHP(artifactSet) && len(track.PointList) > 0 {
			shpBase := filepath.Join(shpDir, tid)
			if err := writeSingleSHP(shpBase, *track); err != nil {
				return fmt.Errorf("写入 SHP %s: %w", tid, err)
			}
		}
	}

	fmt.Fprintf(out, "  [json]    %d 条 -> %s\n", len(trackIDs), jsonDir)
	if shouldWriteGeoJSON(artifactSet) {
		fmt.Fprintf(out, "  [geojson] %d 条 -> %s\n", len(trackIDs), geoDir)
	}
	if shouldWriteSHP(artifactSet) {
		fmt.Fprintf(out, "  [shp]     %d 条 -> %s\n", len(trackIDs), shpDir)
	}

	// 阶段3：合并产物
	if merge && len(trackIDs) > 1 {
		select {
		case <-ctx.Done():
			return fmt.Errorf("任务已被取消")
		default:
		}

		allTracks := make([]Track, 0, len(trackIDs))
		for _, tid := range trackIDs {
			allTracks = append(allTracks, *trackMap[tid])
		}

		if shouldWriteGeoJSON(artifactSet) {
			mergeGeoDir := filepath.Join(outputDir, "geojson_merge")
			os.MkdirAll(mergeGeoDir, 0755)
			geoPath := filepath.Join(mergeGeoDir, "merged.geojson")
			if err := writeMergedGeoJSON(geoPath, allTracks); err != nil {
				fmt.Fprintf(out, "  警告: 合并 GeoJSON 写入失败: %v\n", err)
			} else {
				fmt.Fprintf(out, "  [geojson_merge] %d 条 -> %s\n", len(allTracks), mergeGeoDir)
			}
		}

		if shouldWriteSHP(artifactSet) {
			mergeShpDir := filepath.Join(outputDir, "shp_merge")
			os.MkdirAll(mergeShpDir, 0755)
			shpBase := filepath.Join(mergeShpDir, "merged")
			if err := writeMergedSHP(shpBase, allTracks); err != nil {
				fmt.Fprintf(out, "  警告: 合并 SHP 写入失败: %v\n", err)
			} else {
				fmt.Fprintf(out, "  [shp_merge]     %d 条 -> %s\n", len(allTracks), mergeShpDir)
			}
		}
	}

	fmt.Fprintf(out, "\n输出目录: %s\n", outputDir)
	return nil
}

func Run(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("bxn-route-track-merger", flag.ContinueOnError)
	fs.SetOutput(out)

	var inputDir, outputDir, artifactSet string
	var workers int
	var merge bool

	fs.StringVar(&inputDir, "input", "", "包含 route JSON 文件的输入目录")
	fs.StringVar(&outputDir, "output", "", "输出目录（默认在输入目录下创建 output）")
	fs.StringVar(&artifactSet, "artifact-set", artifactAll, "输出产物: geojson | shp | all")
	fs.IntVar(&workers, "workers", 4, "并发处理线程数")
	fs.BoolVar(&merge, "merge", true, "输出合并轨迹产物")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if inputDir == "" {
		return fmt.Errorf("必须指定 -input 参数")
	}

	if err := validateArtifactSet(artifactSet); err != nil {
		return err
	}

	if outputDir == "" {
		outputDir = filepath.Join(inputDir, "output")
	}

	return runConvert(ctx, inputDir, outputDir, artifactSet, workers, merge, out)
}
