package main

type ExecutionRequest struct {
	ToolID    string `json:"toolId"`
	Args      string `json:"args"`
	PythonEnv string `json:"pythonEnv"`
}

type ExecutionTask struct {
	ID                         string `json:"id"`
	ToolID                     string `json:"toolId"`
	ToolName                   string `json:"toolName"`
	Status                     string `json:"status"`
	Target                     string `json:"target"`
	RemoteConnID               string `json:"remoteConnId,omitempty"`
	Args                       string `json:"args"`
	PythonEnv                  string `json:"pythonEnv,omitempty"`
	Usage                      string `json:"usage"`
	StartedAt                  int64  `json:"startedAt"`
	EndedAt                    int64  `json:"endedAt,omitempty"`
	ExitMessage                string `json:"exitMessage,omitempty"`
	RemoteResultStatus         string `json:"remoteResultStatus,omitempty"`
	RemoteResultPath           string `json:"remoteResultPath,omitempty"`
	RemoteResultKind           string `json:"remoteResultKind,omitempty"`
	RemoteResultMessage        string `json:"remoteResultMessage,omitempty"`
	RemoteResultDownloadedPath string `json:"remoteResultDownloadedPath,omitempty"`

	remoteWorkDir string
}
