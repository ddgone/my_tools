package bxn_point_cloud_renew

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// httpClient 带超时的 HTTP 客户端，避免接口无响应时永久挂起。
var httpClient = &http.Client{Timeout: 30 * time.Second}

// taskFrame 一个任务框（作业框）。
type taskFrame struct {
	id      string
	polygon [][]float64 // [lon, lat] 顶点
}

// 接口返回结构（复用 frame_tool 定义）。
type ktsTaskResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Result  []struct {
		Tags []struct {
			K string `json:"k"`
			V string `json:"v"`
		} `json:"tags"`
	} `json:"result"`
}

type frameQueryResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Result  struct {
		Data []struct {
			TaskFrameID string `json:"taskFrameId"`
			Range       struct {
				Geojson struct {
					Coordinates [][][]float64 `json:"coordinates"`
					Type        string        `json:"type"`
				} `json:"geojson"`
			} `json:"range"`
		} `json:"data"`
	} `json:"result"`
}

// fetchTaskFrames 根据 projectId 获取任务框坐标。
func fetchTaskFrames(projectID string) ([]taskFrame, error) {
	// 1. 获取 taskFrameIds 和 branchName
	ids, branch, err := getTaskInfo(projectID)
	if err != nil {
		return nil, fmt.Errorf("get task info failed: %w", err)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no taskFrameIds found for project %s", projectID)
	}

	// 2. 查询每个框坐标
	frames, err := queryTaskFrames(branch, ids)
	if err != nil {
		return nil, fmt.Errorf("query task frames failed: %w", err)
	}
	if len(frames) == 0 {
		return nil, fmt.Errorf("no frame coordinates returned for project %s", projectID)
	}

	return frames, nil
}

func getTaskInfo(projectID string) ([]string, string, error) {
	url := fmt.Sprintf("http://kts.gzproduction.com/kts/task/findByProjectId?projectId=%s", projectID)
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	var response ktsTaskResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, "", err
	}
	if len(response.Result) == 0 {
		return nil, "", nil
	}

	var taskFrameIDs string
	var branchName string
	for _, tag := range response.Result[0].Tags {
		switch tag.K {
		case "taskFrameIds":
			taskFrameIDs = tag.V
		case "branchName":
			branchName = tag.V
		}
	}

	uniqueMap := make(map[string]bool)
	var uniqueIDs []string
	for _, id := range strings.Split(taskFrameIDs, ",") {
		id = strings.TrimSpace(id)
		if id != "" && !uniqueMap[id] {
			uniqueMap[id] = true
			uniqueIDs = append(uniqueIDs, id)
		}
	}
	return uniqueIDs, branchName, nil
}

func queryTaskFrames(branchName string, taskFrameIDs []string) ([]taskFrame, error) {
	url := "http://data-branch-mgt.gzproduction.com/branch/queryTaskFrames"

	var frames []taskFrame
	for _, frameID := range taskFrameIDs {
		reqBody := map[string]interface{}{
			"branchName":   branchName,
			"queryType":    3,
			"taskFrameIds": []string{frameID},
		}
		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return nil, err
		}

		resp, err := httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, err
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		var fr frameQueryResponse
		if err := json.Unmarshal(body, &fr); err != nil {
			return nil, err
		}
		if fr.Code != 0 {
			return nil, fmt.Errorf("query failed for %s: %s", frameID, fr.Message)
		}

		if len(fr.Result.Data) > 0 {
			coords := fr.Result.Data[0].Range.Geojson.Coordinates
			if len(coords) > 0 && len(coords[0]) > 0 {
				frames = append(frames, taskFrame{
					id:      frameID,
					polygon: coords[0],
				})
			}
		}
	}
	return frames, nil
}

// ==================== 框的离线存储与统一加载 ====================

// frameGeoJSON 任务框的 GeoJSON 结构（FeatureCollection）。
type frameGeoJSON struct {
	Type     string `json:"type"`
	Features []struct {
		Geometry struct {
			Type        string        `json:"type"`
			Coordinates [][][]float64 `json:"coordinates"`
		} `json:"geometry"`
		Properties struct {
			TaskFrameID string `json:"taskFrameId"`
		} `json:"properties"`
	} `json:"features"`
}

// isProjectID 判断第三列是纯数字 projectId（在线拉取）还是 geojson 路径（离线读取）。
func isProjectID(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// readFramesFromGeojson 从 GeoJSON 文件读取任务框。
func readFramesFromGeojson(path string) ([]taskFrame, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read geojson failed: %w", err)
	}
	var fc frameGeoJSON
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parse geojson failed: %w", err)
	}

	var frames []taskFrame
	for _, feat := range fc.Features {
		if len(feat.Geometry.Coordinates) == 0 || len(feat.Geometry.Coordinates[0]) == 0 {
			continue
		}
		frames = append(frames, taskFrame{
			id:      feat.Properties.TaskFrameID,
			polygon: feat.Geometry.Coordinates[0],
		})
	}
	if len(frames) == 0 {
		return nil, fmt.Errorf("no frames in geojson: %s", path)
	}
	return frames, nil
}

// saveFramesToGeojson 将任务框保存为 GeoJSON 文件。
func saveFramesToGeojson(frames []taskFrame, path string) error {
	fc := frameGeoJSON{Type: "FeatureCollection"}
	for _, f := range frames {
		feat := struct {
			Geometry struct {
				Type        string        `json:"type"`
				Coordinates [][][]float64 `json:"coordinates"`
			} `json:"geometry"`
			Properties struct {
				TaskFrameID string `json:"taskFrameId"`
			} `json:"properties"`
		}{}
		feat.Geometry.Type = "Polygon"
		feat.Geometry.Coordinates = [][][]float64{f.polygon}
		feat.Properties.TaskFrameID = f.id
		fc.Features = append(fc.Features, feat)
	}

	data, err := json.MarshalIndent(fc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// loadFrames 统一加载框，带内存缓存与磁盘缓存：
//   - spec 为纯数字 projectId：优先读取 <framesDir>/<pid>.geojson（存在则离线读取）；
//     缺失则在线拉取，若配置了 framesDir 则同时保存进缓存目录。
//   - spec 为文件路径：直接离线读取。
func loadFrames(spec, framesDir string, cache map[string][]taskFrame, out io.Writer) ([]taskFrame, error) {
	if spec == "" {
		return nil, fmt.Errorf("frame spec is empty")
	}
	if f, ok := cache[spec]; ok {
		return f, nil
	}

	var frames []taskFrame
	var err error
	if isProjectID(spec) {
		cachePath := filepath.Join(framesDir, spec+".geojson")
		if framesDir != "" && fileExists(cachePath) {
			frames, err = readFramesFromGeojson(cachePath)
		} else {
			frames, err = fetchTaskFrames(spec)
			if err == nil && framesDir != "" {
				if serr := saveFramesToGeojson(frames, cachePath); serr != nil {
					fmt.Fprintf(out, "WARN: save frames cache failed: %v\n", serr)
				}
			}
		}
	} else {
		frames, err = readFramesFromGeojson(spec)
	}
	if err != nil {
		return nil, err
	}
	cache[spec] = frames
	return frames, nil
}
