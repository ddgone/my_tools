package geojson_to_shapefile

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"my_tools/libs/framework"

	"github.com/jonas-p/go-shp"
)

type GeoJSONGeometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

type GeoJSONFeature struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Geometry   GeoJSONGeometry        `json:"geometry"`
}

type GeoJSONFC struct {
	Type     string           `json:"type"`
	Features []GeoJSONFeature `json:"features"`
}

type SHPFeature struct {
	Properties map[string]string
	X, Y       float64
	LineParts  [][]shp.Point
}

var wgs84WKT = `GEOGCS["WGS 84",DATUM["WGS_1984",SPHEROID["WGS 84",6378137,298.257223563,AUTHORITY["EPSG","7030"]],AUTHORITY["EPSG","6326"]],PRIMEM["Greenwich",0,AUTHORITY["EPSG","8901"]],UNIT["degree",0.0174532925199433,AUTHORITY["EPSG","9122"]],AUTHORITY["EPSG","4326"]]`

func writeWGS84Prj(prjPath string) error {
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

func collectKeys(features []SHPFeature) []string {
	seen := map[string]bool{}
	var keys []string
	for _, f := range features {
		for k := range f.Properties {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	return keys
}

func writePointSHP(ctx context.Context, path string, features []SHPFeature) error {
	if len(features) == 0 {
		return nil
	}

	keys := collectKeys(features)
	fields := make([]shp.Field, len(keys))
	for i, k := range keys {
		fields[i] = shp.StringField(k, 254)
	}

	writer, err := shp.Create(path, shp.POINT)
	if err != nil {
		return err
	}
	_ = writer.SetFields(fields)

	for n, f := range features {
		if n%1000 == 0 {
			select {
			case <-ctx.Done():
				writer.Close()
				return fmt.Errorf("任务已被取消")
			default:
			}
		}
		writer.Write(&shp.Point{X: f.X, Y: f.Y})
		for i, k := range keys {
			_ = writer.WriteAttribute(n, i, f.Properties[k])
		}
	}

	writer.Close()
	if err := writeWGS84Prj(strings.TrimSuffix(path, ".shp") + ".prj"); err != nil {
		return err
	}
	return fixDbfPath(path)
}

func writeLineSHP(ctx context.Context, path string, features []SHPFeature) error {
	if len(features) == 0 {
		return nil
	}

	keys := collectKeys(features)
	fields := make([]shp.Field, len(keys))
	for i, k := range keys {
		fields[i] = shp.StringField(k, 254)
	}

	writer, err := shp.Create(path, shp.POLYLINE)
	if err != nil {
		return err
	}
	_ = writer.SetFields(fields)

	n := 0
	for _, f := range features {
		if n%1000 == 0 {
			select {
			case <-ctx.Done():
				writer.Close()
				return fmt.Errorf("任务已被取消")
			default:
			}
		}
		if len(f.LineParts) == 0 || len(f.LineParts[0]) < 2 {
			continue
		}
		line := shp.NewPolyLine(f.LineParts)
		writer.Write(line)
		for i, k := range keys {
			_ = writer.WriteAttribute(n, i, f.Properties[k])
		}
		n++
	}

	writer.Close()
	if err := writeWGS84Prj(strings.TrimSuffix(path, ".shp") + ".prj"); err != nil {
		return err
	}
	return fixDbfPath(path)
}

func parseGeoJSONFile(ctx context.Context, path string) ([]SHPFeature, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var fc GeoJSONFC
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, err
	}

	var features []SHPFeature
	sourceFile := filepath.Base(path)

	for i, f := range fc.Features {
		if i%1000 == 0 {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("任务已被取消")
			default:
			}
		}
		props := map[string]string{
			"sourceFile": sourceFile,
		}
		for k, v := range f.Properties {
			props[k] = fmt.Sprintf("%v", v)
		}

		sf := SHPFeature{Properties: props}

		switch strings.ToLower(f.Geometry.Type) {
		case "point":
			var coords []float64
			if err := json.Unmarshal(f.Geometry.Coordinates, &coords); err == nil && len(coords) >= 2 {
				sf.X = coords[0]
				sf.Y = coords[1]
				features = append(features, sf)
			}
		case "multipoint":
			var coords [][]float64
			if err := json.Unmarshal(f.Geometry.Coordinates, &coords); err == nil {
				for _, pt := range coords {
					if len(pt) >= 2 {
						cp := sf
						cp.X = pt[0]
						cp.Y = pt[1]
						features = append(features, cp)
					}
				}
			}
		case "linestring":
			var coords [][]float64
			if err := json.Unmarshal(f.Geometry.Coordinates, &coords); err == nil && len(coords) >= 2 {
				var pts []shp.Point
				for _, c := range coords {
					if len(c) >= 2 {
						pts = append(pts, shp.Point{X: c[0], Y: c[1]})
					}
				}
				if len(pts) >= 2 {
					sf.LineParts = [][]shp.Point{pts}
					features = append(features, sf)
				}
			}
		case "multilinestring":
			var coords [][][]float64
			if err := json.Unmarshal(f.Geometry.Coordinates, &coords); err == nil && len(coords) > 0 {
				var parts [][]shp.Point
				for _, line := range coords {
					var pts []shp.Point
					for _, c := range line {
						if len(c) >= 2 {
							pts = append(pts, shp.Point{X: c[0], Y: c[1]})
						}
					}
					if len(pts) >= 2 {
						parts = append(parts, pts)
					}
				}
				if len(parts) > 0 {
					sf.LineParts = parts
					features = append(features, sf)
				}
			}
		}
	}
	return features, nil
}

func runConvert(ctx context.Context, inputPath, outputDir string, out io.Writer) error {
	info, err := os.Stat(inputPath)
	if err != nil {
		return fmt.Errorf("路径不存在: %s", inputPath)
	}

	var geojsonFiles []string
	if info.IsDir() {
		entries, err := os.ReadDir(inputPath)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".geojson") {
				geojsonFiles = append(geojsonFiles, filepath.Join(inputPath, e.Name()))
			}
		}
	} else {
		geojsonFiles = append(geojsonFiles, inputPath)
	}

	if len(geojsonFiles) == 0 {
		return fmt.Errorf("未找到任何 GeoJSON 文件")
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %v", err)
	}

	var pointFeatures []SHPFeature
	var lineFeatures []SHPFeature

	for _, path := range geojsonFiles {
		// 检查是否收到取消信号
		select {
		case <-ctx.Done():
			return fmt.Errorf("任务已被取消")
		default:
		}

		fmt.Fprintf(out, "解析: %s\n", path)
		features, err := parseGeoJSONFile(ctx, path)
		if err != nil {
			if err.Error() == "任务已被取消" {
				return err
			}
			fmt.Fprintf(out, "  错误: %v\n", err)
			continue
		}
		var pointCount, lineCount int
		for _, f := range features {
			if len(f.LineParts) > 0 {
				lineFeatures = append(lineFeatures, f)
				lineCount++
			} else {
				pointFeatures = append(pointFeatures, f)
				pointCount++
			}
		}
		fmt.Fprintf(out, "  点: %d, 线: %d\n", pointCount, lineCount)
	}

	fmt.Fprintf(out, "\n总计: %d 个点, %d 条线\n", len(pointFeatures), len(lineFeatures))

	if len(pointFeatures) > 0 {
		// 检查是否收到取消信号
		select {
		case <-ctx.Done():
			return fmt.Errorf("任务已被取消")
		default:
		}
		pointPath := filepath.Join(outputDir, "merged_points.shp")
		if err := writePointSHP(ctx, pointPath, pointFeatures); err != nil {
			return err
		}
		fmt.Fprintf(out, "点 Shapefile 输出: %s\n", pointPath)
	}

	if len(lineFeatures) > 0 {
		// 检查是否收到取消信号
		select {
		case <-ctx.Done():
			return fmt.Errorf("任务已被取消")
		default:
		}
		linePath := filepath.Join(outputDir, "merged_lines.shp")
		if err := writeLineSHP(ctx, linePath, lineFeatures); err != nil {
			return err
		}
		fmt.Fprintf(out, "线 Shapefile 输出: %s\n", linePath)
	}

	fmt.Fprintf(out, "\n转换完成！\n")
	return nil
}

type G2STool struct{}

func (t *G2STool) ID() string       { return "geojson_to_shp" }
func (t *G2STool) Name() string     { return "GeoJSON转Shapefile工具" }
func (t *G2STool) Category() string { return "KD测试工具 > 点云处理工具" }

func (t *G2STool) Execute(ctx framework.AppContext) {
	usage := `[yellow]GeoJSON 转 Shapefile 工具[-]

[cyan]说明:[-]
本工具用于将 GeoJSON 文件（点/线/面）转换为 Shapefile 格式，支持单个文件或整个目录。

[cyan]支持的几何类型:[-]
  Point, MultiPoint → 点 Shapefile
  LineString, MultiLineString → 线 Shapefile

[cyan]参数详解:[-]
  -input <文件/目录路径>   指定 GeoJSON 文件或包含 .geojson 文件的目录。
  -output <目录路径>       指定输出目录。默认在输入目录下创建 output 子文件夹。

[cyan]实际运行示例 (可以直接复制到下方输入):[-]

1. 转换单个文件:
   -input "<你的.geojson文件路径>"

2. 转换整个目录:
   -input "<你的geojson文件目录>" -output "<输出目录>"
`

	ctx.ShowTerminal(t.Name(), usage, func(runCtx context.Context, args string, out io.Writer) error {
		parsedArgs, err := framework.ParseArgs(args)
		if err != nil {
			return err
		}

		fs := flag.NewFlagSet("geojson2shp", flag.ContinueOnError)
		fs.SetOutput(out)

		var inputPath string
		var outputDir string

		fs.StringVar(&inputPath, "input", "", "")
		fs.StringVar(&outputDir, "output", "", "")

		if err := fs.Parse(parsedArgs); err != nil {
			return err
		}

		if inputPath == "" {
			return fmt.Errorf("错误：必须指定 -input 参数")
		}

		if outputDir == "" {
			info, err := os.Stat(inputPath)
			if err != nil {
				return err
			}
			if info.IsDir() {
				outputDir = filepath.Join(inputPath, "output")
			} else {
				outputDir = filepath.Join(filepath.Dir(inputPath), "output")
			}
		}

		return runConvert(runCtx, inputPath, outputDir, out)
	})
}

func init() {
	framework.Register(&G2STool{})
}
