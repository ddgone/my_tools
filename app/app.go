package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"

	"fire-salamander-desktop/internal/runtimeenv"

	"my_tools/libs/core/toolspec"

	"fire-salamander-desktop/internal/ssh"
)

type App struct {
	ctx       context.Context
	mu        sync.RWMutex
	legacy    map[string]*legacyTool
	manifests map[string]toolspec.ToolManifest
	tasks     map[string]*ExecutionTask
	cancels   map[string]context.CancelFunc
	sshStore  *ssh.Store
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
	return &App{
		legacy:    map[string]*legacyTool{},
		manifests: map[string]toolspec.ToolManifest{},
		tasks:     map[string]*ExecutionTask{},
		cancels:   map[string]context.CancelFunc{},
		sshStore:  ssh.NewStore(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	_ = a.ensureTooling()
	_ = a.sshStore.LoadConfig()
}

func (a *App) GetWindowConfig() WindowState {
	return a.loadWindowConfig()
}

func (a *App) SaveWindowState(state WindowState) error {
	return a.writeWindowConfig(state)
}

func (a *App) GetCurrentWindowState() (WindowState, error) {
	return a.currentWindowState()
}

func (a *App) PersistCurrentWindowState() error {
	return a.persistCurrentWindowState()
}

func (a *App) IsWindowRectVisible(x, y, width, height int) bool {
	return isWindowRectVisible(x, y, width, height)
}

func (a *App) loadWindowConfig() WindowState {
	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return WindowState{Width: 0, Height: 0, X: -1, Y: -1}
	}
	data, err := os.ReadFile(filepath.Join(layout.ConfigDir(), "app.json"))
	if err != nil {
		return WindowState{Width: 0, Height: 0, X: -1, Y: -1}
	}
	var cfg struct {
		Window WindowState `json:"window"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return WindowState{Width: 0, Height: 0, X: -1, Y: -1}
	}
	return cfg.Window
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

func (a *App) writeWindowConfig(state WindowState) error {
	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return fmt.Errorf("解析运行时目录失败: %w", err)
	}
	if err := os.MkdirAll(layout.ConfigDir(), 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}
	configPath := filepath.Join(layout.ConfigDir(), "app.json")

	cfg, err := loadConfigDocument(configPath)
	if err != nil {
		return err
	}

	windowData, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("序列化窗口状态失败: %w", err)
	}
	cfg["window"] = windowData

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("格式化配置文件失败: %w", err)
	}

	if err := os.WriteFile(configPath, out, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}
	return nil
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
	if err := a.ensureTooling(); err != nil {
		return nil, err
	}

	tools := make([]toolspec.ToolManifest, 0, len(a.manifests))
	for _, tool := range a.manifests {
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

func (a *App) ensureTooling() error {
	toolInitOnce.Do(func() {
		legacy := loadLegacyTools()
		manifests, err := buildToolManifests(legacy)
		if err != nil {
			cachedToolingErr = err
			return
		}
		cachedLegacy = legacy
		cachedManifests = manifests
	})

	if cachedToolingErr != nil {
		return cachedToolingErr
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.legacy = cachedLegacy
	a.manifests = cachedManifests

	return nil
}

func (a *App) ListSSHConnections() []*ssh.Connection {
	return a.sshStore.List()
}

func (a *App) GetSSHConnection(id string) (*ssh.Connection, error) {
	return a.sshStore.GetCredentials(id)
}

func (a *App) SaveSSHConnection(conn ssh.Connection) (ssh.Connection, error) {
	id, err := a.sshStore.Save(conn)
	if err != nil {
		return ssh.Connection{}, err
	}
	saved, err := a.sshStore.GetCredentials(id)
	if err != nil {
		return ssh.Connection{}, err
	}
	return *saved, nil
}

func (a *App) DeleteSSHConnection(id string) error {
	return a.sshStore.Delete(id)
}

func (a *App) UpdateSSHConnection(id string, conn ssh.Connection) error {
	return a.sshStore.Update(id, conn)
}

func (a *App) TestSSHConnection(id string) ssh.TestResult {
	creds, err := a.sshStore.GetCredentials(id)
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
		if err := a.sshStore.Update(id, *creds); err != nil {
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
