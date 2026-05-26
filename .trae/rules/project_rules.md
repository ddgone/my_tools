# 项目质量检查规范

## 项目结构（标准 Wails 布局）

```
my_tools/
├── app/                   # Wails 桌面应用（Go module）
│   ├── frontend/          # Vue 3 + Vite 前端
│   ├── internal/          # 后端内部包
│   │   ├── ssh/           # SSH 连接持久化存储
│   │   ├── runtime/       # SSH 远程执行器
│   │   ├── runtimeenv/    # 运行时目录布局
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
├── build/                 # 构建产物（gitignore）
│   ├── image/host/        # 只读：桌面宿主 exe
│   └── runtime/           # 可变：缓存/脚本/日志/导出/配置
├── docs/                  # 文档 + ADR
├── scripts/
│   ├── build.go           # 构建脚本
│   └── package/
│       └── main.go         # 安装包打包脚本
└── go.work                # use . ./app
```

## 构建命令

```bash
# 当前平台构建 + 初始化运行时目录 + 生成默认配置
go run scripts/build.go

# 全平台交叉编译
go run scripts/build.go -all

# 打安装包（产物在 build/exports/）
go run scripts/package/main.go           # 版本号 dev
go run scripts/package/main.go 1.0.0     # 指定版本号

# 开发模式
cd app && "$(go env GOPATH)/bin/wails" dev
```

## 产物目录

| 路径                        | 用途                     |
|---------------------------|------------------------|
| `build/image/host/`       | 只读：桌面宿主构建产物        |
| `build/runtime/cache/`    | 单工具编译缓存、Python脚本副本  |
| `build/runtime/config/`   | SSH连接配置、应用首选项        |
| `build/runtime/logs/`     | 运行日志                    |
| `build/runtime/exports/`  | 单工具导出产物               |
| `build/exports/`          | 安装包输出                   |

开发态自动使用 `build/runtime/`，安装态回退到 `~/.fire-salamander/`。
`build/` 整个目录在 `.gitignore` 中。

## 代码检查

```bash
# Go 静态分析
go vet ./...

# 前端
cd app/frontend && npm run lint
cd app/frontend && npm run typecheck
```
