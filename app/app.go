package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"

	"my_tools/libs/core/toolspec"

	"fire-salamander-desktop/internal/execution"
	"fire-salamander-desktop/internal/ssh"
)

type App struct {
	state     *SharedState
	dialog    *DialogManager
	window    *WindowManager
	export    *ExportManager
	task      *TaskResultManager
	execution *ExecutionManager
	artifact  *ArtifactBatchManager
}

type WindowState struct {
	Width      int  `json:"width"`
	Height     int  `json:"height"`
	X          int  `json:"x"`
	Y          int  `json:"y"`
	Maximised  bool `json:"maximised"`
	Fullscreen bool `json:"fullscreen"`
}

var topLevelCategoryOrder = map[string]int{
	"通用测试工具": 0,
	"KD测试工具": 1,
	"Rust工具": 2,
}

func compareCategoryPath(left, right toolspec.CategoryPath) int {
	leftTop := ""
	rightTop := ""
	if len(left) > 0 {
		leftTop = left[0]
	}
	if len(right) > 0 {
		rightTop = right[0]
	}

	leftOrder, leftHasOrder := topLevelCategoryOrder[leftTop]
	rightOrder, rightHasOrder := topLevelCategoryOrder[rightTop]
	switch {
	case leftHasOrder && rightHasOrder && leftOrder != rightOrder:
		return leftOrder - rightOrder
	case leftHasOrder && !rightHasOrder:
		return -1
	case !leftHasOrder && rightHasOrder:
		return 1
	}

	limit := len(left)
	if len(right) < limit {
		limit = len(right)
	}
	for i := 0; i < limit; i++ {
		if left[i] == right[i] {
			continue
		}
		if left[i] < right[i] {
			return -1
		}
		return 1
	}
	switch {
	case len(left) < len(right):
		return -1
	case len(left) > len(right):
		return 1
	default:
		return 0
	}
}

const defaultAppConfigJSON = `{
  "app": {
    "version": "1.0.0",
    "language": "zh-CN"
  },
  "execution": {
    "defaultPython": "python3",
    "maxHistory": 50,
    "remoteTimeoutSec": 30
  },
  "export": {
    "lastDirectory": "",
    "goMode": "binary",
    "autoOpenDir": true
  },
  "go": {
    "selectedBinary": "",
    "knownBinaries": [],
    "lastInstallDirectory": "",
    "disabled": false
  },
  "python": {
    "selectedBinary": "",
    "knownBinaries": [],
    "disabled": false
  },
  "ui": {
    "theme": "dracula",
    "verboseShortcuts": false
  },
  "window": {
    "width": 0,
    "height": 0,
    "x": -1,
    "y": -1,
    "maximised": false,
    "fullscreen": false
  }
}
`

func NewApp() *App {
	state := NewSharedState()
	dialog := NewDialogManager()
	exportMgr := NewExportManager(state, dialog)
	taskMgr := NewTaskResultManager(state, dialog, exportMgr)
	return &App{
		state:     state,
		dialog:    dialog,
		window:    NewWindowManager(state),
		export:    exportMgr,
		task:      taskMgr,
		execution: execution.NewManager(state, taskMgr, ensureTooling),
		artifact:  NewArtifactBatchManager(state),
	}
}

func (a *App) startup(ctx context.Context) {
	a.state.Ctx = ctx
	_ = ensureTooling(a.state)
	_ = a.state.SSHStore.LoadConfig()
	_ = a.loadArtifactBatchTasks()
}

func (a *App) domReady(ctx context.Context) {
	a.window.DomReady(ctx)
}

func (a *App) beforeClose(ctx context.Context) bool {
	return a.window.BeforeClose(ctx)
}

func (a *App) GetWindowConfig() WindowState {
	return a.window.GetWindowConfig()
}

func (a *App) SaveWindowState(state WindowState) error {
	return a.window.SaveWindowState(state)
}

func (a *App) GetCurrentWindowState() (WindowState, error) {
	return a.window.GetCurrentWindowState()
}

func (a *App) PersistCurrentWindowState() error {
	return a.window.PersistCurrentWindowState()
}

func (a *App) IsWindowRectVisible(x, y, width, height int) bool {
	return a.window.IsWindowRectVisible(x, y, width, height)
}

func defaultConfigDocument() map[string]json.RawMessage {
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal([]byte(defaultAppConfigJSON), &cfg); err != nil || cfg == nil {
		return map[string]json.RawMessage{}
	}
	return cfg
}

func loadConfigDocument(configPath string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultConfigDocument(), nil
		}
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(data, &cfg); err != nil || cfg == nil {
		return defaultConfigDocument(), nil
	}
	return cfg, nil
}

type WorkbenchBootstrap struct {
	AppTitle         string                  `json:"appTitle"`
	Platform         string                  `json:"platform"`
	HostStack        []string                `json:"hostStack"`
	PrimaryFlow      []string                `json:"primaryFlow"`
	ModuleBoundaries []string                `json:"moduleBoundaries"`
	ParameterModes   []string                `json:"parameterModes"`
	Tools            []toolspec.ToolManifest `json:"tools"`
}

func (a *App) GetWorkbenchBootstrap() (*WorkbenchBootstrap, error) {
	if err := ensureTooling(a.state); err != nil {
		return nil, err
	}

	tools := make([]toolspec.ToolManifest, 0, len(a.state.Manifests))
	for _, tool := range a.state.Manifests {
		tools = append(tools, tool)
	}
	sort.Slice(tools, func(i, j int) bool {
		if compareCategoryPath(tools[i].Category, tools[j].Category) == 0 {
			return tools[i].Name < tools[j].Name
		}
		return compareCategoryPath(tools[i].Category, tools[j].Category) < 0
	})

	return &WorkbenchBootstrap{
		AppTitle: "火蜥蜴工具箱 Desktop",
		Platform: runtime.GOOS + "/" + runtime.GOARCH,
		HostStack: []string{
			"Wails v2",
			"Vue 3",
			"TypeScript",
			"Naive UI",
			"Tailwind CSS",
		},
		PrimaryFlow: []string{
			"选工具",
			"配参数",
			"本地执行",
			"看日志结果",
			"切远程执行",
			"上传工具专属产物",
			"远端执行并清理",
			"导出该工具",
		},
		ModuleBoundaries: []string{
			"app/backend: Wails 桌面宿主",
			"libs/core: 工具规格与平台模型",
			"libs/catalog: 内置/自定义工具来源",
			"tools: Go/Python 工具实现",
		},
		ParameterModes: []string{
			"结构化表单",
			"原始参数模式",
		},
		Tools: tools,
	}, nil
}

func (a *App) ListSSHConnections() []*ssh.Connection {
	return a.state.SSHStore.List()
}

func (a *App) GetSSHConnection(id string) (*ssh.Connection, error) {
	return a.state.SSHStore.GetCredentials(id)
}

func (a *App) SaveSSHConnection(conn ssh.Connection) (ssh.Connection, error) {
	id, err := a.state.SSHStore.Save(conn)
	if err != nil {
		return ssh.Connection{}, err
	}
	saved, err := a.state.SSHStore.GetCredentials(id)
	if err != nil {
		return ssh.Connection{}, err
	}
	return *saved, nil
}

func (a *App) DeleteSSHConnection(id string) error {
	return a.state.SSHStore.Delete(id)
}

func (a *App) UpdateSSHConnection(id string, conn ssh.Connection) error {
	return a.state.SSHStore.Update(id, conn)
}

func (a *App) TestSSHConnection(id string) ssh.TestResult {
	creds, err := a.state.SSHStore.GetCredentials(id)
	if err != nil {
		return ssh.TestResult{Success: false, Message: err.Error()}
	}
	if creds.Password == "" && creds.KeyPath == "" {
		return ssh.TestResult{Success: false, Message: "连接缺少认证凭据"}
	}
	verifier := ssh.NewHostKeyVerifier(creds.HostKeyFingerprint)
	result := ssh.TestConnection(creds.Host, creds.Port, creds.User, creds.Password, creds.KeyPath, verifier)
	if result.Success && verifier.Accepted != "" && creds.HostKeyFingerprint != verifier.Accepted {
		creds.HostKeyFingerprint = verifier.Accepted
		if err := a.state.SSHStore.Update(id, *creds); err != nil {
			result.Message += " (注意: 主机指纹保存失败)"
		}
	}
	return result
}

func (a *App) TestSSHConnectionRaw(host string, port int, user, password, keyPath string) ssh.TestResult {
	if host == "" || user == "" {
		return ssh.TestResult{Success: false, Message: "主机地址和用户名不能为空"}
	}
	if password == "" && keyPath == "" {
		return ssh.TestResult{Success: false, Message: "必须提供密码或密钥路径"}
	}
	verifier := ssh.NewHostKeyVerifier("")
	return ssh.TestConnection(host, port, user, password, keyPath, verifier)
}

// Dialog delegates
