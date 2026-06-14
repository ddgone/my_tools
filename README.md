# 火蜥蜴工具箱

火蜥蜴工具箱当前定位为一个个人桌面工具工作台：用一个单页工作台统一承载 Go / Python / Rust 工具的本地执行、基础远程执行、日志导出、单工具导出，以及面向多工具多平台的产物中心。当前目标是把这条主链路做稳、做顺手，而不是继续推进成轻量 IDE 或重型任务系统。

## 当前状态

- 已接通本地执行链路，前后端事件流、日志面板、参数表单和页签工作区均可用。
- 已接通基础远程执行链路，包含 SSH 连接管理、远端上传、执行和清理。
- 已接通远程结果闭环第一包：支持执行后结果探测、当前页签手动下载、目录远端打包后下载、下载任务抽屉与完成后打开目录。
- 已接通单工具导出与产物中心，支持批量构建缓存、批量导出、任务历史持久化、任务快照回看，以及与远程执行共享构建缓存。
- 已接通 Go 环境配置：支持本地 Go 选择、官方 SDK 下载、下载任务进度/速度/停止、环境检查、删除当前托管 SDK、显式禁用当前 SDK，以及状态栏环境提示。
- 已接通 Python 环境配置：支持基础 Python 选择、按解释器隔离的托管虚拟环境、可停止的环境/依赖任务、显式禁用当前环境、动态依赖扫描检查与一键安装，以及状态栏环境提示。
- 已接通 Rust 工具主链：Rust 工具支持本地执行、远程执行、单工具导出、构建缓存复用，以及产物中心矩阵构建。
- 已接通 Rust 环境配置：支持 `<无 SDK>` / 自动探测 / 手动选择 三种模式，支持 Rust 环境目录选择、Zig SDK 选择、托管 Rust / Zig 下载、`cargo-zigbuild` 与常用 targets 补齐，以及状态栏环境提示。
- Python 动态依赖扫描当前使用内置的 `pipreqs` 数据文件 `mapping.txt` 与 `stdlib.txt`，由 Go 宿主统一管理并建议定期更新。
- 当前主要风险集中在 SSH 安全、运行时鲁棒性、构建脚本容错，以及远程执行后的结果回收体验仍未闭环。
- 当前不再把“工作区模型、文件管理器、编辑器、重型任务中心”视为近期主线。

## 主要能力

- 单工具工作台：支持本地执行、远程执行、参数表单、终端日志和单工具导出。
- 远程结果回收：支持执行后探测输出结果、手动下载文件或目录、目录远端打包下载，以及下载任务进度反馈。
- 产物中心：支持工具 × 平台矩阵选择、批量构建缓存、批量导出、缓存命中预估、失败项重试和结果摘要导出。
- 产物工作流：左侧侧栏提供产物工作台入口与任务历史卡片，右侧支持完整工作台页面和单次任务快照页。
- Go 工具的本地执行不依赖运行时 Go 环境；Go 环境当前只影响远程执行、Go 导出和构建缓存准备。
- Python 工具的本地执行依赖宿主管理的托管虚拟环境；当前通过“基础 Python + 托管 venv + 动态依赖扫描”模型接入。
- Rust 工具的本地执行优先复用随宿主打包的本地产物；若当前运行在源码工作区，也可以通过当前选中的 Rust / Zig 环境现场构建本机产物。
- Rust 环境当前影响 Rust 工具的本机现场构建、远程执行前的单工具构建、Rust 单工具导出和构建缓存准备；不影响 Go/Python 工具。
- 当前优先补的不是新模块，而是三条顺手能力：远程结果探测与手动下载、参数级远程路径选择、独立 SSH 终端页签。

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
cd app && go test ./...
go vet ./libs/... ./tools/... ./scripts/...
cd app && go vet ./...
cd app/frontend && npm run lint
cd app/frontend && npm run typecheck
```

## 文档导航

- [CONTEXT.md](CONTEXT.md)：领域术语表，只记录项目语言，不记录实现细节。
- [docs/PROJECT_OVERVIEW.md](docs/PROJECT_OVERVIEW.md)：当前产品定位、能力边界和文档索引。
- [docs/CURRENT_DEVELOPMENT_BRIEF.md](docs/CURRENT_DEVELOPMENT_BRIEF.md)：当前开发边界、首个实现包、涉及文件与完成标准。
- [docs/DEVELOPMENT_ROADMAP.md](docs/DEVELOPMENT_ROADMAP.md)：按“个人桌面工具工作台”定位收缩后的当前开发顺序。
- [docs/GO_ENVIRONMENT.md](docs/GO_ENVIRONMENT.md)：Go 环境的用户视角说明、影响范围、下载任务、失败提示、默认路径、`<无 SDK>` 语义与验收清单。
- [docs/PYTHON_ENVIRONMENT.md](docs/PYTHON_ENVIRONMENT.md)：Python 环境的用户视角说明、基础 Python、托管虚拟环境、动态依赖扫描、一键安装与 `<无 Python>` 语义。
- [docs/RUST_ENVIRONMENT.md](docs/RUST_ENVIRONMENT.md)：Rust / Zig 环境的用户视角说明、模式切换、Rust 环境目录、托管下载、能力补齐、默认路径与状态栏语义。
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)：当前真实落地架构、模块边界、执行链路和约束。
- [docs/DEVELOPMENT_SETUP.md](docs/DEVELOPMENT_SETUP.md)：最新开发环境、启动方式、校验命令和入口文件。
- [docs/RUST_TOOL_INTEGRATION_STANDARD.md](docs/RUST_TOOL_INTEGRATION_STANDARD.md)：Rust 工具接入桌面宿主的目录、manifest、构建、导出、远程执行与验收标准。
- [docs/adr/0022-host-owned-background-artifact-preparation.md](docs/adr/0022-host-owned-background-artifact-preparation.md)：后台产物准备、缓存复用与产物中心工作区形态。
- [docs/adr/0024-rust-toolchain-managed-by-host.md](docs/adr/0024-rust-toolchain-managed-by-host.md)：Rust 环境管理采用“Rust 环境目录 + Zig SDK + 派生能力补齐”的当前决策。
- [docs/ISSUES_AND_REMEDIATION_PLAN.md](docs/ISSUES_AND_REMEDIATION_PLAN.md)：当前已确认的问题清单与修复顺序。
- [docs/archive/README.md](docs/archive/README.md)：历史计划、旧版说明和归档原因。
- [docs/adr/](docs/adr/)：仍有效的架构决策记录。

## 文档约定

- `README.md` 只做入口和导航，不承载历史方案。
- `docs/` 下默认只放当前有效文档；阶段性计划、复盘、旧版说明统一进入 `docs/archive/`。
- 若代码与文档冲突，以代码现状为准，并优先更新 `ARCHITECTURE.md`、`DEVELOPMENT_SETUP.md` 与 `ISSUES_AND_REMEDIATION_PLAN.md`。
