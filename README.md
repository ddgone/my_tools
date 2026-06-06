# 火蜥蜴工具箱

火蜥蜴工具箱是一个桌面宿主型工具平台：用一个单页工作台统一承载 Go/Python 工具的本地执行、基础远程执行、日志导出、单工具导出，以及面向多工具多平台的产物中心。

## 当前状态

- 已接通本地执行链路，前后端事件流、日志面板、参数表单和页签工作区均可用。
- 已接通基础远程执行链路，包含 SSH 连接管理、远端上传、执行和清理。
- 已接通单工具导出与产物中心，支持批量构建缓存、批量导出、任务历史持久化、任务快照回看，以及与远程执行共享构建缓存。
- 已接通 Go 环境配置：支持本地 Go 选择、官方 SDK 下载、显式禁用当前 SDK，以及状态栏环境提示。
- 已接通 Python 环境配置：支持基础 Python 选择、按解释器隔离的托管虚拟环境、可停止的环境/依赖任务、显式禁用当前环境、动态依赖扫描检查与一键安装，以及状态栏环境提示。
- Python 动态依赖扫描当前使用内置的 `pipreqs` 数据文件 `mapping.txt` 与 `stdlib.txt`，由 Go 宿主统一管理并建议定期更新。
- 当前主要风险集中在 SSH 安全、运行时鲁棒性、构建脚本容错和文档真相源漂移。

## 主要能力

- 单工具工作台：支持本地执行、远程执行、参数表单、终端日志和单工具导出。
- 产物中心：支持工具 × 平台矩阵选择、批量构建缓存、批量导出、缓存命中预估、失败项重试和结果摘要导出。
- 产物工作流：左侧侧栏提供产物工作台入口与任务历史卡片，右侧支持完整工作台页面和单次任务快照页。
- Go 工具的本地执行不依赖运行时 Go 环境；Go 环境当前只影响远程执行、Go 导出和构建缓存准备。
- Python 工具的本地执行依赖宿主管理的托管虚拟环境；当前通过“基础 Python + 托管 venv + 动态依赖扫描”模型接入。

## 仓库结构

```text
my_tools/
├── app/                   # Wails 桌面宿主
│   ├── frontend/          # Vue 3 + TypeScript 单页工作台
│   ├── internal/          # ssh/runtime/builder/runtimeenv 等后端内部包
│   ├── main.go            # Wails 入口
│   ├── app.go             # bootstrap、SSH API、窗口状态
│   ├── execution.go       # 本地/远程执行编排
│   └── legacy.go          # 旧工具桥接
├── libs/                  # 工具规格与内置清单加载器
├── tools/                 # Go 工具与 Python 工具资产
├── docs/                  # 当前文档、问题计划、ADR、归档
├── scripts/               # 构建与打包脚本
├── build/                 # 构建产物与开发态运行时目录
└── go.work                # Go workspace
```

## 快速开始

### 安装依赖

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
cd app/frontend && npm install
```

### 开发模式

```bash
cd app
"$(go env GOPATH)/bin/wails" dev
```

### 构建桌面应用

```bash
go run scripts/build.go
```

## 质量检查

以下命令已在当前仓库复核通过：

```bash
go test ./...
go vet ./libs/... ./tools/... ./scripts/...
cd app && go vet ./...
cd app/frontend && npm run lint
cd app/frontend && npm run typecheck
```

## 文档导航

- [CONTEXT.md](CONTEXT.md)：领域术语表，只记录项目语言，不记录实现细节。
- [docs/PROJECT_OVERVIEW.md](docs/PROJECT_OVERVIEW.md)：当前产品定位、能力边界和文档索引。
- [docs/GO_ENVIRONMENT.md](docs/GO_ENVIRONMENT.md)：Go 环境的用户视角说明、影响范围、默认路径、`<无 SDK>` 语义与验收清单。
- [docs/PYTHON_ENVIRONMENT.md](docs/PYTHON_ENVIRONMENT.md)：Python 环境的用户视角说明、基础 Python、托管虚拟环境、动态依赖扫描、一键安装与 `<无 Python>` 语义。
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)：当前真实落地架构、模块边界、执行链路和约束。
- [docs/DEVELOPMENT_SETUP.md](docs/DEVELOPMENT_SETUP.md)：最新开发环境、启动方式、校验命令和入口文件。
- [docs/adr/0022-host-owned-background-artifact-preparation.md](docs/adr/0022-host-owned-background-artifact-preparation.md)：后台产物准备、缓存复用与产物中心工作区形态。
- [docs/ISSUES_AND_REMEDIATION_PLAN.md](docs/ISSUES_AND_REMEDIATION_PLAN.md)：本轮梳理出的全部问题，以及严格按 1 到 2 个问题拆分的修复步骤。
- [docs/archive/README.md](docs/archive/README.md)：历史计划、旧版说明和归档原因。
- [docs/adr/](docs/adr/)：仍有效的架构决策记录。

## 文档约定

- `README.md` 只做入口和导航，不承载历史方案。
- `docs/` 下默认只放当前有效文档；阶段性计划、复盘、旧版说明统一进入 `docs/archive/`。
- 若代码与文档冲突，以代码现状为准，并优先更新 `ARCHITECTURE.md`、`DEVELOPMENT_SETUP.md` 与 `ISSUES_AND_REMEDIATION_PLAN.md`。
