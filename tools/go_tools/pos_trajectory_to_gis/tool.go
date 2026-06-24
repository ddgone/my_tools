package pos_trajectory_to_gis

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"my_tools/libs/core/procutil"
	"my_tools/libs/framework"

	"github.com/jonas-p/go-shp"
)

type PointData struct {
	X               float64 `json:"x"`
	Y               float64 `json:"y"`
	Z               float64 `json:"z"`
	Timestamp       int64   `json:"timestamp"`
	Azimuth         float64 `json:"azimuth"`
	Pitch           float64 `json:"pitch"`
	Roll            float64 `json:"roll"`
	VideoFile       string  `json:"videoFile"`
	VideoFrameIndex string  `json:"videoFrameIndex"`
}

type InputJSON struct {
	TrackID   string      `json:"trackId"`
	PointList []PointData `json:"pointList"`
}

type GeoJSONFeature struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Geometry   GeoJSONPointGeometry   `json:"geometry"`
}

type GeoJSONPointGeometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

type GeoJSONFeatureCollection struct {
	Type     string           `json:"type"`
	Features []GeoJSONFeature `json:"features"`
}

type ExtractedPoint struct {
	PointData
	SourceFile string
	TrackID    string
}

var pointFields = []shp.Field{
	shp.StringField("trackId", 50),
	shp.StringField("sourceFile", 100),
	shp.StringField("timestamp", 30),
	shp.StringField("azimuth", 20),
	shp.StringField("pitch", 20),
	shp.StringField("roll", 20),
	shp.StringField("videoFile", 100),
	shp.StringField("videoFrameIdx", 20),
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

func validateArtifactSet(artifactSet string) error {
	switch artifactSet {
	case artifactGeoJSON, artifactSHP, artifactAll:
		return nil
	default:
		return fmt.Errorf("错误：-artifact-set 只能是 geojson、shp 或 all")
	}
}

func parseJSONFile(ctx context.Context, path string) ([]ExtractedPoint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var input InputJSON
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, err
	}

	fileName := filepath.Base(path)
	var points []ExtractedPoint
	for i, p := range input.PointList {
		if i%1000 == 0 {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("任务已被取消")
			default:
			}
		}
		ep := ExtractedPoint{
			PointData:  p,
			SourceFile: fileName,
			TrackID:    input.TrackID,
		}
		points = append(points, ep)
	}
	return points, nil
}

func writeGeoJSON(ctx context.Context, path string, points []ExtractedPoint) error {
	features := make([]GeoJSONFeature, 0, len(points))
	for i, p := range points {
		if i%1000 == 0 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("任务已被取消")
			default:
			}
		}
		feature := GeoJSONFeature{
			Type: "Feature",
			Properties: map[string]interface{}{
				"trackId":         p.TrackID,
				"sourceFile":      p.SourceFile,
				"timestamp":       p.Timestamp,
				"azimuth":         p.Azimuth,
				"pitch":           p.Pitch,
				"roll":            p.Roll,
				"videoFile":       p.VideoFile,
				"videoFrameIndex": p.VideoFrameIndex,
			},
			Geometry: GeoJSONPointGeometry{
				Type:        "Point",
				Coordinates: []float64{p.X, p.Y, p.Z},
			},
		}
		features = append(features, feature)
	}

	fc := GeoJSONFeatureCollection{
		Type:     "FeatureCollection",
		Features: features,
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

func writePointSHP(ctx context.Context, path string, points []ExtractedPoint) error {
	writer, err := shp.Create(path, shp.POINT)
	if err != nil {
		return err
	}

	_ = writer.SetFields(pointFields)

	for n, p := range points {
		if n%1000 == 0 {
			select {
			case <-ctx.Done():
				writer.Close()
				return fmt.Errorf("任务已被取消")
			default:
			}
		}
		pt := shp.Point{X: p.X, Y: p.Y}
		writer.Write(&pt)
		_ = writer.WriteAttribute(n, 0, p.TrackID)
		_ = writer.WriteAttribute(n, 1, p.SourceFile)
		_ = writer.WriteAttribute(n, 2, fmt.Sprintf("%d", p.Timestamp))
		_ = writer.WriteAttribute(n, 3, fmt.Sprintf("%.6f", p.Azimuth))
		_ = writer.WriteAttribute(n, 4, fmt.Sprintf("%.6f", p.Pitch))
		_ = writer.WriteAttribute(n, 5, fmt.Sprintf("%.6f", p.Roll))
		_ = writer.WriteAttribute(n, 6, p.VideoFile)
		_ = writer.WriteAttribute(n, 7, p.VideoFrameIndex)
	}

	writer.Close()

	if err := writeWGS84Prj(strings.TrimSuffix(path, ".shp") + ".prj"); err != nil {
		return err
	}

	dbfPath := strings.TrimSuffix(path, ".shp") + "dbf"
	correctDbfPath := strings.TrimSuffix(path, ".shp") + ".dbf"
	if _, err := os.Stat(dbfPath); err == nil {
		if err := os.Rename(dbfPath, correctDbfPath); err != nil {
			return fmt.Errorf("重命名 dbf 文件失败: %v", err)
		}
	}

	return nil
}

func writeWGS84Prj(prjPath string) error {
	wgs84WKT := `GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9122"]],AUTHORITY["EPSG","4326"]]`
	return os.WriteFile(prjPath, []byte(wgs84WKT), 0644)
}

func defaultOutputDir(inputDir string) string {
	return filepath.Join(inputDir, "output")
}

func allocateArtifactBase(base string, used map[string]int) string {
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

func writeTrackArtifacts(ctx context.Context, outputDir, baseName string, points []ExtractedPoint, artifactSet string, out io.Writer) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录 '%s' 失败: %v", outputDir, err)
	}

	base := filepath.Join(outputDir, baseName)
	if shouldWriteGeoJSON(artifactSet) {
		geojsonPath := base + ".geojson"
		if err := writeGeoJSON(ctx, geojsonPath, points); err != nil {
			return err
		}
		fmt.Fprintf(out, "  [GeoJSON] %s (%d 点)\n", filepath.Base(geojsonPath), len(points))
	}

	if !shouldWriteSHP(artifactSet) {
		return nil
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("任务已被取消")
	default:
	}

	pointShpPath := base + "_point.shp"
	if err := writePointSHP(ctx, pointShpPath, points); err != nil {
		return err
	}
	fmt.Fprintf(out, "  [SHP] %s\n", filepath.Base(pointShpPath))
	return nil
}

func writeMergedArtifacts(ctx context.Context, outputDir string, points []ExtractedPoint, artifactSet string, out io.Writer) error {
	return writeTrackArtifacts(ctx, outputDir, "merged_pos", points, artifactSet, out)
}

func runConvert(ctx context.Context, inputDir, outputDir, artifactSet string, out io.Writer) error {
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

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录 '%s' 失败: %v", outputDir, err)
	}

	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return err
	}

	var jsonFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			jsonFiles = append(jsonFiles, filepath.Join(inputDir, entry.Name()))
		}
	}

	var allPoints []ExtractedPoint
	usedBaseNames := map[string]int{}
	converted := 0

	for _, jsonPath := range jsonFiles {
		select {
		case <-ctx.Done():
			return fmt.Errorf("任务已被取消")
		default:
		}

		points, err := parseJSONFile(ctx, jsonPath)
		if err != nil {
			if err.Error() == "任务已被取消" {
				return err
			}
			fmt.Fprintf(out, "处理 %s 出错: %v\n", jsonPath, err)
			continue
		}
		if len(points) == 0 {
			continue
		}

		baseName := allocateArtifactBase(strings.TrimSuffix(filepath.Base(jsonPath), filepath.Ext(jsonPath)), usedBaseNames)
		fmt.Fprintf(out, "转换: %s\n", filepath.Base(jsonPath))
		if err := writeTrackArtifacts(ctx, outputDir, baseName, points, artifactSet, out); err != nil {
			return err
		}

		allPoints = append(allPoints, points...)
		converted++
	}

	if converted == 0 {
		return fmt.Errorf("未找到任何有效 POS JSON 轨迹")
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("任务已被取消")
	default:
	}

	fmt.Fprintf(out, "\n转换完成！共处理 %d 个 JSON 文件，%d 个有效轨迹，%d 个点\n", len(jsonFiles), converted, len(allPoints))
	if converted > 1 {
		if err := writeMergedArtifacts(ctx, outputDir, allPoints, artifactSet, out); err != nil {
			return err
		}
	}
	fmt.Fprintf(out, "输出目录: %s\n", outputDir)
	return nil
}

type POSTool struct{}

func (t *POSTool) ID() string       { return "pos2gis_converter" }
func (t *POSTool) Name() string     { return "点云pos轨迹转换GeoJSON+Shapefile" }
func (t *POSTool) Category() string { return "KD测试工具 > 点云处理工具" }

func (t *POSTool) Execute(ctx framework.AppContext) {
	usage := `[yellow]点云 POS 轨迹转换 GeoJSON + Shapefile 工具[-]

[cyan]说明:[-]
本工具用于将点云 POS 编译系统输出的 JSON 轨迹数据转换为 GeoJSON 点集和点 Shapefile，便于在 GIS 软件中查看和分析。

[cyan]参数详解:[-]
  -input <目录路径>         指定包含 POS JSON 文件的目录。
                             程序会遍历该目录下所有 .json 文件进行转换。
  -output <目录路径>        指定输出目录。默认在输入目录下创建 output 子文件夹。
  -artifact-set <模式>      输出内容：all、shp、geojson。
                             all: GeoJSON + 点 Shapefile
                             shp: 仅点 Shapefile
                             geojson: 仅 GeoJSON

[cyan]实际运行示例 (可以直接复制到下方输入):[-]

1. 基本用法（只指定输入目录，输出默认放 input/output）:
   -input "<你的pos数据目录>"

2. 指定输出目录:
   -input "<你的pos数据目录>" -output "<输出目录>"

3. 仅输出点 Shapefile:
   -input "<你的pos数据目录>" -artifact-set shp
`

	ctx.ShowTerminal(t.Name(), usage, func(runCtx context.Context, args string, out io.Writer) error {
		parsedArgs, err := procutil.ParseArgs(args)
		if err != nil {
			return err
		}

		fs := flag.NewFlagSet("pos2gis", flag.ContinueOnError)
		fs.SetOutput(out)

		var inputDir string
		var outputDir string
		var artifactSet string

		fs.StringVar(&inputDir, "input", "", "")
		fs.StringVar(&outputDir, "output", "", "")
		fs.StringVar(&artifactSet, "artifact-set", artifactAll, "")

		if err := fs.Parse(parsedArgs); err != nil {
			return err
		}

		if inputDir == "" {
			return fmt.Errorf("错误：必须指定 -input 参数")
		}

		if err := validateArtifactSet(artifactSet); err != nil {
			return err
		}

		if outputDir == "" {
			outputDir = defaultOutputDir(inputDir)
		}

		return runConvert(runCtx, inputDir, outputDir, artifactSet, out)
	})
}

func init() {
	framework.Register(&POSTool{})
}
