package main

import (
	"context"
	"runtime"
	"sort"
	"sync"

	"fire-salamander-desktop/internal/ssh"
	"my_tools/libs/core/toolspec"
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
		if tools[i].Category == tools[j].Category {
			return tools[i].Name < tools[j].Name
		}
		return tools[i].Category < tools[j].Category
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

func (a *App) SaveSSHConnection(conn ssh.Connection) error {
	return a.sshStore.Save(conn)
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
	return ssh.TestConnection(creds.Host, creds.Port, creds.User, creds.Password)
}

func (a *App) TestSSHConnectionRaw(host string, port int, user, password string) ssh.TestResult {
	if host == "" || user == "" {
		return ssh.TestResult{Success: false, Message: "主机地址和用户名不能为空"}
	}
	return ssh.TestConnection(host, port, user, password)
}
