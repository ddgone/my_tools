# 当前架构落地情况

本文档描述当前仓库已经落地的桌面化重构骨架，而不是理想态示意图。

## 当前目录

```text
/
├─ app/
│  └─ desktop/                # Wails v2 桌面宿主子模块
├─ catalog/
│  └─ builtin/               # 内置工具清单与加载器
├─ core/
│  └─ toolspec/              # 工具规格核心类型
├─ docs/
├─ runtime/                  # 预留
├─ builder/                  # 预留
├─ toolkits/                 # 预留
├─ go.mod                    # 根平台模块
└─ go.work                   # 根模块 + Wails 子模块工作区
```

## 已落地的边界

- `app/desktop`
  - 当前桌面宿主入口
  - 负责 Wails 启动、窗口配置、前后端绑定
- `core/toolspec`
  - 定义统一工具规格
  - 包含工具类型、参数字段、执行方式、导出方式等模型
- `catalog/builtin`
  - 管理内置工具清单
  - 当前已通过嵌入式 YAML 提供 1 个 Go 样板工具和 1 个 Python 样板工具

## 当前实现说明

- 桌面宿主已经可编译打包。
- 前端已经替换为 `Vue 3 + TypeScript + Pinia + Vue Router + Naive UI + Tailwind CSS`。
- 首页不再是 Wails 默认示例，而是新的“工具主页”。
- 已新增“执行页面模式”，对应旧 TUI 的说明、输入、输出工作流的现代化改造。
- 前端当前仍位于 `app/desktop/frontend`，这是为了优先保持 Wails v2 默认构建链稳定。
- `frontend/` 顶级目录仍保留为未来继续抽离前端边界时的扩展位。

## 当前工具清单策略

- 内置工具通过 `catalog/builtin/manifests/*.yaml` 定义。
- `catalog/builtin/service.go` 负责将 YAML 解析为 `core/toolspec.ToolManifest`。
- 当前样板：
  - `utm_extract_to_gis.yaml`：Go 工具样板
  - `restore_pcd_by_mgrs.yaml`：Python 工具样板

## 当前桌面宿主能力

- 加载内置工具清单
- 向前端暴露工作台初始化数据
- 显示“主页模式”与“执行页面模式”
- 支持“结构化表单 / 原始参数模式”双视图
- 通过 Wails 事件流回传任务状态和日志
- 已接通本地执行、取消执行、任务切换和日志输出
- 旧 Go/Python 工具不重写，直接复用原先挂给 TUI 的执行闭包

## 下一阶段实现入口

优先顺序：

1. `runtime/`
   - 远程 SSH 上传、执行、清理
   - 统一日志事件模型继续抽离
2. `builder/`
   - Go 单工具构建缓存
   - Python 脚本产物整理
3. `catalog/custom`
   - 复制为自定义工具
4. `app/desktop/frontend`
   - 远程执行 UI
   - 执行页的拖拽输入、历史参数和更完整的结果面板

## 不要再做的事

- 不要继续扩展旧 `internal/tui`
- 不要再为旧 `Tool` 接口新增能力
- 不要把远程执行做成长期驻留代理
- 不要为了 UI 兼容去复刻旧 TUI 页面结构
