package bxn_point_cloud_renew

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	trajUTMFile      = "utm.txt"
	trajOptiPoseFile = "opti_pose_enu.txt"
)

// trajLine 带时间戳的轨迹行。
type trajLine struct {
	ts   string
	line string
}

// trajPoint 带 UTM 坐标的轨迹点（用于 utm.txt 的框内判断）。
type trajPoint struct {
	trajLine
	easting  float64
	northing float64
	lon      float64
	lat      float64
}

// frameMatchResult 框内匹配结果。
type frameMatchResult struct {
	keepTS     map[string]bool // 保留的时间戳（框外全部 + 框内交集）
	replaceTS  map[string]bool // 需要替换（框内交集）的时间戳
	replacedTS []string        // 被替换的时间戳（完整列表，便于日志）
	deletedTS  []string        // 删除的时间戳（框内老包独有）
	keptTS     []string        // 框外保留的时间戳（完整列表，便于日志）
	skippedTS  []string        // 新包独有（跳过）
	stat       trajStat
}

// framesZone 从框的顶点经度推算 UTM 带号。
func framesZone(frames []taskFrame) int {
	for _, f := range frames {
		if len(f.polygon) > 0 && len(f.polygon[0]) > 0 {
			return lonToZone(f.polygon[0][0])
		}
	}
	return 50
}

// readUTMFile 读取 utm.txt，解析时间戳和 UTM 坐标，并转为经纬度。
func readUTMFile(path string, zone int) ([]trajPoint, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var points []trajPoint
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		text := scanner.Text()
		if text == "" {
			continue
		}
		fields := strings.Fields(text)
		if len(fields) < 3 {
			continue
		}
		easting, err1 := strconv.ParseFloat(fields[1], 64)
		northing, err2 := strconv.ParseFloat(fields[2], 64)
		if err1 != nil || err2 != nil {
			continue
		}
		lat, lon, err := utmToLatLon(easting, northing, zone)
		if err != nil {
			lat, lon = 0, 0
		}
		points = append(points, trajPoint{
			trajLine: trajLine{ts: fields[0], line: text},
			easting:  easting,
			northing: northing,
			lon:      lon,
			lat:      lat,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return points, nil
}

// matchByFrames 用框内匹配决定老包每个轨迹点的去留。
func matchByFrames(oldUTM, newUTM []trajPoint, frames []taskFrame) frameMatchResult {
	res := frameMatchResult{
		keepTS:    make(map[string]bool),
		replaceTS: make(map[string]bool),
	}
	res.stat.OldLines = len(oldUTM)
	res.stat.NewLines = len(newUTM)

	// 经纬度在 readUTMFile 时已转换好

	// 找出新包压盖的框
	var activeFrames []taskFrame
	for _, f := range frames {
		for _, p := range newUTM {
			if pointInPolygon(p.lon, p.lat, f.polygon) {
				activeFrames = append(activeFrames, f)
				break
			}
		}
	}

	// 新包时间戳集合
	newByTS := make(map[string]bool)
	for _, p := range newUTM {
		newByTS[p.ts] = true
	}

	// 对老包每个点决定去留
	for _, p := range oldUTM {
		inFrame := pointInAnyFrame(p.lon, p.lat, activeFrames)
		if inFrame {
			if newByTS[p.ts] {
				res.keepTS[p.ts] = true
				res.replaceTS[p.ts] = true
				res.replacedTS = append(res.replacedTS, p.ts)
				res.stat.Replaced++
			} else {
				res.deletedTS = append(res.deletedTS, p.ts)
				res.stat.Deleted++
			}
		} else {
			res.keepTS[p.ts] = true
			res.keptTS = append(res.keptTS, p.ts)
		}
	}

	// 新包独有点（跳过）
	oldTS := make(map[string]bool)
	for _, p := range oldUTM {
		oldTS[p.ts] = true
	}
	for _, p := range newUTM {
		if !oldTS[p.ts] {
			res.skippedTS = append(res.skippedTS, p.ts)
			res.stat.Skipped++
		}
	}

	return res
}

// writeFilteredTrajectory 根据保留/替换映射，写回轨迹文件。
func writeFilteredTrajectory(path string, oldLines []trajLine, newByTS map[string]string, res frameMatchResult) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, l := range oldLines {
		if !res.keepTS[l.ts] {
			continue
		}
		if res.replaceTS[l.ts] {
			if nl, ok := newByTS[l.ts]; ok {
				w.WriteString(nl + "\n")
				continue
			}
		}
		w.WriteString(l.line + "\n")
	}
	return w.Flush()
}

// replaceByFrames 完整处理 utm.txt 和 opti_pose_enu.txt 的框内匹配替换，
// 返回各文件统计和完整匹配结果（供全量日志和点云同步使用）。
func replaceByFrames(oldDir, newDir string, frames []taskFrame) (map[string]trajStat, frameMatchResult, error) {
	zone := framesZone(frames)

	oldUTMPath := filepath.Join(oldDir, trajUTMFile)
	newUTMPath := filepath.Join(newDir, trajUTMFile)
	oldOptiPath := filepath.Join(oldDir, trajOptiPoseFile)
	newOptiPath := filepath.Join(newDir, trajOptiPoseFile)

	oldUTM, err := readUTMFile(oldUTMPath, zone)
	if err != nil {
		return nil, frameMatchResult{}, fmt.Errorf("read old utm failed: %w", err)
	}
	newUTM, err := readUTMFile(newUTMPath, zone)
	if err != nil {
		return nil, frameMatchResult{}, fmt.Errorf("read new utm failed: %w", err)
	}
	oldOpti, err := readTrajectoryFile(oldOptiPath)
	if err != nil {
		return nil, frameMatchResult{}, fmt.Errorf("read old opti failed: %w", err)
	}
	newOpti, err := readTrajectoryFile(newOptiPath)
	if err != nil {
		return nil, frameMatchResult{}, fmt.Errorf("read new opti failed: %w", err)
	}

	res := matchByFrames(oldUTM, newUTM, frames)

	// 构建新包行映射
	newUTMByTS := make(map[string]string)
	for _, p := range newUTM {
		newUTMByTS[p.ts] = p.line
	}
	newOptiByTS := make(map[string]string)
	for _, p := range newOpti {
		newOptiByTS[p.ts] = p.line
	}

	// 写回 utm.txt
	var oldUTMLines []trajLine
	for _, p := range oldUTM {
		oldUTMLines = append(oldUTMLines, p.trajLine)
	}
	if err := writeFilteredTrajectory(oldUTMPath, oldUTMLines, newUTMByTS, res); err != nil {
		return nil, res, fmt.Errorf("write utm failed: %w", err)
	}

	// 写回 opti_pose_enu.txt
	if err := writeFilteredTrajectory(oldOptiPath, oldOpti, newOptiByTS, res); err != nil {
		return nil, res, fmt.Errorf("write opti failed: %w", err)
	}

	statUTM := res.stat
	statOpti := res.stat
	statOpti.OldLines = len(oldOpti)
	statOpti.NewLines = len(newOpti)

	stats := map[string]trajStat{
		trajUTMFile:      statUTM,
		trajOptiPoseFile: statOpti,
	}
	return stats, res, nil
}

// readTrajectoryFile 读取轨迹文件（首列为时间戳）。
func readTrajectoryFile(path string) ([]trajLine, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []trajLine

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		text := scanner.Text()
		if text == "" {
			continue
		}
		ts := extractTimestamp(text)
		if ts == "" {
			continue
		}
		lines = append(lines, trajLine{ts: ts, line: text})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

// extractTimestamp 提取行首的时间戳字段。
func extractTimestamp(line string) string {
	line = strings.TrimLeft(line, " \t")
	idx := strings.IndexAny(line, " \t")
	if idx < 0 {
		return line
	}
	return line[:idx]
}
