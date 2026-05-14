package utm_geojson

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
	"time"

	"my_tools/pkg/framework"

	"github.com/im7mortal/UTM"
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

		target := filepath.Join(destDir, header.Name)

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

		outPath := filepath.Join(destDir, header.Name)
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

func convertUTMToGeoJSON(utmFile, trackID string, zone int) ([]GeoJSONFeature, error) {
	file, err := os.Open(utmFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var features []GeoJSONFeature
	scanner := bufio.NewScanner(file)
	idx := 0

	for scanner.Scan() {
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

func convertSingleFile(utmPath, trackID string, zone int, out io.Writer) error {
	features, err := convertUTMToGeoJSON(utmPath, trackID, zone)
	if err != nil {
		return err
	}
	if len(features) == 0 {
		return fmt.Errorf("无有效点")
	}

	ext := filepath.Ext(utmPath)
	base := strings.TrimSuffix(utmPath, ext)
	outFile := base + ".geojson"
	if err := saveGeoJSON(features, outFile); err != nil {
		return err
	}
	fmt.Fprintf(out, "转换成功: %s (%d 点)\n", outFile, len(features))
	return nil
}

func convertDirectory(dirPath string, zone int, out io.Writer) error {
	var converted int
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		name := strings.ToLower(info.Name())
		if name == "utm.txt" || strings.HasSuffix(name, ".utm.txt") {
			var trackID string
			if name == "utm.txt" {
				trackID = filepath.Base(filepath.Dir(path))
			} else {
				trackID = strings.TrimSuffix(name, ".utm.txt")
			}
			fmt.Fprintf(out, "转换: %s (track_id: %s)\n", path, trackID)
			if err := convertSingleFile(path, trackID, zone, out); err != nil {
				fmt.Fprintf(out, "  错误: %v\n", err)
				return nil
			}
			converted++
		}
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "\n完成！共转换 %d 个文件\n", converted)
	return nil
}

func mergeGeoJSON(dirPath string, out io.Writer) error {
	var allFeatures []GeoJSONFeature
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(path), ".geojson") {
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
			fmt.Fprintf(out, "合并: %s (%d 个特征点)\n", path, len(fc.Features))
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(allFeatures) == 0 {
		return fmt.Errorf("未找到任何 GeoJSON 文件")
	}
	outFile := fmt.Sprintf("merged_%d.geojson", time.Now().UnixMilli())
	if err := saveGeoJSON(allFeatures, outFile); err != nil {
		return err
	}
	fmt.Fprintf(out, "\n合并完成！输出: %s (总共 %d 个特征点)\n", outFile, len(allFeatures))
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

func runBatchMode(ctx context.Context, inputDir string, zone int, fullExtract bool, workers int, cleanupLevel int, out io.Writer) error {
	if workers <= 0 {
		workers = 1
	}
	if _, err := os.Stat(inputDir); os.IsNotExist(err) {
		return fmt.Errorf("输入目录不存在: %s", inputDir)
	}
	timestamp := time.Now().UnixMilli()
	outputDir := fmt.Sprintf("output_%d", timestamp)
	os.MkdirAll(outputDir, 0755)

	utmBackupDir := filepath.Join(outputDir, "utm")
	os.MkdirAll(utmBackupDir, 0755)

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

			utmFile := findUTMFile(folderPath)
			var extractedFile string

			if utmFile == "" {
				tarPath := filepath.Join(folderPath, "process_result_0.tar.gz")
				if _, err := os.Stat(tarPath); err == nil {
					if fullExtract {
						fmt.Fprintf(out, "  [完全解压] %s ...\n", filepath.Base(tarPath))
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

			features, err := convertUTMToGeoJSON(utmFile, trackID, zone)
			if err != nil {
				fmt.Fprintf(out, "  [错误] %s: %v\n", trackID, err)
				cleanExtractedFile(extractedFile, folderPath, out)
				return
			}
			if len(features) == 0 {
				fmt.Fprintf(out, "  [警告] %s: 无有效点\n", trackID)
				cleanExtractedFile(extractedFile, folderPath, out)
				return
			}

			outFile := filepath.Join(outputDir, trackID+".geojson")
			if err := saveGeoJSON(features, outFile); err != nil {
				fmt.Fprintf(out, "  [错误] 保存 geojson 失败 %s: %v\n", trackID, err)
				cleanExtractedFile(extractedFile, folderPath, out)
				return
			}

			dstUtm := filepath.Join(utmBackupDir, trackID+".utm.txt")
			if err := copyFile(utmFile, dstUtm); err != nil {
				fmt.Fprintf(out, "  [警告] 备份 utm 失败 %s: %v\n", trackID, err)
			}

			fmt.Fprintf(out, "  [OK] %s.geojson (%d 点)\n", trackID, len(features))

			cleanExtractedFile(extractedFile, folderPath, out)

			mu.Lock()
			allFeatures = append(allFeatures, features...)
			processedFolders = append(processedFolders, folderPath)
			mu.Unlock()
		}(folder)
	}

	wg.Wait()

	if len(allFeatures) > 0 {
		mergeFile := filepath.Join(outputDir, "merged_tracks.geojson")
		saveGeoJSON(allFeatures, mergeFile)
		fmt.Fprintf(out, "\n  [合并] merged_tracks.geojson (%d 点)\n", len(allFeatures))
	}

	fmt.Fprintf(out, "\n处理完成！共处理 %d 个文件夹\n", len(processedFolders))
	fmt.Fprintf(out, "输出目录: %s\n", outputDir)
	fmt.Fprintf(out, "UTM 备份目录: %s\n", utmBackupDir)

	if fullExtract {
		if cleanupLevel == 2 {
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
		} else if cleanupLevel == 3 {
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
func (t *UTMTool) Name() string     { return "点云UTM转GeoJSON工具" }
func (t *UTMTool) Category() string { return "数据处理" }

func (t *UTMTool) Execute(ctx framework.AppContext) {
	usage := `[yellow]线下点云资料编译输出压缩文件解包与 UTM-GeoJSON 转换工具[-]

[cyan]说明:[-]
本工具用于将无人车/点云编译系统输出的 UTM 坐标文本转换为标准的 GeoJSON 格式，便于在 GIS 软件中查看。

[cyan]使用方法 (注意：不需要输入 .exe，直接输入参数即可):[-]
在下方输入框中直接输入你想要的参数组合，然后按下 Enter 键执行。

[cyan]参数详解:[-]
  -input <目录路径>          [批量模式] 指定 out_source 目录路径。
                             程序会遍历该目录下的子文件夹进行解压和转换。
  -convert <文件/目录路径>   [单独模式] 单独转换一个 utm.txt，或包含该文件的目录。
  -merge <目录路径>          [合并模式] 将指定目录下所有的 .geojson 文件合并成一个。
  -zone <int>                指定 UTM Zone 编号。默认: 50 (适用于东经 114°-120°)。
  -workers <int>             [批量模式] 并发处理的工作线程数。默认: 4。
  -full-extract              [批量模式] 是否完全解压 process_result_0.tar.gz。
                             如果不带此参数，默认只提取 utm.txt，速度更快且用后即清。
  -cleanup <int>             [批量模式] 清理级别 (仅完全解压模式有效):
                               0 = 不清理
                               2 = 删除解压出的文件
                               3 = 删除原始压缩包

[cyan]实际运行示例 (可以直接复制到下方输入):[-]

1. 批量处理一个目录 (常用，注意路径中有空格请用引号包裹):
   -input "C:\Users\zhangzijiang\Desktop\my_tools\test\out_source" -workers 4

2. 单独转换某一个文件:
   -convert C:\Users\zhangzijiang\Desktop\my_tools\test\utm.txt -zone 50

3. 将分散的 GeoJSON 文件合并:
   -merge C:\Users\zhangzijiang\Desktop\my_tools\test\output
`

	ctx.ShowTerminal(t.Name(), usage, func(runCtx context.Context, args string, out io.Writer) error {
		// Parse args manually using framework parser
		parsedArgs := framework.ParseArgs(args)

		fs := flag.NewFlagSet("utm_tool", flag.ContinueOnError)
		fs.SetOutput(out)

		var inputDir string
		var zone int
		var convertPath string
		var mergeDir string
		var fullExtract bool
		var workers int
		var cleanupLevel int

		fs.StringVar(&inputDir, "input", "", "")
		fs.IntVar(&zone, "zone", 50, "")
		fs.StringVar(&convertPath, "convert", "", "")
		fs.StringVar(&mergeDir, "merge", "", "")
		fs.BoolVar(&fullExtract, "full-extract", false, "")
		fs.IntVar(&workers, "workers", 4, "")
		fs.IntVar(&cleanupLevel, "cleanup", 0, "")

		if err := fs.Parse(parsedArgs); err != nil {
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

		if inputDir != "" {
			return runBatchMode(runCtx, inputDir, zone, fullExtract, workers, cleanupLevel, out)
		}
		if convertPath != "" {
			info, err := os.Stat(convertPath)
			if err != nil {
				return fmt.Errorf("路径不存在: %s", convertPath)
			}
			if !info.IsDir() && strings.HasSuffix(strings.ToLower(convertPath), ".txt") {
				trackID := strings.TrimSuffix(filepath.Base(convertPath), filepath.Ext(convertPath))
				return convertSingleFile(convertPath, trackID, zone, out)
			}
			return convertDirectory(convertPath, zone, out)
		}
		if mergeDir != "" {
			return mergeGeoJSON(mergeDir, out)
		}
		return nil
	})
}

func init() {
	framework.Register(&UTMTool{})
}
