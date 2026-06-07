# 工作区内置工具

本文档只描述当前已经落地的“工作区内置工具”能力，包括入口、边界、已实现工具和后续扩展方式。

## 1. 它是什么

工作区内置工具是由桌面宿主直接提供的一组轻量工具页面。

- 入口在活动栏的“内置工具”按钮。
- 左侧以搜索 + 卡片列表展示。
- 点击后在右侧工作区打开专属标签页。
- 它们运行在前端本地，不依赖 Go/Python 工具执行链路。

这类能力适合即时转换、解析、编解码、格式化、校验等高频研发小任务。

## 2. 它不是什么

工作区内置工具不是 `toolspec.ToolManifest` 驱动的普通工具，因此它们当前不具备以下能力：

- 不走 `app/execution.go` 的本地或远程执行任务链路。
- 不走 `app/export.go` 的导出链路。
- 不走 `app/internal/builder/` 的单工具产物准备链路。
- 不承诺运行日志终端、远程环境选择或导出入口。

如果一个能力需要远端环境、长时任务、流式日志、导出产物或脚本分发，它就不应先做成工作区内置工具。

## 3. 当前入口与结构

### 前端入口

- 活动栏入口：`app/frontend/src/components/ActivityBar.vue`
- 左侧面板：`app/frontend/src/components/BuiltinSidebarPanel.vue`
- 右侧容器：`app/frontend/src/components/BuiltinToolPanel.vue`
- 注册表：`app/frontend/src/builtin/registry.ts`
- 类型定义：`app/frontend/src/types/builtin.ts`
- 页面实现：`app/frontend/src/components/builtin/*.vue`

### 工作区集成

- 工作区状态统一由 `app/frontend/src/stores/workspace.ts` 管理。
- 工作区内置工具使用独立标签类型 `builtin`。
- 固定标签恢复沿用现有工作区标签机制，不单独发明第二套页签恢复系统。

## 4. 当前已实现工具

### 时间处理

- 支持以下格式互转：
  - Unix 秒
  - Unix 毫秒
  - ISO UTC
  - ISO 带偏移
  - 日期时间
  - 日期
- 支持源时区和目标时区。
- 支持同一时间点的派生格式总览。

### JSON 工具

- 格式化
- 压缩
- 稳定排序格式化
- 校验
- 输出复制与回填

### Base64 工具

- 文本编码为 Base64
- Base64 解码为文本
- Unicode 文本支持
- 输入输出互换与结果复制

### URL 工具

- URL 编码 / 解码
- URL Component 编码 / 解码
- 查询串拆解
- 输出复制与回填

### Hash 摘要

- 当前支持：
  - SHA-1
  - SHA-256
  - SHA-384
  - SHA-512
- 输出十六进制摘要，适合快速比对内容一致性。

### JWT 查看

- 解析 Header
- 解析 Payload
- 快速展示常见字段，如 `iss`、`sub`、`aud`、`iat`、`exp`
- 当前只做结构解析，不做签名校验

## 5. 设计约束

### 交互约束

- 不破坏现有普通工具、SSH、产物中心的交互逻辑。
- 不复用普通工具的参数表单、执行终端、远程切换和导出按钮。
- 内置工具之间尽量共享交互习惯，例如：
  - 顶部操作按钮风格一致
  - 左输入右输出或源格式/目标格式的结构一致
  - 常见动作提供复制、回填、交换等快捷入口

### 架构约束

- 工作区内置工具优先做成前端本地能力。
- 不为了“统一”而强行并入 `ToolManifest`、远程执行或导出链路。
- 只有当某个能力明确需要宿主 API、运行时文件系统、远端环境或安全边界时，才考虑把它提升为后端参与能力。

## 6. 何时应该新增一个工作区内置工具

适合新增为工作区内置工具的能力通常满足以下特征：

- 输入和输出都比较轻量，多为文本。
- 处理过程短平快，用户希望即时看到结果。
- 不依赖远程主机、外部解释器或长时后台任务。
- 更像“研发工作台小工具”，而不是需要独立分发的正式工具。

例如：

- 文本编解码
- 结构化内容格式化
- token / header / query 参数解析
- 时间与时区换算
- 摘要和轻量校验

## 7. 扩展方式

新增一个工作区内置工具时，当前推荐流程如下：

1. 在 `app/frontend/src/types/builtin.ts` 中补充 `BuiltinToolId` 和图标类型。
2. 在 `app/frontend/src/builtin/registry.ts` 中注册名称、说明、关键字、分组、徽标和图标。
3. 在 `app/frontend/src/components/builtin/` 下新增对应页面组件。
4. 在 `app/frontend/src/components/BuiltinToolPanel.vue` 中接入页面映射。
5. 若引入了新的交互术语或边界规则，同步更新 `CONTEXT.md`。
6. 若形成了难以逆转的架构决策，再补 ADR。

## 8. 决策来源

- 术语定义：[`CONTEXT.md`](../CONTEXT.md)
- 架构决策：[`docs/adr/0023-workspace-builtins-outside-tool-spec-pipeline.md`](./adr/0023-workspace-builtins-outside-tool-spec-pipeline.md)
- 总体架构：[`docs/ARCHITECTURE.md`](./ARCHITECTURE.md)
