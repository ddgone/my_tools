package main

type DownloadTask struct {
	ID               string  `json:"id"`
	SourceTaskID     string  `json:"sourceTaskId"`
	ToolID           string  `json:"toolId"`
	ToolName         string  `json:"toolName"`
	Status           string  `json:"status"`
	RemoteResultPath string  `json:"remoteResultPath"`
	RemoteResultKind string  `json:"remoteResultKind"`
	LocalPath        string  `json:"localPath,omitempty"`
	Directory        string  `json:"directory,omitempty"`
	Message          string  `json:"message,omitempty"`
	DownloadedBytes  int64   `json:"downloadedBytes"`
	TotalBytes       int64   `json:"totalBytes"`
	ProgressPercent  float64 `json:"progressPercent"`
	StartedAt        int64   `json:"startedAt"`
	EndedAt          int64   `json:"endedAt,omitempty"`
}
