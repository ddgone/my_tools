# 当前架构

本文档只记录当前仓库已经真实落地的架构，不记录历史草案，也不把“计划中的能力”伪装成“已完成能力”。

## 1. 系统形态

- 宿主：Wails v2 桌面应用。
- 前端：Vue 3 + TypeScript + Pinia + Naive UI 的单页工作台。
- 后端：Go 宿主编排层，负责 bootstrap、执行、SSH、窗口状态和运行时目录。
- 工具实现：Go 工具与 Python 脚本继续复用既有资产，通过统一工具规格和桥接层接入。

## 2. 模块边界

### `app/`

负责桌面宿主本身，而不是工具实现细节。

- `main.go`：Wails 入口、窗口配置、前端资源加载。
- `app.go`：bootstrap、SSH CRUD、窗口状态读写、前端绑定 API。
- `execution.go`：本地执行、远程执行、事件推送、任务生命周期。
- `legacy.go`：旧工具注册闭包向新宿主的桥接。

### `app/internal/ssh/`

负责 SSH 连接的模型与本地持久化。当前是基于 JSON 文件的 CRUD 存储，不是加密凭据仓库。

### `app/internal/runtime/`

负责 SSH 连接、远端平台探测、上传、执行与清理，是远程执行链路的运行时实现。

### `app/internal/builder/`

负责为远程执行和后续导出准备单工具产物。

- Go 工具：现场生成 wrapper 并编译。
- Python 工具：复制脚本到目标位置。

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
- SSH 表单：`SSHDetailPanel.vue`
- 状态存储：`src/stores/workspace.ts`

当前没有 `vue-router` 依赖，也不再维护 `HomeView` / `ExecuteView` 的旧结构。

## 4. 执行链路

### 本地执行

1. 前端提交工具参数。
2. 宿主根据工具 ID 找到桥接后的执行器。
3. `execution.go` 创建任务、推送状态事件、收集日志。
4. 前端通过 Wails 事件更新标签状态和执行终端。

### 远程执行

1. 前端选择 SSH 连接并提交执行请求。
2. 宿主构建或准备单工具产物。
3. 远程执行器建立 SSH 会话、探测远端平台、上传文件。
4. 在远端临时目录执行工具并回传日志。
5. 执行结束后清理远端临时目录和本地任务状态。

## 5. 数据与状态边界

- 运行时配置：`build/runtime/config/` 或安装态用户目录。
- SSH 连接：由后端存储在运行时配置目录下。
- 前端偏好：收藏夹、最近使用、参数历史等主要在 `localStorage`。
- 任务日志：事件流为主，日志导出是当前已完成的持久化出口。

## 6. 当前已知约束

- 远程执行已可用，但 SSH 安全与认证链路尚未完全收口。
- 单工具导出仍未闭环，Builder 现在主要服务远程执行。
- 构建链路对仓库目录和部分默认环境仍有依赖。
- 部分设置项已经暴露到 UI，但尚未全部接入实际业务逻辑。

## 7. 文档维护规则

- 若代码与本文冲突，先以代码现状为准，再立即修订本文。
- 若新增的是术语定义，更新 `CONTEXT.md`。
- 若新增的是难以逆转的架构决策，再考虑新增 ADR。
- 历史方案、阶段计划、过程复盘不得回流到本文。
