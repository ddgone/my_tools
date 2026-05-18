package pos2gis

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"my_tools/pkg/framework"

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

var lineFields = []shp.Field{
	shp.StringField("trackId", 50),
	shp.StringField("sourceFile", 100),
	shp.StringField("pointCount", 20),
	shp.StringField("startTime", 30),
	shp.StringField("endTime", 30),
	shp.StringField("startVideo", 100),
	shp.StringField("endVideo", 100),
}

func parseJSONFile(path string) ([]ExtractedPoint, error) {
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
	for _, p := range input.PointList {
		ep := ExtractedPoint{
			PointData:  p,
			SourceFile: fileName,
			TrackID:    input.TrackID,
		}
		points = append(points, ep)
	}
	return points, nil
}

func writeGeoJSON(path string, points []ExtractedPoint) error {
	features := make([]GeoJSONFeature, 0, len(points))
	for _, p := range points {
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

func writePointSHP(path string, points []ExtractedPoint) error {
	writer, err := shp.Create(path, shp.POINT)
	if err != nil {
		return err
	}

	_ = writer.SetFields(pointFields)

	for n, p := range points {
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

func writeLineSHP(path string, fileLines map[string][]ExtractedPoint) error {
	writer, err := shp.Create(path, shp.POLYLINE)
	if err != nil {
		return err
	}

	_ = writer.SetFields(lineFields)

	n := 0
	for fileName, points := range fileLines {
		if len(points) < 2 {
			continue
		}

		var pts []shp.Point
		for _, p := range points {
			pts = append(pts, shp.Point{X: p.X, Y: p.Y})
		}
		line := shp.NewPolyLine([][]shp.Point{pts})
		writer.Write(line)
		_ = writer.WriteAttribute(n, 0, points[0].TrackID)
		_ = writer.WriteAttribute(n, 1, fileName)
		_ = writer.WriteAttribute(n, 2, fmt.Sprintf("%d", len(points)))
		_ = writer.WriteAttribute(n, 3, fmt.Sprintf("%d", points[0].Timestamp))
		_ = writer.WriteAttribute(n, 4, fmt.Sprintf("%d", points[len(points)-1].Timestamp))
		_ = writer.WriteAttribute(n, 5, points[0].VideoFile)
		_ = writer.WriteAttribute(n, 6, points[len(points)-1].VideoFile)
		n++
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

func runConvert(inputDir, outputDir string, out io.Writer) error {
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
	linesFromFiles := make(map[string][]ExtractedPoint)

	for _, jsonPath := range jsonFiles {
		points, err := parseJSONFile(jsonPath)
		if err != nil {
			fmt.Fprintf(out, "处理 %s 出错: %v\n", jsonPath, err)
			continue
		}
		if len(points) == 0 {
			continue
		}
		allPoints = append(allPoints, points...)
		fileName := filepath.Base(jsonPath)
		linesFromFiles[fileName] = points
	}

	geojsonPath := filepath.Join(outputDir, "merged_pos_point.geojson")
	if err := writeGeoJSON(geojsonPath, allPoints); err != nil {
		return err
	}
	fmt.Fprintf(out, "GeoJSON 输出: %s (%d 点)\n", geojsonPath, len(allPoints))

	pointShpPath := filepath.Join(outputDir, "merged_pos_point.shp")
	if err := writePointSHP(pointShpPath, allPoints); err != nil {
		return err
	}
	fmt.Fprintf(out, "点 Shapefile 输出: %s\n", pointShpPath)

	lineShpPath := filepath.Join(outputDir, "merged_pos_line.shp")
	if err := writeLineSHP(lineShpPath, linesFromFiles); err != nil {
		return err
	}
	fmt.Fprintf(out, "线 Shapefile 输出: %s\n", lineShpPath)

	fmt.Fprintf(out, "\n转换完成！共处理 %d 个 JSON 文件，%d 个点\n", len(jsonFiles), len(allPoints))
	return nil
}

type POSTool struct{}

func (t *POSTool) ID() string       { return "pos2gis_converter" }
func (t *POSTool) Name() string     { return "点云pos轨迹转换GeoJSON+Shapefile" }
func (t *POSTool) Category() string { return "数据处理" }

func (t *POSTool) Execute(ctx framework.AppContext) {
	usage := `[yellow]点云 POS 轨迹转换 GeoJSON + Shapefile 工具[-]

[cyan]说明:[-]
本工具用于将点云 POS 编译系统输出的 JSON 轨迹数据转换为 GeoJSON 点集和 Shapefile（点+线）格式，便于在 GIS 软件中查看和分析。

[cyan]参数详解:[-]
  -input <目录路径>         指定包含 POS JSON 文件的目录。
                             程序会遍历该目录下所有 .json 文件进行转换。
  -output <目录路径>        指定输出目录。默认在输入目录下创建 output 子文件夹。

[cyan]实际运行示例 (可以直接复制到下方输入):[-]

1. 基本用法（只指定输入目录，输出默认放 input/output）:
   -input "C:\Users\zhangzijiang\Desktop\my_tools\test\pos_data"

2. 指定输出目录:
   -input "C:\Users\zhangzijiang\Desktop\my_tools\test\pos_data" -output "C:\Users\zhangzijiang\Desktop\result"
`

	ctx.ShowTerminal(t.Name(), usage, func(runCtx context.Context, args string, out io.Writer) error {
		parsedArgs := framework.ParseArgs(args)

		fs := flag.NewFlagSet("pos2gis", flag.ContinueOnError)
		fs.SetOutput(out)

		var inputDir string
		var outputDir string

		fs.StringVar(&inputDir, "input", "", "")
		fs.StringVar(&outputDir, "output", "", "")

		if err := fs.Parse(parsedArgs); err != nil {
			return err
		}

		if inputDir == "" {
			return fmt.Errorf("错误：必须指定 -input 参数")
		}

		if outputDir == "" {
			outputDir = filepath.Join(inputDir, "output")
		}

		return runConvert(inputDir, outputDir, out)
	})
}

func init() {
	framework.Register(&POSTool{})
}
