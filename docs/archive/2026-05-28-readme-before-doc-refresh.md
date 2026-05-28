# 🦎 火蜥蜴工具箱 (Fire Salamander Tools)

桌面工具平台。一个 IDE/Postman 风格的单页工作台，统一管理 Go/Python 工具的本地执行、远程执行和导出分发。

基于 **Wails v2 + Vue 3 + Go**。

## 项目结构

```
my_tools/
├── app/                   # Wails 桌面应用（Go module: fire-salamander-desktop）
│   ├── frontend/          # Vue 3 + Vite + Naive UI + Tailwind CSS
│   ├── internal/          # 后端内部包 (ssh/runtime/builder)
│   ├── main.go            # Wails 入口
│   ├── app.go             # App struct, startup, bootstrap API
│   ├── execution.go       # 本地/远程执行引擎 + 任务事件
│   ├── dialog.go          # Wails 原生文件对话框
│   ├── legacy.go          # 旧工具闭包桥接
│   ├── go.mod             # Go module
│   └── wails.json         # Wails 配置
├── libs/                  # 共享 Go 库
│   ├── core/toolspec/     # 工具规格核心类型
│   ├── catalog/builtin/   # 内置工具清单 (YAML manifests)
│   └── framework/         # 旧工具框架（桥接用）
├── tools/                 # 工具实现
│   ├── go_tools/          # Go 原生工具 (3个)
│   └── python_tools/      # Python 脚本工具
├── docs/                  # 架构文档 + ADR
│   ├── ARCHITECTURE.md
│   ├── PROJECT_OVERVIEW.md
│   ├── CONTEXT.md
│   └── adr/ (13篇)
├── scripts/
│   └── build.go           # 跨平台构建脚本
└── go.work                # Go workspace (use . ./app)
```

## 快速开始

### 构建桌面应用

```bash
# 一键完整构建
go run scripts/build.go

# 或直接 Wails 构建
cd app && "$(go env GOPATH)/bin/wails" build -clean

# 开发模式
cd app && "$(go env GOPATH)/bin/wails" dev
```

产物位置：`build/image/host/`（构建脚本）

### 构建单平台/全平台

```bash
# 仅当前平台
go run scripts/build.go

# 全平台交叉编译
go run scripts/build.go -all
```

## 工具管理

### 内置工具（4个）

| 工具 | 类型 | 说明 |
|------|------|------|
| utm_extract_to_gis | Go | 点云UTM坐标提取与GIS转换 |
| geojson_to_shapefile | Go | GeoJSON转Shapefile |
| pos_trajectory_to_gis | Go | POS轨迹转GIS格式 |
| restore_pcd_by_mgrs | Python | MGRS坐标还原点云数据 |

工具通过 `libs/catalog/builtin/manifests/*.yaml` 定义清单，`libs/catalog/builtin/service.go` 加载。

### 添加自定义工具

**Go 工具**：在 `tools/go_tools/` 下新建目录，实现 `Tool` 接口，在 `app/legacy.go` 中注册。

**Python 工具**：将 `.py` 文件放入 `tools/python_tools/scripts/`，重新编译即可在 `Python 脚本` 分类下看到。

## 前端

前端位于 `app/frontend/`，技术栈：
- Vue 3 + TypeScript + Pinia + Naive UI + Tailwind CSS
- 单页工作台（无缝路由跳转）
- 工具分类树 + 可关闭页签 + 执行终端

### 前端命令

```bash
cd app/frontend
npm install          # 安装依赖
npm run dev          # 启动开发服务器
npm run lint         # ESLint
npm run typecheck    # TypeScript 类型检查
```

## 文档

| 文档 | 内容 |
|------|------|
| [CONTEXT.md](CONTEXT.md) | 领域术语表与项目语言 |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | 当前架构落地情况 |
| [docs/PROJECT_OVERVIEW.md](docs/PROJECT_OVERVIEW.md) | 项目全景 |
| [docs/adr/](docs/adr/) | 架构决策记录 (13篇) |

## 许可证

MIT
