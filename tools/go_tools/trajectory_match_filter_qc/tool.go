package trajectory_match_filter_qc

import (
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	shp "github.com/jonas-p/go-shp"
)

const (
	earthRadiusMeters = 6378137.0
	defaultErrorM     = 0.02
	defaultThreads    = 4
	minKeepRun        = 2
)

type OrderBy string

const (
	OrderByRecord     OrderBy = "record"
	OrderByPointIndex OrderBy = "pointIndex"
	OrderByTimestamp  OrderBy = "timestamp"
)

var supportedOrders = []OrderBy{
	OrderByRecord,
	OrderByPointIndex,
	OrderByTimestamp,
}

type Config struct {
	ReferenceSHPs []string
	TargetSHPs    []string
	OutputDir     string
	ErrorMeters   float64
	Merge         bool
	OrderBy       OrderBy
	Threads       int
	Logger        *slog.Logger
}

type PointFeature struct {
	RecordIndex  int
	PointIndex   int
	TimestampNum float64
	X            float64
	Y            float64
	LocalX       float64
	LocalY       float64
}

type FileSummary struct {
	TargetSHP            string
	TargetStem           string
	PointCount           int
	MatchCount           int
	MatchRatio           float64
	MaxContinuousMatches int
	ErrorMeters          float64
	Keep                 bool
}

type DetailSegment struct {
	TargetSHP        string
	TargetStem       string
	Matched          bool
	StartSequence    int
	EndSequence      int
	PointCount       int
	StartPointIndex  int
	EndPointIndex    int
	StartRecordIndex int
	EndRecordIndex   int
}

type MergePointSet struct {
	SourcePath string
	TrackName  string
	SrcFile    string
	Points     []PointFeature
}

type SpatialProjector struct {
	geographic bool
	lon0       float64
	lat0       float64
	cosLat0    float64
}

type cellKey struct {
	X int64
	Y int64
}

type GridIndex struct {
	cellSize     float64
	errorSquared float64
	cells        map[cellKey][]PointFeature
}

type cliOptions struct {
	InputPath string
	BasePath  string
	OutputDir string
	ErrorM    float64
	Merge     bool
	OrderBy   OrderBy
	Threads   int
	Timeout   time.Duration
	Verbose   bool
}

func Run(ctx context.Context, args []string, out io.Writer) error {
	opts, err := parseCLIArgs(args, out)
	if err != nil {
		return err
	}

	logger := newLogger(out, opts.Verbose)
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	referenceSHPs, err := resolveSHPInputs(opts.BasePath)
	if err != nil {
		logger.Error("解析 base 失败", slog.Any("err", err))
		return err
	}
	targetSHPs, err := resolveSHPInputs(opts.InputPath)
	if err != nil {
		logger.Error("解析 input 失败", slog.Any("err", err))
		return err
	}
	targetSHPs = excludePaths(targetSHPs, referenceSHPs)
	if len(targetSHPs) == 0 {
		err = fmt.Errorf("排除基准文件后没有剩余待测轨迹")
		logger.Error("解析 input 失败", slog.Any("err", err))
		return err
	}

	cfg := Config{
		ReferenceSHPs: referenceSHPs,
		TargetSHPs:    targetSHPs,
		OutputDir:     normalizePath(opts.OutputDir),
		ErrorMeters:   opts.ErrorM,
		Merge:         opts.Merge,
		OrderBy:       opts.OrderBy,
		Threads:       opts.Threads,
		Logger:        logger,
	}

	startedAt := time.Now()
	if err := runWithConfig(ctx, cfg); err != nil {
		logger.Error("执行失败", slog.Any("err", err))
		return err
	}

	logger.Info("执行完成",
		slog.Duration("elapsed", time.Since(startedAt)),
		slog.String("output_dir", cfg.OutputDir),
	)
	return nil
}

func parseCLIArgs(args []string, out io.Writer) (cliOptions, error) {
	fs := flag.NewFlagSet("trajectory_match_filter_qc", flag.ContinueOnError)
	fs.SetOutput(out)

	var opts cliOptions
	opts.OutputDir = "output"
	opts.ErrorM = defaultErrorM
	opts.Merge = true
	opts.OrderBy = OrderByPointIndex
	opts.Threads = defaultThreads

	fs.StringVar(&opts.InputPath, "input", "", "待测轨迹点 shp 文件或目录")
	fs.StringVar(&opts.BasePath, "base", "", "基准轨迹点 shp 文件或目录")
	fs.StringVar(&opts.OutputDir, "output", opts.OutputDir, "结果输出目录")
	fs.Float64Var(&opts.ErrorM, "error", opts.ErrorM, "空间误差阈值，单位米，默认 0.02")
	fs.BoolVar(&opts.Merge, "merge", opts.Merge, "是否输出保留/丢弃汇总 point shp，默认 true")
	fs.BoolVar(&opts.Merge, "mearge", opts.Merge, "兼容旧拼写：是否输出保留/丢弃汇总 point shp")
	fs.Func("order-by", "目标点排序方式: record|pointIndex|timestamp", func(value string) error {
		opts.OrderBy = OrderBy(value)
		return nil
	})
	fs.IntVar(&opts.Threads, "threads", opts.Threads, "并发处理待测轨迹的线程数")
	fs.DurationVar(&opts.Timeout, "timeout", 0, "整体执行超时，例如 2m；0 表示不限制")
	fs.BoolVar(&opts.Verbose, "verbose", false, "输出 debug 日志")

	if err := fs.Parse(args); err != nil {
		return cliOptions{}, err
	}
	return opts, nil
}

func (c Config) WithDefaults() Config {
	if c.OrderBy == "" {
		c.OrderBy = OrderByPointIndex
	}
	if c.ErrorMeters <= 0 {
		c.ErrorMeters = defaultErrorM
	}
	if c.Threads <= 0 {
		c.Threads = defaultThreads
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	return c
}

func (c Config) Validate() error {
	var errs []error
	if len(c.ReferenceSHPs) == 0 {
		errs = append(errs, errors.New("base 不能为空"))
	}
	if len(c.TargetSHPs) == 0 {
		errs = append(errs, errors.New("input 不能为空"))
	}
	if c.OutputDir == "" {
		errs = append(errs, errors.New("output 不能为空"))
	}
	if c.ErrorMeters <= 0 {
		errs = append(errs, errors.New("error 必须大于 0"))
	}
	if c.Threads <= 0 {
		errs = append(errs, errors.New("threads 必须大于 0"))
	}
	if !slices.Contains(supportedOrders, c.OrderBy) {
		errs = append(errs, fmt.Errorf("不支持的排序方式: %s", c.OrderBy))
	}
	return errors.Join(errs...)
}

func runWithConfig(ctx context.Context, cfg Config) error {
	cfg = cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return err
	}

	logger := cfg.Logger.With(
		slog.Int("base_count", len(cfg.ReferenceSHPs)),
		slog.Int("target_count", len(cfg.TargetSHPs)),
		slog.Int("threads", cfg.Threads),
		slog.Float64("error_m", cfg.ErrorMeters),
	)
	logger.InfoContext(ctx, "开始加载基准轨迹")

	basePoints := make([]PointFeature, 0, 4096)
	for _, basePath := range cfg.ReferenceSHPs {
		points, err := ReadPointShapefile(basePath, OrderByRecord)
		if err != nil {
			return err
		}
		basePoints = append(basePoints, points...)
	}
	if len(basePoints) == 0 {
		return fmt.Errorf("基准轨迹为空")
	}

	projector := NewSpatialProjector(basePoints, isGeographicCoordinateSystem(cfg.ReferenceSHPs[0], basePoints))
	for i := range basePoints {
		basePoints[i].LocalX, basePoints[i].LocalY = projector.Project(basePoints[i].X, basePoints[i].Y)
	}

	grid := NewGridIndex(basePoints, cfg.ErrorMeters)
	logger.InfoContext(ctx, "基准网格索引构建完成",
		slog.Int("reference_points", len(basePoints)),
		slog.Int("grid_cells", len(grid.cells)),
	)

	threadCount := cfg.Threads
	if threadCount > len(cfg.TargetSHPs) {
		threadCount = len(cfg.TargetSHPs)
	}
	if threadCount <= 0 {
		threadCount = 1
	}

	type fileResult struct {
		index    int
		summary  FileSummary
		segments []DetailSegment
		mergeSet MergePointSet
		err      error
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan int)
	results := make(chan fileResult, len(cfg.TargetSHPs))
	var workerWG sync.WaitGroup

	for workerID := 0; workerID < threadCount; workerID++ {
		workerWG.Add(1)
		go func(workerID int) {
			defer workerWG.Done()
			for jobIndex := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				targetPath := cfg.TargetSHPs[jobIndex]
				logger.DebugContext(ctx, "开始处理目标 shp",
					slog.Int("thread_id", workerID),
					slog.String("target_shp", normalizePath(targetPath)),
				)
				summary, segments, mergeSet, err := processTargetFile(ctx, grid, projector, targetPath, cfg)
				results <- fileResult{
					index:    jobIndex,
					summary:  summary,
					segments: segments,
					mergeSet: mergeSet,
					err:      err,
				}
				if err != nil {
					cancel()
					return
				}
			}
		}(workerID)
	}

	go func() {
		defer close(jobs)
		for i := range cfg.TargetSHPs {
			select {
			case <-ctx.Done():
				return
			case jobs <- i:
			}
		}
	}()

	go func() {
		workerWG.Wait()
		close(results)
	}()

	summaries := make([]FileSummary, len(cfg.TargetSHPs))
	detailSegments := make([][]DetailSegment, len(cfg.TargetSHPs))
	mergeSets := make([]MergePointSet, len(cfg.TargetSHPs))
	var firstErr error
	for result := range results {
		if result.err != nil && firstErr == nil {
			firstErr = result.err
			continue
		}
		summaries[result.index] = result.summary
		detailSegments[result.index] = result.segments
		mergeSets[result.index] = result.mergeSet
		logger.DebugContext(ctx, "目标 shp 处理完成",
			slog.String("target_shp", result.summary.TargetSHP),
			slog.Int("point_count", result.summary.PointCount),
			slog.Int("match_count", result.summary.MatchCount),
			slog.Int("max_continuous_matches", result.summary.MaxContinuousMatches),
			slog.Bool("keep", result.summary.Keep),
		)
	}
	if firstErr != nil {
		return firstErr
	}

	summaryCSVPath := filepath.Join(cfg.OutputDir, "summary.csv")
	detailCSVPath := filepath.Join(cfg.OutputDir, "detail.csv")
	resTXTPath := filepath.Join(cfg.OutputDir, "res.txt")
	dropTXTPath := filepath.Join(cfg.OutputDir, "drop.txt")
	resSHPPath := filepath.Join(cfg.OutputDir, "res_merged_point.shp")
	dropSHPPath := filepath.Join(cfg.OutputDir, "drop_merged_point.shp")

	if err := WriteSummaryCSV(summaryCSVPath, summaries); err != nil {
		return err
	}
	if err := WriteDetailCSV(detailCSVPath, detailSegments); err != nil {
		return err
	}
	if err := WriteResultLists(resTXTPath, dropTXTPath, summaries); err != nil {
		return err
	}
	if cfg.Merge {
		keepSets, dropSets := splitMergeSets(summaries, mergeSets)
		if err := WriteMergedPointSHP(resSHPPath, keepSets); err != nil {
			return err
		}
		if err := WriteMergedPointSHP(dropSHPPath, dropSets); err != nil {
			return err
		}
	}
	return nil
}

func processTargetFile(ctx context.Context, grid *GridIndex, projector *SpatialProjector, targetPath string, cfg Config) (FileSummary, []DetailSegment, MergePointSet, error) {
	select {
	case <-ctx.Done():
		return FileSummary{}, nil, MergePointSet{}, ctx.Err()
	default:
	}

	targets, err := ReadPointShapefile(targetPath, cfg.OrderBy)
	if err != nil {
		return FileSummary{}, nil, MergePointSet{}, err
	}
	for i := range targets {
		targets[i].LocalX, targets[i].LocalY = projector.Project(targets[i].X, targets[i].Y)
	}

	matches := make([]bool, len(targets))
	matchCount := 0
	maxContinuous := 0
	currentContinuous := 0
	for i := range targets {
		matched := grid.Match(targets[i].LocalX, targets[i].LocalY)
		matches[i] = matched
		if matched {
			matchCount++
			currentContinuous++
			if currentContinuous > maxContinuous {
				maxContinuous = currentContinuous
			}
		} else {
			currentContinuous = 0
		}
	}

	summary := FileSummary{
		TargetSHP:            normalizePath(targetPath),
		TargetStem:           TrackStemFromPath(targetPath),
		PointCount:           len(targets),
		MatchCount:           matchCount,
		MaxContinuousMatches: maxContinuous,
		ErrorMeters:          cfg.ErrorMeters,
		Keep:                 maxContinuous >= minKeepRun,
	}
	if len(targets) > 0 {
		summary.MatchRatio = float64(matchCount) / float64(len(targets))
	}
	segments := BuildDetailSegments(summary, targets, matches)
	mergeSet := MergePointSet{}
	if cfg.Merge {
		mergeSet = MergePointSet{
			SourcePath: summary.TargetSHP,
			TrackName:  summary.TargetStem,
			SrcFile:    filepath.Base(summary.TargetSHP),
			Points:     targets,
		}
	}
	return summary, segments, mergeSet, nil
}

func NewGridIndex(points []PointFeature, errorMeters float64) *GridIndex {
	cellSize := errorMeters
	if cellSize <= 0 {
		cellSize = defaultErrorM
	}

	index := &GridIndex{
		cellSize:     cellSize,
		errorSquared: errorMeters * errorMeters,
		cells:        make(map[cellKey][]PointFeature, len(points)),
	}
	for _, point := range points {
		key := index.keyFor(point.LocalX, point.LocalY)
		index.cells[key] = append(index.cells[key], point)
	}
	return index
}

func (g *GridIndex) keyFor(x, y float64) cellKey {
	return cellKey{
		X: int64(math.Floor(x / g.cellSize)),
		Y: int64(math.Floor(y / g.cellSize)),
	}
}

func (g *GridIndex) Match(x, y float64) bool {
	center := g.keyFor(x, y)
	for dy := int64(-1); dy <= 1; dy++ {
		for dx := int64(-1); dx <= 1; dx++ {
			key := cellKey{X: center.X + dx, Y: center.Y + dy}
			candidates := g.cells[key]
			for _, point := range candidates {
				dx := point.LocalX - x
				dy := point.LocalY - y
				if dx*dx+dy*dy <= g.errorSquared {
					return true
				}
			}
		}
	}
	return false
}

func NewSpatialProjector(points []PointFeature, geographic bool) *SpatialProjector {
	projector := &SpatialProjector{geographic: geographic}
	if !geographic || len(points) == 0 {
		return projector
	}

	var sumX, sumY float64
	for _, point := range points {
		sumX += point.X
		sumY += point.Y
	}
	projector.lon0 = sumX / float64(len(points))
	projector.lat0 = sumY / float64(len(points))
	projector.cosLat0 = math.Cos(degreesToRadians(projector.lat0))
	return projector
}

func (p *SpatialProjector) Project(x, y float64) (float64, float64) {
	if p == nil || !p.geographic {
		return x, y
	}
	localX := earthRadiusMeters * degreesToRadians(x-p.lon0) * p.cosLat0
	localY := earthRadiusMeters * degreesToRadians(y-p.lat0)
	return localX, localY
}

func BuildDetailSegments(summary FileSummary, targets []PointFeature, matches []bool) []DetailSegment {
	if len(targets) == 0 {
		return nil
	}

	segments := make([]DetailSegment, 0, 8)
	start := 0
	currentMatch := matches[0]
	for i := 1; i < len(matches); i++ {
		if matches[i] == currentMatch {
			continue
		}
		segments = append(segments, buildSegment(summary, targets, currentMatch, start, i-1))
		start = i
		currentMatch = matches[i]
	}
	segments = append(segments, buildSegment(summary, targets, currentMatch, start, len(matches)-1))
	return segments
}

func buildSegment(summary FileSummary, targets []PointFeature, matched bool, start, end int) DetailSegment {
	startPoint := targets[start]
	endPoint := targets[end]
	return DetailSegment{
		TargetSHP:        summary.TargetSHP,
		TargetStem:       summary.TargetStem,
		Matched:          matched,
		StartSequence:    start + 1,
		EndSequence:      end + 1,
		PointCount:       end - start + 1,
		StartPointIndex:  startPoint.PointIndex,
		EndPointIndex:    endPoint.PointIndex,
		StartRecordIndex: startPoint.RecordIndex,
		EndRecordIndex:   endPoint.RecordIndex,
	}
}

func ReadPointShapefile(path string, orderBy OrderBy) ([]PointFeature, error) {
	reader, err := shp.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开 shp 失败 %s: %w", path, err)
	}
	defer reader.Close()

	fields := reader.Fields()
	dataFields := fields
	if len(dataFields) > 0 && strings.EqualFold(strings.TrimSpace(dataFields[0].String()), "DeletionFlag") {
		dataFields = dataFields[1:]
	}

	fieldIndexes := make(map[string]int, len(dataFields))
	for idx, field := range dataFields {
		fieldIndexes[strings.ToLower(strings.TrimSpace(field.String()))] = idx
	}

	pointIndexIdx, hasPointIndex := fieldIndexes["pointindex"]
	timestampIdx, hasTimestamp := fieldIndexes["timestamp"]
	needPointIndex := orderBy == OrderByPointIndex
	needTimestamp := orderBy == OrderByTimestamp

	features := make([]PointFeature, 0, 1024)
	for reader.Next() {
		recordIndex, shape := reader.Shape()
		point, ok := shape.(*shp.Point)
		if !ok {
			return nil, fmt.Errorf("文件 %s 包含非点要素: %T", path, shape)
		}

		feature := PointFeature{
			RecordIndex: recordIndex,
			PointIndex:  -1,
			X:           point.X,
			Y:           point.Y,
		}
		if needPointIndex && hasPointIndex {
			rawPointIndex := cleanDBFString(reader.ReadAttribute(recordIndex, pointIndexIdx))
			feature.PointIndex = parseFlexibleInt(rawPointIndex, -1)
		}
		if needTimestamp && hasTimestamp {
			rawTimestamp := cleanDBFString(reader.ReadAttribute(recordIndex, timestampIdx))
			feature.TimestampNum = parseFlexibleFloat(rawTimestamp)
		}
		features = append(features, feature)
	}
	if err := reader.Err(); err != nil {
		return nil, fmt.Errorf("读取 shp 失败 %s: %w", path, err)
	}

	sortFeatures(features, orderBy)
	return features, nil
}

func sortFeatures(features []PointFeature, orderBy OrderBy) {
	switch orderBy {
	case OrderByRecord:
		return
	case OrderByPointIndex:
		sort.SliceStable(features, func(i, j int) bool {
			left, right := features[i], features[j]
			switch {
			case left.PointIndex >= 0 && right.PointIndex >= 0 && left.PointIndex != right.PointIndex:
				return left.PointIndex < right.PointIndex
			case left.PointIndex >= 0 && right.PointIndex < 0:
				return true
			case left.PointIndex < 0 && right.PointIndex >= 0:
				return false
			default:
				return left.RecordIndex < right.RecordIndex
			}
		})
	case OrderByTimestamp:
		sort.SliceStable(features, func(i, j int) bool {
			left, right := features[i], features[j]
			switch {
			case left.TimestampNum != 0 && right.TimestampNum != 0 && left.TimestampNum != right.TimestampNum:
				return left.TimestampNum < right.TimestampNum
			case left.TimestampNum != 0 && right.TimestampNum == 0:
				return true
			case left.TimestampNum == 0 && right.TimestampNum != 0:
				return false
			default:
				return left.RecordIndex < right.RecordIndex
			}
		})
	default:
		return
	}
}

func WriteSummaryCSV(path string, summaries []FileSummary) error {
	file, err := createOutputFile(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{
		"轨迹文件",
		"轨迹名称",
		"点数量",
		"匹配点数",
		"匹配比例",
		"最长连续匹配点数",
		"误差阈值(米)",
		"最终结果",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("写入 summary csv 表头失败: %w", err)
	}

	for _, summary := range summaries {
		record := []string{
			summary.TargetSHP,
			summary.TargetStem,
			strconv.Itoa(summary.PointCount),
			strconv.Itoa(summary.MatchCount),
			formatFloat(summary.MatchRatio),
			strconv.Itoa(summary.MaxContinuousMatches),
			formatFloat(summary.ErrorMeters),
			decisionLabel(summary.Keep),
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("写入 summary csv 记录失败: %w", err)
		}
	}
	if err := writer.Error(); err != nil {
		return fmt.Errorf("刷新 summary csv 失败: %w", err)
	}
	return nil
}

func WriteDetailCSV(path string, segmentsByTarget [][]DetailSegment) error {
	file, err := createOutputFile(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{
		"轨迹文件",
		"轨迹名称",
		"是否匹配",
		"起始序号",
		"结束序号",
		"连续点数",
		"起始点索引",
		"结束点索引",
		"起始记录号",
		"结束记录号",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("写入 detail csv 表头失败: %w", err)
	}

	for _, segments := range segmentsByTarget {
		for _, segment := range segments {
			record := []string{
				segment.TargetSHP,
				segment.TargetStem,
				matchStatusLabel(segment.Matched),
				strconv.Itoa(segment.StartSequence),
				strconv.Itoa(segment.EndSequence),
				strconv.Itoa(segment.PointCount),
				formatInt(segment.StartPointIndex),
				formatInt(segment.EndPointIndex),
				strconv.Itoa(segment.StartRecordIndex),
				strconv.Itoa(segment.EndRecordIndex),
			}
			if err := writer.Write(record); err != nil {
				return fmt.Errorf("写入 detail csv 记录失败: %w", err)
			}
		}
	}
	if err := writer.Error(); err != nil {
		return fmt.Errorf("刷新 detail csv 失败: %w", err)
	}
	return nil
}

func WriteResultLists(resPath, dropPath string, summaries []FileSummary) error {
	kept := make([]string, 0, len(summaries))
	dropped := make([]string, 0, len(summaries))
	for _, summary := range summaries {
		if summary.Keep {
			kept = append(kept, summary.TargetStem)
		} else {
			dropped = append(dropped, summary.TargetStem)
		}
	}
	if err := writeLines(resPath, kept); err != nil {
		return err
	}
	if err := writeLines(dropPath, dropped); err != nil {
		return err
	}
	return nil
}

func splitMergeSets(summaries []FileSummary, mergeSets []MergePointSet) ([]MergePointSet, []MergePointSet) {
	kept := make([]MergePointSet, 0, len(summaries))
	dropped := make([]MergePointSet, 0, len(summaries))
	for index, summary := range summaries {
		mergeSet := mergeSets[index]
		if len(mergeSet.Points) == 0 {
			continue
		}
		if summary.Keep {
			kept = append(kept, mergeSet)
			continue
		}
		dropped = append(dropped, mergeSet)
	}
	return kept, dropped
}

func WriteMergedPointSHP(outputPath string, mergeSets []MergePointSet) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("创建 shp 输出目录失败 %s: %w", outputPath, err)
	}

	basePath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath))
	writer, err := shp.Create(basePath, shp.POINT)
	if err != nil {
		return fmt.Errorf("创建 shp 失败 %s: %w", outputPath, err)
	}

	fields := []shp.Field{
		shp.StringField("track_name", 80),
		shp.StringField("src_file", 100),
		shp.NumberField("point_idx", 12),
		shp.NumberField("record_idx", 12),
	}
	if err := writer.SetFields(fields); err != nil {
		return fmt.Errorf("设置 shp 字段失败 %s: %w", outputPath, err)
	}

	rowIndex := 0
	var prjSource string
	for _, mergeSet := range mergeSets {
		if prjSource == "" {
			prjSource = mergeSet.SourcePath
		}
		for _, point := range mergeSet.Points {
			writer.Write(&shp.Point{X: point.X, Y: point.Y})
			if err := writer.WriteAttribute(rowIndex, 0, mergeSet.TrackName); err != nil {
				writer.Close()
				return fmt.Errorf("写入 shp 属性失败 %s: %w", outputPath, err)
			}
			if err := writer.WriteAttribute(rowIndex, 1, mergeSet.SrcFile); err != nil {
				writer.Close()
				return fmt.Errorf("写入 shp 属性失败 %s: %w", outputPath, err)
			}
			if err := writer.WriteAttribute(rowIndex, 2, point.PointIndex); err != nil {
				writer.Close()
				return fmt.Errorf("写入 shp 属性失败 %s: %w", outputPath, err)
			}
			if err := writer.WriteAttribute(rowIndex, 3, point.RecordIndex); err != nil {
				writer.Close()
				return fmt.Errorf("写入 shp 属性失败 %s: %w", outputPath, err)
			}
			rowIndex++
		}
	}
	writer.Close()

	if err := normalizeDBFFileName(basePath); err != nil {
		return err
	}

	if err := copyProjectionFile(prjSource, outputPath); err != nil {
		return err
	}
	return nil
}

func normalizeDBFFileName(basePath string) error {
	legacyDBFPath := basePath + "dbf"
	standardDBFPath := basePath + ".dbf"
	if _, err := os.Stat(legacyDBFPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("检查 dbf 文件失败 %s: %w", legacyDBFPath, err)
	}
	if _, err := os.Stat(standardDBFPath); err == nil {
		if removeErr := os.Remove(standardDBFPath); removeErr != nil {
			return fmt.Errorf("删除旧 dbf 文件失败 %s: %w", standardDBFPath, removeErr)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("检查标准 dbf 文件失败 %s: %w", standardDBFPath, err)
	}
	if err := os.Rename(legacyDBFPath, standardDBFPath); err != nil {
		return fmt.Errorf("重命名 dbf 文件失败 %s -> %s: %w", legacyDBFPath, standardDBFPath, err)
	}
	return nil
}

func copyProjectionFile(sourceSHPPath, outputSHPPath string) error {
	if sourceSHPPath == "" {
		return nil
	}
	sourcePRJPath := strings.TrimSuffix(sourceSHPPath, filepath.Ext(sourceSHPPath)) + ".prj"
	content, err := os.ReadFile(sourcePRJPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取投影文件失败 %s: %w", sourcePRJPath, err)
	}
	outputPRJPath := strings.TrimSuffix(outputSHPPath, filepath.Ext(outputSHPPath)) + ".prj"
	if err := os.WriteFile(outputPRJPath, content, 0o644); err != nil {
		return fmt.Errorf("写入投影文件失败 %s: %w", outputPRJPath, err)
	}
	return nil
}

func createOutputFile(path string) (*os.File, error) {
	absolutePath := normalizePath(path)
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败 %s: %w", absolutePath, err)
	}
	file, err := os.Create(absolutePath)
	if err != nil {
		return nil, fmt.Errorf("创建输出文件失败 %s: %w", absolutePath, err)
	}
	return file, nil
}

func newLogger(out io.Writer, verbose bool) *slog.Logger {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{Level: level}))
}

func resolveSHPInputs(input string) ([]string, error) {
	normalized := normalizeOptional(input)
	if normalized == "" {
		return nil, fmt.Errorf("路径不能为空")
	}

	info, err := os.Stat(normalized)
	if err != nil {
		return nil, fmt.Errorf("读取路径失败 %s: %w", normalized, err)
	}
	if !info.IsDir() {
		if !isPointSHPFile(normalized) {
			return nil, fmt.Errorf("仅支持 point shp 文件: %s", normalized)
		}
		return []string{normalized}, nil
	}

	entries, err := os.ReadDir(normalized)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败 %s: %w", normalized, err)
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fullPath := filepath.Join(normalized, entry.Name())
		if isPointSHPFile(fullPath) {
			paths = append(paths, normalizePath(fullPath))
		}
	}
	slices.Sort(paths)
	if len(paths) == 0 {
		return nil, fmt.Errorf("目录下未找到 point shp 文件: %s", normalized)
	}
	return paths, nil
}

func excludePaths(inputs, excludes []string) []string {
	excludeSet := make(map[string]struct{}, len(excludes))
	for _, item := range excludes {
		excludeSet[normalizePath(item)] = struct{}{}
	}
	filtered := make([]string, 0, len(inputs))
	for _, item := range inputs {
		if _, exists := excludeSet[normalizePath(item)]; exists {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func normalizePath(filePath string) string {
	cleaned := filepath.Clean(filePath)
	if abs, err := filepath.Abs(cleaned); err == nil {
		return abs
	}
	return cleaned
}

func normalizeOptional(filePath string) string {
	if filePath == "" {
		return ""
	}
	return normalizePath(filePath)
}

func isPointSHPFile(filePath string) bool {
	if !strings.EqualFold(filepath.Ext(filePath), ".shp") {
		return false
	}
	name := strings.TrimSuffix(strings.ToLower(filepath.Base(filePath)), ".shp")
	return !strings.HasSuffix(name, "_line")
}

func TrackStemFromPath(filePath string) string {
	base := filepath.Base(filePath)
	ext := path.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if strings.HasSuffix(name, "_point") {
		return strings.TrimSuffix(name, "_point")
	}
	return name
}

func isGeographicCoordinateSystem(path string, points []PointFeature) bool {
	prjPath := strings.TrimSuffix(path, filepath.Ext(path)) + ".prj"
	content, err := os.ReadFile(prjPath)
	if err == nil {
		text := strings.ToUpper(string(content))
		if strings.Contains(text, "GEOGCS") || strings.Contains(text, "GEOGCRS") || strings.Contains(text, "UNIT[\"DEGREE\"") {
			return true
		}
		if strings.Contains(text, "PROJCS") || strings.Contains(text, "PROJCRS") {
			return false
		}
	}

	if len(points) == 0 {
		return false
	}
	sample := points[0]
	return math.Abs(sample.X) <= 180 && math.Abs(sample.Y) <= 90
}

func parseFlexibleInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	if value, err := strconv.Atoi(raw); err == nil {
		return value
	}
	if value, err := strconv.ParseFloat(raw, 64); err == nil {
		return int(value)
	}
	return fallback
}

func parseFlexibleFloat(raw string) float64 {
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	return value
}

func cleanDBFString(raw string) string {
	return strings.TrimSpace(strings.TrimRight(raw, "\x00"))
}

func degreesToRadians(value float64) float64 {
	return value * math.Pi / 180
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 6, 64)
}

func formatInt(value int) string {
	if value < 0 {
		return ""
	}
	return strconv.Itoa(value)
}

func decisionLabel(keep bool) string {
	if keep {
		return "保留"
	}
	return "舍弃"
}

func matchStatusLabel(matched bool) string {
	if matched {
		return "匹配"
	}
	return "不匹配"
}

func writeLines(path string, lines []string) error {
	file, err := createOutputFile(path)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, line := range lines {
		if _, err := file.WriteString(line + "\n"); err != nil {
			return fmt.Errorf("写入文本文件失败 %s: %w", path, err)
		}
	}
	return nil
}
