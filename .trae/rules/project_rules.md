# 项目质量检查规范

## 项目结构（标准 Wails 布局）

```
my_tools/
├── app/                   # Wails 桌面应用（Go module）
│   ├── frontend/          # Vue 3 + Vite 前端
│   ├── internal/          # 后端内部包
│   │   ├── ssh/           # SSH 连接持久化存储
│   │   ├── runtime/       # SSH 远程执行器
│   │   └── builder/       # 工具构建打包
│   ├── main.go            # Wails 入口 + go:embed
│   ├── app.go             # App struct + startup + SSH API
│   ├── execution.go       # 本地/远程执行引擎
│   ├── dialog.go          # Wails 原生文件对话框
│   ├── legacy.go          # 旧工具桥接注册
│   ├── go.mod             # module fire-salamander-desktop
│   └── wails.json         # frontend:dir = "frontend"
├── libs/                  # 共享 Go 库
│   ├── core/toolspec/     # 工具规格核心类型
│   ├── catalog/builtin/   # 内置工具清单 (YAML + loader)
│   └── framework/         # 旧工具框架（桥接用）
├── tools/                 # 工具实现
│   ├── go_tools/          # Go 原生工具 (3个)
│   └── python_tools/      # Python 脚本工具
├── docs/                  # 文档 + ADR
├── scripts/
│   └── build.go           # 构建脚本
└── go.work                # use . ./app
```

## 构建命令

```bash
# 当前平台构建
go run scripts/build.go

# 全平台交叉编译
go run scripts/build.go -all

# 快速 Wails 编译（仅当前平台）
cd app && "$(go env GOPATH)/bin/wails" build -clean

# 开发模式
cd app && "$(go env GOPATH)/bin/wails" dev
```

产物：`go run scripts/build.go` → `build/` 根目录下
       `wails build` → `app/build/bin/`

## 代码检查

```bash
# Go 静态分析
go vet ./...

# 前端
cd app/frontend && npm run lint
cd app/frontend && npm run typecheck
```
