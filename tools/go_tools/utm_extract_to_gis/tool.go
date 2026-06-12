package utm_extract_to_gis

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"my_tools/libs/framework"

	"github.com/im7mortal/UTM"
	"github.com/jonas-p/go-shp"
)

type GeoJSONFeature struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Geometry   GeoJSONGeometry        `json:"geometry"`
}

type GeoJSONGeometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

type FeatureCollection struct {
	Type     string           `json:"type"`
	Features []GeoJSONFeature `json:"features"`
}

func utmToLatLon(easting, northing float64, zoneNumber int) (lat, lon float64) {
	lat, lon, err := UTM.ToLatLon(easting, northing, zoneNumber, "", true)
	if err != nil {
		return math.NaN(), math.NaN()
	}
	return lat, lon
}

func getIDFromFolder(folderName string) string {
	re := regexp.MustCompile(`_(\d+)$`)
	matches := re.FindStringSubmatch(folderName)
	if matches != nil {
		return matches[1]
	}
	return folderName
}

func findUTMFile(folderPath string) string {
	paths := []string{
		filepath.Join(folderPath, "process_result_0", "utm.txt"),
		filepath.Join(folderPath, "utm.txt"),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func safeJoinPath(destDir, name string) (string, error) {
	cleanName := filepath.Clean(name)
	if cleanName == "." {
		return filepath.Abs(destDir)
	}
	if filepath.IsAbs(cleanName) || strings.HasPrefix(cleanName, ".."+string(filepath.Separator)) || cleanName == ".." {
		return "", fmt.Errorf("非法路径: %s", name)
	}
	target := filepath.Join(destDir, cleanName)
	absDest, err := filepath.Abs(destDir)
	if err != nil {
		return "", err
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(absTarget, absDest+string(filepath.Separator)) && absTarget != absDest {
		return "", fmt.Errorf("路径穿越: %s", name)
	}
	return target, nil
}

func extractTarGz(tarPath, destDir string) error {
	file, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target, err := safeJoinPath(destDir, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}
	return nil
}

func extractUTMFromTarGz(tarPath, destDir string) (string, error) {
	file, err := os.Open(tarPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	targetNames := []string{
		"process_result_0/utm.txt",
		"utm.txt",
	}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		found := false
		for _, name := range targetNames {
			if header.Name == name || filepath.Base(header.Name) == "utm.txt" {
				found = true
				break
			}
		}
		if !found {
			continue
		}

		outPath, err := safeJoinPath(destDir, header.Name)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return "", err
		}

		outFile, err := os.Create(outPath)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(outFile, tarReader); err != nil {
			outFile.Close()
			return "", err
		}
		outFile.Close()

		return outPath, nil
	}
	return "", fmt.Errorf("tar.gz 中未找到 utm.txt")
}

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	dest, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dest.Close()

	_, err = io.Copy(dest, source)
	return err
}

func convertUTMToGeoJSON(ctx context.Context, utmFile, trackID string, zone int) ([]GeoJSONFeature, error) {
	file, err := os.Open(utmFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var features []GeoJSONFeature
	scanner := bufio.NewScanner(file)
	idx := 0

	for scanner.Scan() {
		if idx%1000 == 0 {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("任务已被取消")
			default:
			}
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		timestamp := fields[0]
		x, err1 := strconv.ParseFloat(fields[1], 64)
		y, err2 := strconv.ParseFloat(fields[2], 64)
		if err1 != nil || err2 != nil {
			continue
		}

		lat, lon := utmToLatLon(x, y, zone)

		props := map[string]interface{}{
			"track_id":    trackID,
			"point_index": idx,
			"timestamp":   timestamp,
		}

		if len(fields) >= 4 {
			if alt, err := strconv.ParseFloat(fields[3], 64); err == nil {
				props["altitude"] = alt
			}
		}

		feature := GeoJSONFeature{
			Type:       "Feature",
			Properties: props,
			Geometry: GeoJSONGeometry{
				Type:        "Point",
				Coordinates: []float64{lon, lat},
			},
		}
		features = append(features, feature)
		idx++
	}

	return features, nil
}

func saveGeoJSON(features []GeoJSONFeature, outputPath string) error {
	fc := FeatureCollection{Type: "FeatureCollection", Features: features}
	data, err := json.MarshalIndent(fc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, data, 0644)
}

var utmPointFields = []shp.Field{
	shp.StringField("track_id", 50),
	shp.StringField("pointIndex", 20),
	shp.StringField("timestamp", 30),
	shp.StringField("altitude", 20),
}

var utmLineFields = []shp.Field{
	shp.StringField("track_id", 50),
	shp.StringField("pointCount", 20),
	shp.StringField("startTime", 30),
	shp.StringField("endTime", 30),
}

func writeWGS84Prj(prjPath string) error {
	wgs84WKT := `GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9122"]],AUTHORITY["EPSG","4326"]]`
	return os.WriteFile(prjPath, []byte(wgs84WKT), 0644)
}

func fixDbfPath(shpPath string) error {
	dbfPath := strings.TrimSuffix(shpPath, ".shp") + "dbf"
	correctDbfPath := strings.TrimSuffix(shpPath, ".shp") + ".dbf"
	if _, err := os.Stat(dbfPath); err == nil {
		if err := os.Rename(dbfPath, correctDbfPath); err != nil {
			return fmt.Errorf("重命名 dbf 文件失败: %v", err)
		}
	}
	return nil
}

func writeGeoJSONPointSHP(ctx context.Context, shpPath string, features []GeoJSONFeature) error {
	if len(features) == 0 {
		return nil
	}
	writer, err := shp.Create(shpPath, shp.POINT)
	if err != nil {
		return err
	}
	_ = writer.SetFields(utmPointFields)
	for n, f := range features {
		if n%1000 == 0 {
			select {
			case <-ctx.Done():
				writer.Close()
				return fmt.Errorf("任务已被取消")
			default:
			}
		}
		coords := f.Geometry.Coordinates
		if len(coords) < 2 {
			continue
		}
		writer.Write(&shp.Point{X: coords[0], Y: coords[1]})
		trackID, _ := f.Properties["track_id"].(string)
		pointIndex := fmt.Sprintf("%v", f.Properties["point_index"])
		timestamp := fmt.Sprintf("%v", f.Properties["timestamp"])
		altitude := ""
		if alt, ok := f.Properties["altitude"]; ok {
			altitude = fmt.Sprintf("%v", alt)
		}
		_ = writer.WriteAttribute(n, 0, trackID)
		_ = writer.WriteAttribute(n, 1, pointIndex)
		_ = writer.WriteAttribute(n, 2, timestamp)
		_ = writer.WriteAttribute(n, 3, altitude)
	}
	writer.Close()
	if err := writeWGS84Prj(strings.TrimSuffix(shpPath, ".shp") + ".prj"); err != nil {
		return err
	}
	return fixDbfPath(shpPath)
}

func writeGeoJSONLineSHP(ctx context.Context, shpPath string, trackFeatures map[string][]GeoJSONFeature) error {
	if len(trackFeatures) == 0 {
		return nil
	}
	writer, err := shp.Create(shpPath, shp.POLYLINE)
	if err != nil {
		return err
	}
	_ = writer.SetFields(utmLineFields)
	n := 0
	for trackID, features := range trackFeatures {
		if n%1000 == 0 {
			select {
			case <-ctx.Done():
				writer.Close()
				return fmt.Errorf("任务已被取消")
			default:
			}
		}
		if len(features) < 2 {
			continue
		}
		var pts []shp.Point
		for _, f := range features {
			coords := f.Geometry.Coordinates
			if len(coords) >= 2 {
				pts = append(pts, shp.Point{X: coords[0], Y: coords[1]})
			}
		}
		if len(pts) < 2 {
			continue
		}
		line := shp.NewPolyLine([][]shp.Point{pts})
		writer.Write(line)
		startTime := fmt.Sprintf("%v", features[0].Properties["timestamp"])
		endTime := fmt.Sprintf("%v", features[len(features)-1].Properties["timestamp"])
		_ = writer.WriteAttribute(n, 0, trackID)
		_ = writer.WriteAttribute(n, 1, fmt.Sprintf("%d", len(features)))
		_ = writer.WriteAttribute(n, 2, startTime)
		_ = writer.WriteAttribute(n, 3, endTime)
		n++
	}
	writer.Close()
	if err := writeWGS84Prj(strings.TrimSuffix(shpPath, ".shp") + ".prj"); err != nil {
		return err
	}
	return fixDbfPath(shpPath)
}

const (
	artifactGeoJSON = "geojson"
	artifactSHP     = "shp"
	artifactAll     = "all"

	extractModeQuick = "quick"
	extractModeFull  = "full"

	cleanupKeep            = "keep"
	cleanupDeleteExtracted = "delete-extracted"
	cleanupDeleteArchive   = "delete-archive"
	cleanupDeleteAll       = "delete-all"
)

func shouldWriteGeoJSON(artifactSet string) bool {
	return artifactSet == artifactGeoJSON || artifactSet == artifactAll
}

func shouldWriteSHP(artifactSet string) bool {
	return artifactSet == artifactSHP || artifactSet == artifactAll
}

func validateArtifactSet(artifactSet string) error {
	switch artifactSet {
	case artifactGeoJSON, artifactSHP, artifactAll:
		return nil
	default:
		return fmt.Errorf("错误：-artifact-set 只能是 geojson、shp 或 all")
	}
}

func validateExtractMode(mode string) error {
	switch mode {
	case extractModeQuick, extractModeFull:
		return nil
	default:
		return fmt.Errorf("错误：-extract-mode 只能是 quick 或 full")
	}
}

func validateCleanupPolicy(policy string) error {
	switch policy {
	case cleanupKeep, cleanupDeleteExtracted, cleanupDeleteArchive, cleanupDeleteAll:
		return nil
	default:
		return fmt.Errorf("错误：-cleanup-policy 只能是 keep、delete-extracted、delete-archive 或 delete-all")
	}
}

func isWithinDir(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func defaultOutputDir(inputPath string, isDir bool) string {
	if isDir {
		return filepath.Join(inputPath, "output")
	}
	return filepath.Join(filepath.Dir(inputPath), "output")
}

func splitInputPaths(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	})
	paths := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			paths = append(paths, trimmed)
		}
	}
	return paths
}

func deriveTrackIDFromUTMPath(utmPath string) string {
	name := filepath.Base(utmPath)
	lowerName := strings.ToLower(name)
	if lowerName == "utm.txt" {
		return filepath.Base(filepath.Dir(utmPath))
	}
	if strings.HasSuffix(lowerName, ".utm.txt") {
		return name[:len(name)-len(".utm.txt")]
	}
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func allocateTrackID(base string, used map[string]int) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "track"
	}
	if used[base] == 0 {
		used[base] = 1
		return base
	}
	used[base]++
	return fmt.Sprintf("%s_%d", base, used[base])
}

func loadTrackFeatures(ctx context.Context, utmPath, trackID string, zone int) ([]GeoJSONFeature, error) {
	features, err := convertUTMToGeoJSON(ctx, utmPath, trackID, zone)
	if err != nil {
		return nil, err
	}
	if len(features) == 0 {
		return nil, fmt.Errorf("无有效点")
	}
	return features, nil
}

type convertAccumulator struct {
	converted     int
	allFeatures   []GeoJSONFeature
	trackFeatures map[string][]GeoJSONFeature
	usedTrackIDs  map[string]int
}

func newConvertAccumulator() *convertAccumulator {
	return &convertAccumulator{
		trackFeatures: map[string][]GeoJSONFeature{},
		usedTrackIDs:  map[string]int{},
	}
}

func (a *convertAccumulator) addTrack(ctx context.Context, sourceLabel, baseTrackID, utmPath, outputDir string, zone int, artifactSet string, out io.Writer) error {
	trackID := allocateTrackID(baseTrackID, a.usedTrackIDs)
	fmt.Fprintf(out, "转换: %s (track_id: %s)\n", sourceLabel, trackID)
	features, err := loadTrackFeatures(ctx, utmPath, trackID, zone)
	if err != nil {
		return err
	}
	if err := writeTrackArtifacts(ctx, outputDir, trackID, features, artifactSet, out); err != nil {
		return err
	}
	a.allFeatures = append(a.allFeatures, features...)
	a.trackFeatures[trackID] = features
	a.converted++
	return nil
}

func writeTrackArtifacts(ctx context.Context, outputDir, trackID string, features []GeoJSONFeature, artifactSet string, out io.Writer) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	base := filepath.Join(outputDir, trackID)
	if shouldWriteGeoJSON(artifactSet) {
		geojsonPath := base + ".geojson"
		if err := saveGeoJSON(features, geojsonPath); err != nil {
			return err
		}
		fmt.Fprintf(out, "  [GeoJSON] %s (%d 点)\n", filepath.Base(geojsonPath), len(features))
	}

	if !shouldWriteSHP(artifactSet) {
		return nil
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("任务已被取消")
	default:
	}

	pointShp := base + "_point.shp"
	if err := writeGeoJSONPointSHP(ctx, pointShp, features); err != nil {
		fmt.Fprintf(out, "  [警告] 点 SHP 失败: %v\n", err)
	} else {
		fmt.Fprintf(out, "  [SHP] %s\n", filepath.Base(pointShp))
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("任务已被取消")
	default:
	}

	lineShp := base + "_line.shp"
	if err := writeGeoJSONLineSHP(ctx, lineShp, map[string][]GeoJSONFeature{trackID: features}); err != nil {
		fmt.Fprintf(out, "  [警告] 线 SHP 失败: %v\n", err)
	} else {
		fmt.Fprintf(out, "  [SHP] %s\n", filepath.Base(lineShp))
	}

	return nil
}

func writeMergedArtifacts(ctx context.Context, outputDir, baseName string, allFeatures []GeoJSONFeature, trackFeatures map[string][]GeoJSONFeature, artifactSet string, out io.Writer) error {
	if len(allFeatures) == 0 {
		return nil
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	base := filepath.Join(outputDir, baseName)
	if shouldWriteGeoJSON(artifactSet) {
		geojsonPath := base + ".geojson"
		if err := saveGeoJSON(allFeatures, geojsonPath); err != nil {
			return err
		}
		fmt.Fprintf(out, "  [合并 GeoJSON] %s (%d 点)\n", filepath.Base(geojsonPath), len(allFeatures))
	}

	if !shouldWriteSHP(artifactSet) {
		return nil
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("任务已被取消")
	default:
	}

	pointShp := base + "_point.shp"
	if err := writeGeoJSONPointSHP(ctx, pointShp, allFeatures); err != nil {
		fmt.Fprintf(out, "  [警告] 合并点 SHP 失败: %v\n", err)
	} else {
		fmt.Fprintf(out, "  [合并 SHP] %s\n", filepath.Base(pointShp))
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("任务已被取消")
	default:
	}

	lineShp := base + "_line.shp"
	if err := writeGeoJSONLineSHP(ctx, lineShp, trackFeatures); err != nil {
		fmt.Fprintf(out, "  [警告] 合并线 SHP 失败: %v\n", err)
	} else {
		fmt.Fprintf(out, "  [合并 SHP] %s\n", filepath.Base(lineShp))
	}

	return nil
}

func convertSingleFile(ctx context.Context, utmPath, outputDir string, zone int, artifactSet string, out io.Writer) error {
	acc := newConvertAccumulator()
	if err := acc.addTrack(ctx, utmPath, deriveTrackIDFromUTMPath(utmPath), utmPath, outputDir, zone, artifactSet, out); err != nil {
		return err
	}
	fmt.Fprintf(out, "\n完成！输出目录: %s\n", outputDir)
	return nil
}

func convertDirectoryInto(ctx context.Context, dirPath, outputDir string, zone int, artifactSet string, out io.Writer, acc *convertAccumulator) error {
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path != dirPath && isWithinDir(path, outputDir) {
				return filepath.SkipDir
			}
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("任务已被取消")
		default:
		}

		name := strings.ToLower(info.Name())
		if name != "utm.txt" && !strings.HasSuffix(name, ".utm.txt") {
			return nil
		}

		if err := acc.addTrack(ctx, path, deriveTrackIDFromUTMPath(path), path, outputDir, zone, artifactSet, out); err != nil {
			if err.Error() == "任务已被取消" {
				return err
			}
			fmt.Fprintf(out, "  [错误] %v\n", err)
			return nil
		}
		return nil
	})
	return err
}

func resolveUTMPathFromInputDir(dirPath string) (utmPath, extractedFile string, err error) {
	utmPath = findUTMFile(dirPath)
	if utmPath != "" {
		return utmPath, "", nil
	}

	tarPath := filepath.Join(dirPath, "process_result_0.tar.gz")
	if _, statErr := os.Stat(tarPath); statErr != nil {
		if os.IsNotExist(statErr) {
			return "", "", nil
		}
		return "", "", statErr
	}

	extractedFile, err = extractUTMFromTarGz(tarPath, dirPath)
	if err != nil {
		return "", "", err
	}
	utmPath = findUTMFile(dirPath)
	if utmPath == "" {
		cleanExtractedFile(extractedFile, dirPath, io.Discard)
		return "", "", fmt.Errorf("解压/提取后仍未找到 utm.txt: %s", dirPath)
	}
	return utmPath, extractedFile, nil
}

func convertDirectory(ctx context.Context, dirPath, outputDir string, zone int, artifactSet string, out io.Writer) error {
	acc := newConvertAccumulator()
	if err := convertDirectoryInto(ctx, dirPath, outputDir, zone, artifactSet, out, acc); err != nil {
		return err
	}
	if acc.converted == 0 {
		return fmt.Errorf("未找到任何 utm.txt 文件")
	}

	fmt.Fprintf(out, "\n完成！共转换 %d 个文件\n", acc.converted)
	if acc.converted > 1 {
		if err := writeMergedArtifacts(ctx, outputDir, "merged_tracks", acc.allFeatures, acc.trackFeatures, artifactSet, out); err != nil {
			return err
		}
	}
	fmt.Fprintf(out, "输出目录: %s\n", outputDir)
	return nil
}

func convertInputs(ctx context.Context, inputRaw, outputDir string, zone int, artifactSet string, out io.Writer) error {
	paths := splitInputPaths(inputRaw)
	if len(paths) == 0 {
		return fmt.Errorf("错误：-convert 至少需要一个文件或目录路径")
	}
	if len(paths) > 1 && outputDir == "" {
		return fmt.Errorf("错误：多个 -convert 路径同时输入时，必须显式指定 -output 输出目录")
	}

	acc := newConvertAccumulator()
	for _, inputPath := range paths {
		info, err := os.Stat(inputPath)
		if err != nil {
			return fmt.Errorf("路径不存在: %s", inputPath)
		}

		if !info.IsDir() {
			if !strings.HasSuffix(strings.ToLower(inputPath), ".txt") {
				return fmt.Errorf("不支持的文件类型: %s", inputPath)
			}
			if err := acc.addTrack(ctx, inputPath, deriveTrackIDFromUTMPath(inputPath), inputPath, outputDir, zone, artifactSet, out); err != nil {
				return err
			}
			continue
		}

		utmPath, extractedFile, err := resolveUTMPathFromInputDir(inputPath)
		if err != nil {
			return err
		}
		if utmPath != "" {
			baseTrackID := getIDFromFolder(filepath.Base(inputPath))
			err = acc.addTrack(ctx, inputPath, baseTrackID, utmPath, outputDir, zone, artifactSet, out)
			cleanExtractedFile(extractedFile, inputPath, out)
			if err != nil {
				return err
			}
			continue
		}

		if err := convertDirectoryInto(ctx, inputPath, outputDir, zone, artifactSet, out, acc); err != nil {
			if err.Error() == "任务已被取消" {
				return err
			}
			return err
		}
	}

	if acc.converted == 0 {
		return fmt.Errorf("未找到任何 utm.txt 文件")
	}

	fmt.Fprintf(out, "\n完成！共转换 %d 个输入轨迹\n", acc.converted)
	if acc.converted > 1 {
		if err := writeMergedArtifacts(ctx, outputDir, "merged_tracks", acc.allFeatures, acc.trackFeatures, artifactSet, out); err != nil {
			return err
		}
	}
	fmt.Fprintf(out, "输出目录: %s\n", outputDir)
	return nil
}

func mergeGeoJSON(ctx context.Context, dirPath, outputDir, artifactSet string, out io.Writer) error {
	var allFeatures []GeoJSONFeature
	trackFeatures := map[string][]GeoJSONFeature{}

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path != dirPath && isWithinDir(path, outputDir) {
				return filepath.SkipDir
			}
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("任务已被取消")
		default:
		}

		if !strings.HasSuffix(strings.ToLower(path), ".geojson") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(out, "读取失败 %s: %v\n", path, err)
			return nil
		}
		var fc FeatureCollection
		if err := json.Unmarshal(data, &fc); err != nil {
			fmt.Fprintf(out, "解析失败 %s: %v\n", path, err)
			return nil
		}
		allFeatures = append(allFeatures, fc.Features...)
		for _, f := range fc.Features {
			trackID, _ := f.Properties["track_id"].(string)
			trackFeatures[trackID] = append(trackFeatures[trackID], f)
		}
		fmt.Fprintf(out, "合并: %s (%d 个特征点)\n", path, len(fc.Features))
		return nil
	})
	if err != nil {
		return err
	}
	if len(allFeatures) == 0 {
		return fmt.Errorf("未找到任何 GeoJSON 文件")
	}
	if err := writeMergedArtifacts(ctx, outputDir, "merged_tracks", allFeatures, trackFeatures, artifactSet, out); err != nil {
		return err
	}
	fmt.Fprintf(out, "\n合并完成！输出目录: %s\n", outputDir)
	return nil
}

func cleanExtractedFile(extractedFile, folderPath string, out io.Writer) {
	if extractedFile == "" {
		return
	}
	dir := filepath.Dir(extractedFile)
	if strings.HasPrefix(dir, folderPath) && filepath.Base(dir) == "process_result_0" {
		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintf(out, "  [清理警告] 无法删除 %s: %v\n", dir, err)
		}
	} else if strings.HasPrefix(dir, folderPath) && dir == folderPath {
		if err := os.Remove(extractedFile); err != nil {
			fmt.Fprintf(out, "  [清理警告] 无法删除 %s: %v\n", extractedFile, err)
		}
	}
}

func runBatchMode(ctx context.Context, inputDir, outputDir string, zone int, extractMode string, workers int, cleanupPolicy string, artifactSet string, out io.Writer) error {
	if workers <= 0 {
		return fmt.Errorf("错误：-workers 必须大于 0")
	}
	if _, err := os.Stat(inputDir); os.IsNotExist(err) {
		return fmt.Errorf("输入目录不存在: %s", inputDir)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	utmBackupDir := filepath.Join(outputDir, "utm")
	if err := os.MkdirAll(utmBackupDir, 0755); err != nil {
		return fmt.Errorf("创建备份目录失败: %w", err)
	}

	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "输入目录:", inputDir)
	fmt.Fprintln(out, "输出目录:", outputDir)
	fmt.Fprintf(out, "并发线程数: %d\n", workers)

	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return err
	}
	var folders []string
	for _, e := range entries {
		if e.IsDir() {
			folders = append(folders, e.Name())
		}
	}
	sort.Strings(folders)
	fmt.Fprintf(out, "\n找到 %d 个文件夹，开始处理...\n\n", len(folders))

	var (
		wg               sync.WaitGroup
		mu               sync.Mutex
		allFeatures      []GeoJSONFeature
		trackFeatures    = map[string][]GeoJSONFeature{}
		usedTrackIDs     = map[string]int{}
		processedFolders []string
		sem              = make(chan struct{}, workers)
	)

	for _, folder := range folders {
		wg.Add(1)
		go func(folder string) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()

			folderPath := filepath.Join(inputDir, folder)
			trackID := getIDFromFolder(folder)
			uniqueTrackID := trackID

			utmFile := findUTMFile(folderPath)
			var extractedFile string

			if utmFile == "" {
				tarPath := filepath.Join(folderPath, "process_result_0.tar.gz")
				if _, err := os.Stat(tarPath); err == nil {
					if extractMode == extractModeFull {
						fmt.Fprintf(out, "  [完整解压] %s ...\n", filepath.Base(tarPath))
						os.RemoveAll(filepath.Join(folderPath, "process_result_0"))
						if err := extractTarGz(tarPath, folderPath); err != nil {
							fmt.Fprintf(out, "  [错误] 解压失败 %s: %v\n", folder, err)
							return
						}
						fmt.Fprintf(out, "  解压完成\n")
					} else {
						fmt.Fprintf(out, "  [快速提取] utm.txt 从 %s ...\n", filepath.Base(tarPath))
						f, err := extractUTMFromTarGz(tarPath, folderPath)
						if err != nil {
							fmt.Fprintf(out, "  [错误] 提取失败 %s: %v\n", folder, err)
							return
						}
						extractedFile = f
						fmt.Fprintf(out, "  提取完成: %s\n", filepath.Base(extractedFile))
					}
					utmFile = findUTMFile(folderPath)
					if utmFile == "" {
						fmt.Fprintf(out, "  [错误] 解压/提取后仍未找到 utm.txt: %s\n", folder)
						return
					}
				} else {
					fmt.Fprintf(out, "  [跳过] %s (无 utm.txt 且无 process_result_0.tar.gz)\n", folder)
					return
				}
			} else {
				fmt.Fprintf(out, "  [已解压] %s\n", folder)
			}

			select {
			case <-ctx.Done():
				cleanExtractedFile(extractedFile, folderPath, out)
				return
			default:
			}

			mu.Lock()
			uniqueTrackID = allocateTrackID(trackID, usedTrackIDs)
			mu.Unlock()

			features, err := loadTrackFeatures(ctx, utmFile, uniqueTrackID, zone)
			if err != nil {
				if err.Error() == "任务已被取消" {
					cleanExtractedFile(extractedFile, folderPath, out)
					return
				}
				fmt.Fprintf(out, "  [错误] %s: %v\n", uniqueTrackID, err)
				cleanExtractedFile(extractedFile, folderPath, out)
				return
			}
			if err := writeTrackArtifacts(ctx, outputDir, uniqueTrackID, features, artifactSet, out); err != nil {
				fmt.Fprintf(out, "  [错误] 保存输出失败 %s: %v\n", uniqueTrackID, err)
				cleanExtractedFile(extractedFile, folderPath, out)
				return
			}

			dstUtm := filepath.Join(utmBackupDir, uniqueTrackID+".utm.txt")
			if err := copyFile(utmFile, dstUtm); err != nil {
				fmt.Fprintf(out, "  [警告] 备份 utm 失败 %s: %v\n", uniqueTrackID, err)
			}

			cleanExtractedFile(extractedFile, folderPath, out)

			mu.Lock()
			allFeatures = append(allFeatures, features...)
			trackFeatures[uniqueTrackID] = features
			processedFolders = append(processedFolders, folderPath)
			mu.Unlock()
		}(folder)
	}

	wg.Wait()

	if len(allFeatures) > 0 {
		if err := writeMergedArtifacts(ctx, outputDir, "merged_tracks", allFeatures, trackFeatures, artifactSet, out); err != nil {
			return err
		}
	}

	fmt.Fprintf(out, "\n处理完成！共处理 %d 个文件夹\n", len(processedFolders))
	fmt.Fprintf(out, "输出目录: %s\n", outputDir)
	fmt.Fprintf(out, "UTM 备份目录: %s\n", utmBackupDir)

	if extractMode == extractModeFull {
		if cleanupPolicy == cleanupDeleteExtracted || cleanupPolicy == cleanupDeleteAll {
			fmt.Fprintln(out, "\n正在删除解压产生的文件...")
			for _, folder := range processedFolders {
				processDir := filepath.Join(folder, "process_result_0")
				if _, err := os.Stat(processDir); err == nil {
					if err := os.RemoveAll(processDir); err == nil {
						fmt.Fprintf(out, "  已删除: %s\n", processDir)
					}
				}
				cityFile := filepath.Join(folder, "city.txt")
				if _, err := os.Stat(cityFile); err == nil {
					if err := os.Remove(cityFile); err == nil {
						fmt.Fprintf(out, "  已删除: %s\n", cityFile)
					}
				}
			}
		}
		if cleanupPolicy == cleanupDeleteArchive || cleanupPolicy == cleanupDeleteAll {
			fmt.Fprintln(out, "\n正在删除压缩包...")
			for _, folder := range processedFolders {
				tarFile := filepath.Join(folder, "process_result_0.tar.gz")
				if _, err := os.Stat(tarFile); err == nil {
					if err := os.Remove(tarFile); err == nil {
						fmt.Fprintf(out, "  已删除: %s\n", tarFile)
					}
				}
			}
		}
	}
	return nil
}

// Custom parsing logic has been moved to framework.ParseArgs

// ---------------- Tool Integration ----------------

type UTMTool struct{}

func (t *UTMTool) ID() string       { return "utm_geojson_converter" }
func (t *UTMTool) Name() string     { return "点云UTM提取&UTM转换GeoJSON+Shapefile" }
func (t *UTMTool) Category() string { return "KD测试工具 > 点云处理工具" }

func (t *UTMTool) Execute(ctx framework.AppContext) {
	usage := `[yellow]UTM 轨迹提取与 GIS 转换工具[-]

[cyan]工具用途:[-]
将点云/无人车资料中的 UTM 轨迹文本转换为 GIS 可直接使用的 GeoJSON 与 Shapefile。
支持三种工作模式：
  1. 批量处理 out_source 目录，自动从 process_result_0.tar.gz 中提取或解压 utm.txt
  2. 直接转换单个 utm.txt，或转换包含多个 utm.txt 的目录
  3. 合并已有的 GeoJSON 结果，重新输出 merged_tracks 结果

[cyan]输出规则:[-]
  - 所有模式都支持 -output 指定输出目录
  - 不指定时，默认在输入路径旁边创建 output 目录
  - 输出内容由 -artifact-set 控制：
      geojson = 仅输出 .geojson
      shp     = 仅输出点/线 Shapefile
      all     = 同时输出 GeoJSON 和 Shapefile

[cyan]参数说明:[-]
  -input <目录路径>             [批量模式] 批量处理 out_source 目录
  -convert <路径列表>           [转换模式] 支持单个 utm.txt、单个目录，或多个路径批量转换
  -merge <目录路径>             [合并模式] 合并目录内所有 .geojson 文件
  -output <目录路径>            可选，指定输出目录
  -artifact-set <geojson|shp|all>
                               可选，控制输出产物，默认 all
  -zone <int>                   [批量/转换] UTM Zone，默认 50
  -workers <int>                [批量模式] 并发数，默认 4
  -extract-mode <quick|full>    [批量模式] 提取策略，默认 quick
                               quick = 只提取 utm.txt，速度快，临时文件自动清理
                               full  = 完整解压 process_result_0.tar.gz 后再转换
  -cleanup-policy <keep|delete-extracted|delete-archive|delete-all>
                               [批量模式] 仅在 extract-mode=full 时生效
                               keep             = 保留解压结果和压缩包
                               delete-extracted = 删除解压出的文件
                               delete-archive   = 删除原始压缩包
                               delete-all       = 同时删除两者

[cyan]常用示例:[-]
1. 批量处理 out_source，快速提取并输出全部产物:
   -input "<out_source目录>" -workers 4 -extract-mode quick -artifact-set all

2. 批量处理并完整解压，转换后删除解压目录:
   -input "<out_source目录>" -extract-mode full -cleanup-policy delete-extracted

3. 转换单个 utm.txt 到指定目录:
   -convert "<utm.txt文件路径>" -output "<输出目录>" -zone 50

4. 转换多个目录并合并输出:
   -convert "D:\a,D:\b,D:\c" -output "D:\merged_output" -artifact-set shp -zone 50

5. 合并目录内已有 GeoJSON:
   -merge "<geojson目录>" -output "<输出目录>" -artifact-set shp
`

	ctx.ShowTerminal(t.Name(), usage, func(runCtx context.Context, args string, out io.Writer) error {
		// Parse args manually using framework parser
		parsedArgs, err := framework.ParseArgs(args)
		if err != nil {
			return err
		}

		fs := flag.NewFlagSet("utm_tool", flag.ContinueOnError)
		fs.SetOutput(out)

		var inputDir string
		var zone int
		var convertPath string
		var mergeDir string
		var outputDir string
		var artifactSet string
		var extractMode string
		var workers int
		var cleanupPolicy string

		fs.StringVar(&inputDir, "input", "", "")
		fs.IntVar(&zone, "zone", 50, "")
		fs.StringVar(&convertPath, "convert", "", "")
		fs.StringVar(&mergeDir, "merge", "", "")
		fs.StringVar(&outputDir, "output", "", "")
		fs.StringVar(&artifactSet, "artifact-set", artifactAll, "")
		fs.StringVar(&extractMode, "extract-mode", extractModeQuick, "")
		fs.IntVar(&workers, "workers", 4, "")
		fs.StringVar(&cleanupPolicy, "cleanup-policy", cleanupKeep, "")

		if err := fs.Parse(parsedArgs); err != nil {
			return err
		}
		if zone < 1 || zone > 60 {
			return fmt.Errorf("错误：-zone 必须在 1 到 60 之间")
		}
		if err := validateArtifactSet(artifactSet); err != nil {
			return err
		}
		if err := validateExtractMode(extractMode); err != nil {
			return err
		}
		if err := validateCleanupPolicy(cleanupPolicy); err != nil {
			return err
		}

		modeCount := 0
		if inputDir != "" {
			modeCount++
		}
		if convertPath != "" {
			modeCount++
		}
		if mergeDir != "" {
			modeCount++
		}
		if modeCount > 1 {
			return fmt.Errorf("错误：-input、-convert、-merge 不能同时使用，请选择一个模式")
		}
		if modeCount == 0 {
			return fmt.Errorf("错误：必须指定一个运行模式 (-input / -convert / -merge)")
		}
		if extractMode != extractModeFull && cleanupPolicy != cleanupKeep {
			return fmt.Errorf("错误：只有在 -extract-mode full 时才能使用非 keep 的 -cleanup-policy")
		}

		if inputDir != "" {
			if outputDir == "" {
				outputDir = defaultOutputDir(inputDir, true)
			}
			return runBatchMode(runCtx, inputDir, outputDir, zone, extractMode, workers, cleanupPolicy, artifactSet, out)
		}
		if convertPath != "" {
			if outputDir == "" {
				convertPaths := splitInputPaths(convertPath)
				if len(convertPaths) == 1 {
					info, err := os.Stat(convertPaths[0])
					if err != nil {
						return fmt.Errorf("路径不存在: %s", convertPaths[0])
					}
					outputDir = defaultOutputDir(convertPaths[0], info.IsDir())
				}
			}
			return convertInputs(runCtx, convertPath, outputDir, zone, artifactSet, out)
		}
		if mergeDir != "" {
			if outputDir == "" {
				outputDir = defaultOutputDir(mergeDir, true)
			}
			return mergeGeoJSON(runCtx, mergeDir, outputDir, artifactSet, out)
		}
		return nil
	})
}

func init() {
	framework.Register(&UTMTool{})
}
