# 当前架构落地情况

本文档描述当前仓库已经落地的桌面化重构骨架，而不是理想态示意图。

## 当前目录

```text
my_tools/
├── app/                     # Wails v2 桌面宿主（Go module: fire-salamander-desktop）
│   ├── frontend/            # Vue 3 前端
│   ├── internal/            # 后端内部包
│   │   ├── ssh/             # SSH 连接存储
│   │   ├── runtime/         # SSH 远程执行器
│   │   └── builder/         # 工具构建打包
│   ├── main.go              # Wails 入口
│   ├── app.go               # App struct, bootstrap, SSH API
│   ├── execution.go         # 本地 + 远程执行 + 日志事件
│   ├── dialog.go            # Wails 原生文件对话框
│   ├── legacy.go            # 旧 tview 工具闭包捕获
│   ├── go.mod               # Go module
│   └── wails.json           # Wails 配置
├── libs/                    # 共享 Go 库
│   ├── core/toolspec/       # 工具规格核心类型
│   ├── catalog/builtin/     # 内置工具清单与加载器
│   └── framework/           # 旧工具框架
├── tools/                   # 工具实现
│   ├── go_tools/            # Go 原生工具
│   └── python_tools/        # Python 脚本工具
├── docs/                    # 文档 + ADR
├── scripts/
│   └── build.go             # 构建脚本
└── go.work                  # Go workspace
```

## 已落地的边界

- `app`
  - 当前桌面宿主入口
  - 负责 Wails 启动、窗口配置、前后端绑定
- `libs/core/toolspec`
  - 定义统一工具规格
  - 包含工具类型、参数字段、执行方式、导出方式等模型
- `libs/catalog/builtin`
  - 管理内置工具清单
  - 当前已通过嵌入式 YAML 提供 1 个 Go 样板工具和 1 个 Python 样板工具

## 当前实现说明

- 桌面宿主已经可编译打包。
- 前端已经替换为 `Vue 3 + TypeScript + Pinia + Vue Router + Naive UI + Tailwind CSS`。
- 首页不再是 Wails 默认示例，而是新的“工具主页”。
- 已新增“执行页面模式”，对应旧 TUI 的说明、输入、输出工作流的现代化改造。
- 前端位于 `app/frontend`，由 Wails v2 构建链直接管理。

## 当前工具清单策略

- 内置工具通过 `libs/catalog/builtin/manifests/*.yaml` 定义。
- `libs/catalog/builtin/service.go` 负责将 YAML 解析为 `libs/core/toolspec.ToolManifest`。
- 当前样板：
  - `utm_extract_to_gis.yaml`：Go 工具样板
  - `restore_pcd_by_mgrs.yaml`：Python 工具样板

## 当前桌面宿主能力

- 加载内置工具清单，向前端暴露工作台初始化数据
- **单页工作台范式**（IDE/Postman 风格）：左侧常驻侧边栏 + 右侧工作区
- **侧边栏多功能**：工具分类树（含收藏夹+最近使用） / SSH连接管理（含表单CRUD） / 历史占位 / 导出占位
- 支持多工具页签并行打开
- 支持"结构化表单 / 命令行模式 / 工具说明"三模式参数区
- **Wails 原生文件对话框**：path 类型参数带 📂 按钮，自动判断目录/文件选择
- 通过 Wails 事件流回传任务状态和日志
- 始终可见的执行终端面板（原生 overflow + 行号 + 关键词着色 + 导出）
- 已接通本地执行、取消执行、任务切换和日志输出
- 旧 Go/Python 工具不重写，直接复用原先挂给 TUI 的执行闭包
- **SSH 连接管理 + 远程执行**：本地加密存储，侧边栏CRUD，弹窗选择连接远程执行
- **收藏夹**：Ctrl+F 收藏/取消，侧边栏 ❤️ 分组显示，持久化到 localStorage
- **最近使用**：执行后自动记录，侧边栏 ⭐ 分组显示最后N个工具的参数字符串
- **命令历史/参数记忆**：每次打开工具自动带出上次参数，每工具独立历史去重
- **快捷键帮助**：F1 弹出分组着色快捷键速查表
- **系统设置**：⚙️ 模态弹窗，8项可配置（最近使用/历史保留数、日志导出目录、Python默认路径、退出确认、自动换行、展开分类、快捷键提示模式）+ 初始化应用
- **日志导出**：💾 按钮调用保存文件对话框导出 .log 文件
- **Builder 模块**：Go 工具复制可执行，Python 工具 tar.gz 打包

## 后端模块落地

```
app/
├── main.go                  # Wails 入口
├── app.go                   # App struct, bootstrap, SSH API
├── execution.go             # 本地 + 远程执行 + 日志事件
├── dialog.go                # Wails 原生文件对话框
├── legacy.go                # 旧 tview 工具闭包捕获
└── internal/
    ├── ssh/store.go         # SSH 连接模型 + JSON 存储 CRUD
    ├── runtime/remote.go    # SSH Dial/Upload/Execute
    └── builder/pack.go      # Go 可执行复制 + Python tar.gz 打包
```

## 前端组件架构

```
App.vue
└── WorkspaceLayout.vue                  (全局布局壳)
    ├── AppHeader.vue                    (顶部导航栏: Logo, 搜索, 快捷入口)
    ├── ToolSidebar.vue                  (左侧工具分类树 + 搜索 + 底部视图切换)
    ├── WorkspaceTabs.vue                (工具页签栏 + 活动工具内容)
    │   ├── ToolDetailPanel.vue          (工具名称, 说明, 运行/远程/导出按钮)
    │   ├── ParameterPanel.vue           (三模式: 可视化表单 | 命令行 | 工具说明)
    │   └── ExecutionTerminal.vue        (始终可见的日志终端)
    └── StatusBar.vue                    (底部状态栏: 版本信息)
```

## 不再需要的旧前端结构

- ~~`HomeView.vue`~~（功能已被 `ToolSidebar` + `ToolDetailPanel` 替代）
- ~~`ExecuteView.vue`~~（功能已被 `WorkspaceTabs` + `ParameterPanel` + `ExecutionTerminal` 替代）
- ~~`Vue Router`~~（已移除，单页工作台不需要路由）
- ~~`useToolArgs` composable~~（逻辑内化到 `ParameterPanel.vue` 和 workspace store）

## 下一阶段实现入口

优先顺序：

1. `app/internal/runtime/`
   - 远程 SSH 上传、执行、清理
   - 统一日志事件模型继续抽离
2. `app/internal/builder/`
   - Go 单工具构建缓存
   - Python 脚本产物整理
3. `libs/catalog/custom`
   - 复制为自定义工具
4. `app/frontend`
   - 远程执行 UI（`ParameterPanel` 内的远程配置区已有占位）
   - 执行页的拖拽输入、历史参数和更完整的结果面板

## 不要再做的事

- 不要继续扩展旧 `internal/tui`
- 不要再为旧 `Tool` 接口新增能力
- 不要把远程执行做成长期驻留代理
- 不要为了 UI 兼容去复刻旧 TUI 页面结构
- 不要再使用 Vue Router 做页面跳转（单页工作台已确立）
