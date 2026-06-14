# 当前架构

本文档只记录当前仓库已经真实落地的架构，不记录历史草案，也不把“计划中的能力”伪装成“已完成能力”。

## 1. 系统形态

- 宿主：Wails v2 桌面应用。
- 前端：Vue 3 + TypeScript + Pinia + Naive UI 的单页工作台。
- 后端：Go 宿主编排层，负责 bootstrap、执行、SSH、窗口状态和运行时目录。
- 工具实现：Go 工具、Python 脚本与 Rust 工具继续复用既有资产，通过统一工具规格和针对不同语言的执行/构建适配层接入。
- 工作区内置工具：由前端本地页面提供的轻量工具箱能力，独立于 Go/Python 工具执行链路。

## 2. 模块边界

### `app/`

负责桌面宿主本身，而不是工具实现细节。

- `main.go`：Wails 入口、窗口配置、前端资源加载。
- `app.go`：bootstrap、SSH CRUD、窗口状态读写、前端绑定 API。
- `execution.go`：本地执行、远程执行、事件推送、任务生命周期。
- `export.go`：单工具导出、最近导出目录、导出结果打开。
- `go_settings.go`：Go 环境状态、Go SDK 选择、官方 SDK 下载与禁用态桥接 API。
- `python_settings.go`：Python 环境状态、基础 Python 选择、托管虚拟环境创建/重建、依赖一键安装与禁用态桥接 API。
- `rust_settings.go`：Rust 环境状态、Rust / Zig 候选、托管下载、能力补齐与禁用态桥接 API。
- `rust_toolchain_task.go`：Rust / Zig 下载安装与能力补齐任务状态桥接。
- `rust_tools.go`：Rust 工具本地执行入口、本地产物解析与源码工作区下的本机构建兜底。
- `legacy.go`：旧工具注册闭包向新宿主的桥接。

### `app/internal/ssh/`

负责 SSH 连接的模型与本地持久化。当前是基于 JSON 文件的 CRUD 存储，不是加密凭据仓库。

### `app/internal/runtime/`

负责 SSH 连接、远端平台探测、上传、执行与清理，是远程执行链路的运行时实现。

### `app/internal/builder/`

负责为远程执行和单工具导出准备单工具产物。

- Go 工具：现场生成 wrapper 并编译。
- Python 工具：复制脚本到目标位置。
- Rust 工具：解析 crate 根目录，按目标平台调用 `cargo build` 或 `cargo zigbuild` 生成单工具产物。
- Go / Rust 工具链解析由 `app/internal/toolchain/` 提供，不再散落在 builder 内部的路径猜测逻辑中。

### `app/internal/toolchain/`

负责 Go / Python / Rust 环境发现、状态计算、依赖状态检查，以及 Go SDK 的下载安装、Python 托管虚拟环境创建、Rust / Zig 托管环境下载与能力补齐。

- 支持“已配置路径 / 历史已知路径 / PATH / 常见系统路径 / 托管 SDK”多来源检测。
- 支持 `disabled=true` 的显式关闭态；关闭后不会自动回退到可发现的 Go。
- 默认把托管 SDK 放在运行时目录下的 `toolchains/`。
- Python 当前不做托管下载，但已经管理“基础 Python + 托管 venv + 动态依赖扫描”。
- Rust 当前采用“模式 + Rust 环境目录 + Zig SDK + 派生能力补齐”模型；`cargo-zigbuild` 与常用 targets 不作为独立主 SDK 暴露，而是依附当前激活的 Rust 环境。
- Rust 托管下载当前支持官方源优先、镜像源回退、组件化安装（Rust / Zig / Rust + Zig），并允许多个托管版本共存。
- Rust 环境检测当前会优先使用托管 Rust 作为自动探测候选，避免系统 Rust 在设置页中抢占当前激活环境。
- Python 动态依赖扫描使用内置资源文件 `pythondata/mapping.txt` 与 `pythondata/stdlib.txt`；两者来自 `pipreqs` 数据，并由 Go 宿主通过 `go:embed` 内置到程序中。

### `app/internal/runtimeenv/`

负责统一运行时目录定位。开发态优先使用仓库内 `build/runtime/`，安装态回退到用户目录。

### `libs/core/toolspec/`

定义工具规格、参数、执行策略、远程策略和导出策略等平台级模型。

### `libs/catalog/builtin/`

加载内置 YAML manifest，并把静态工具描述交给宿主和前端消费。

### `tools/`

存放工具实现资产。新宿主优先复用这些现有能力，而不是重写一套新的工具实现。

## 3. 前端结构

前端不是多页面路由应用，而是单页工作台。

- 根组件：`app/frontend/src/App.vue`
- 工作台壳：`WorkspaceLayout.vue`
- 左侧导航：`ActivityBar.vue`、`ToolSidebar.vue`
- 主工作区：`WorkspaceTabs.vue`
- 工具详情与执行：`ToolDetailPanel.vue`、`ParameterPanel.vue`、`ExecutionTerminal.vue`
- 工作区内置工具：`src/builtin/registry.ts`、`BuiltinSidebarPanel.vue`、`BuiltinToolPanel.vue`、`src/components/builtin/*.vue`
- 宿主设置与环境提示：`SettingsModal.vue`、`StatusBar.vue`
- SSH 表单：`SSHDetailPanel.vue`
- 状态存储：`src/stores/workspace.ts`、`src/stores/goenv.ts`、`src/stores/pythonenv.ts`、`src/stores/rustenv.ts`

当前没有 `vue-router` 依赖，也不再维护 `HomeView` / `ExecuteView` 的旧结构。

### 工作区内置工具

- 入口位于活动栏的“内置工具”按钮，经 `BuiltinSidebarPanel.vue` 以搜索 + 卡片形式展示。
- 打开后在 `WorkspaceTabs.vue` 中以独立标签类型渲染，不复用普通工具的参数区、执行终端、远程执行或导出入口。
- 当前实现全部位于前端本地页面，状态由浏览器内存和现有工作区标签体系管理。
- 这条链路不消费 `toolspec.ToolManifest`，也不进入 `app/execution.go`、`app/export.go` 或 `app/internal/builder/`。
- 当前已落地的工具包括时间处理、JSON 工具、Base64 工具、URL 工具、Hash 摘要、JWT 查看。

## 4. 执行链路

### 本地执行

1. 前端提交工具参数。
2. 宿主根据工具 ID 找到桥接后的执行器。
3. `execution.go` 创建任务、推送状态事件、收集日志。
4. 前端通过 Wails 事件更新标签状态和执行终端。

本地执行不会现场调用 `go build`。对 Go 工具来说，它直接运行编译进宿主的 bridge 闭包，因此不依赖用户额外配置 Go 环境。

对 Python 工具来说，本地执行会先解析当前基础 Python 对应的托管虚拟环境，并检查该工具脚本动态扫描出的第三方依赖是否已经安装；若未就绪，则阻断当前操作并引导用户进入 Python 设置页。

对 Rust 工具来说，本地执行优先复用随宿主打包的本地产物；如果当前运行在源码工作区且未找到已打包产物，则会基于当前激活的 Rust / Zig 环境现场构建宿主平台产物，然后直接执行该二进制。

### 远程执行

1. 前端选择 SSH 连接并提交执行请求。
2. 宿主先解析当前所需环境；Go 工具需要可用 Go SDK，Rust 工具需要可用 Rust / Zig 交叉编译环境。
3. 宿主构建或准备单工具产物。
4. 远程执行器建立 SSH 会话、探测远端平台、上传文件。
5. 在远端临时目录执行工具并回传日志。
6. 执行结束后按结果探测规则判断输出文件或目录是否可下载。
7. 若用户触发“下载结果”，宿主会对单文件直接下载，对目录执行远端打包后下载，并通过下载任务抽屉反馈进度。
8. 在结果回收阶段结束后，再清理远端临时目录和本地任务状态；若结果需要保留供后续下载，远端工作目录会暂时保留。

当前仍待补的后续能力主要有：

- 远程模式下参数浏览按钮对应的远程路径选择器。
- 远程结果能力在轻量历史里的补一次下载入口与少量交互收尾。

## 5. 数据与状态边界

- 运行时配置：`build/runtime/config/` 或安装态用户目录。
- SSH 连接：由后端存储在运行时配置目录下。
- Go 环境配置：后端维护在运行时配置目录中的 `app.json`，当前稳定字段包括 `selectedBinary`、`knownBinaries`、`lastInstallDirectory`、`disabled`。
- Python 环境配置：后端同样维护在运行时配置目录中的 `app.json`，当前稳定字段包括 `selectedBinary`、`knownBinaries`、`disabled`；托管虚拟环境元数据落在运行时目录 `toolchains/python/` 下。
- Rust 环境配置：后端维护在运行时配置目录中的 `app.json`，当前稳定字段包括 `mode`、`selectedRustRoot`、`knownRustRoots`、`selectedZigBinary`、`knownZigBinaries`、`lastInstallDirectory`、`disabled`；托管 Rust / Zig 元数据落在运行时目录 `toolchains/rust/` 与 `toolchains/zig/` 下。
- 前端偏好：收藏夹、最近使用、参数历史、工具参数快照、导出目标平台、设置页签、固定标签与标签顺序等当前主要在 `localStorage`。
- 工作区内置工具：当前不做独立持久化数据存储，但可通过现有固定标签机制恢复其页签入口。
- 任务日志：事件流为主，日志导出是当前已完成的持久化出口。
- 单工具导出：最近导出目录由后端配置文件维护；导出模式、是否自动打开目录、工具级导出目标平台由前端偏好维护。

## 6. 当前已知约束

- 远程执行已可用，但 SSH 安全与认证链路尚未完全收口。
- 单工具导出与产物中心已落地，但远程执行后的结果下载仍未形成闭环。
- 构建链路对仓库目录和部分默认环境仍有依赖，但 Go SDK 默认落点已统一到运行时目录下的 `toolchains/`。
- 部分设置项已经暴露到 UI，但仍有历史选项尚未接入实际业务逻辑。
- Go 环境当前只影响远程执行、Go 导出和构建缓存，不影响 Go 工具的本地执行。
- Python 环境当前直接影响 Python 工具的本地执行；当前通过托管虚拟环境隔离依赖，但仍不提供托管 Python 下载。
- Rust 环境当前影响 Rust 工具的本机现场构建、远程执行前的单工具构建、Rust 导出与构建缓存准备；系统 Rust 的 `cargo-zigbuild` / targets 补齐默认受保护，不再直接由宿主无提示修改。
- 工作区内置工具当前运行在前端本地，适合即时转换、编解码、解析和校验类能力，不适合需要远端环境、长任务或导出产物的工具。
- SSH 连接管理已落地，但仍缺独立 SSH 终端页签。
- 路径参数当前只有 `pathMode` 级别的信息，尚不足以完整表达远程工作路径与本地接收路径的差异。

## 7. 文档维护规则

- 若代码与本文冲突，先以代码现状为准，再立即修订本文。
- 若新增的是术语定义，更新 `CONTEXT.md`。
- 若新增的是难以逆转的架构决策，再考虑新增 ADR。
- 历史方案、阶段计划、过程复盘不得回流到本文。

如果需要从用户视角理解 Go 环境，而不是从架构视角理解它，直接看 [GO_ENVIRONMENT.md](./GO_ENVIRONMENT.md)。
如果需要从用户视角理解 Python 环境，而不是从架构视角理解它，直接看 [PYTHON_ENVIRONMENT.md](./PYTHON_ENVIRONMENT.md)。
如果需要从用户视角理解 Rust / Zig 环境，而不是从架构视角理解它，直接看 [RUST_ENVIRONMENT.md](./RUST_ENVIRONMENT.md)。
如果需要从实现视角理解 Rust 工具如何接入宿主，直接看 [RUST_TOOL_INTEGRATION_STANDARD.md](./RUST_TOOL_INTEGRATION_STANDARD.md)。
如果需要从用户和实现视角理解工作区内置工具，直接看 [WORKSPACE_BUILTINS.md](./WORKSPACE_BUILTINS.md)。
