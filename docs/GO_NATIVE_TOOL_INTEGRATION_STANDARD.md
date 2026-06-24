# Go 原生工具接入标准（新模型）

本文档定义当前仓库里 Go 原生工具接入桌面宿主的统一标准。

旧模型（`framework.Tool` 接口 + `init()` 注册 + `framework.ParseArgs`）已完全退役。当前所有 Go 工具均已迁移至纯库包模型。

参考工具：
- `tools/go_tools/echo_tool/tool.go`（最小示例）
- `tools/go_tools/recursive_content_dir_diff/tool.go`（复杂参数示例）

## 1. 目标

一个新的 Go 原生工具要被桌面宿主正确接入，必须同时满足以下 3 件事：

1. 工具包是纯库包，暴露唯一入口 `Run(ctx, args, out)`。
2. 内置 manifest 提供名称、分类、参数、执行策略和源码入口。
3. 代码和 manifest 一致后，本地执行、远程执行和导出链路自动贯通。

与旧模型不同：不再需要实现任何框架接口、不再需要在 `init()` 里注册、不再需要在 `app/legacy.go` 里匿名导入。

## 2. 接入模型总览

### 2.1 工具代码层

Go 原生工具是 `tools/go_tools/<tool_id>/` 下的纯库包。唯一约定是：

```go
func Run(ctx context.Context, args []string, out io.Writer) error
```

没有 `func main()`，没有框架 import，没有任何注册副作用。

### 2.2 Manifest 层

`libs/catalog/builtin/manifests/<tool_id>.yaml` 是工具的**唯一元数据来源**，供前端、本地执行、远程执行和导出使用：

- 基本信息：`id`、`name`、`kind`、`category`、`description`
- 文档信息：`docs.summary`、`docs.usage`
- 参数表单：`params`
- 执行策略：`execution`
- 导出策略：`export`
- 源码入口：`source.entry`

### 2.3 构建与分发层

- **Builder**（`app/internal/builder/`）根据 `source.entry` 生成临时 `main.go` wrapper，调用 `<pkg>.Run(ctx, os.Args[1:], io.Stdout)`，编译为单工具二进制。
- **build.go**（`scripts/build.go`）在桌面宿主构建时，将所有 Go 工具编译为预置二进制，放入 `assets/go/<os>_<arch>/`，随宿主分发。

### 2.4 本地执行与远程执行

- **本地执行**：优先使用 `assets/go/<os>_<arch>/<tool_id>` 下的预置二进制；开发模式下如果找不到，会现场从源码构建。
- **远程执行**：根据 manifest 的 `source.entry` 临时构建单工具产物，上传到远端执行。

本地执行不依赖用户在设置页额外配置 Go 环境，因为预置二进制已经编译好了。Go 环境仅影响远程执行、导出和构建缓存。

## 3. 目录与文件标准

新的 Go 原生工具应放在：

```text
tools/go_tools/<tool_id>/tool.go
```

一个工具一个独立目录，`tool.go` 作为清晰入口。若还有其它业务文件，可与 `tool.go` 同目录组织。

最低要求：
- 必须存在一个可导入的 Go package
- 必须暴露 `func Run(ctx context.Context, args []string, out io.Writer) error`

**不需要**：
- 实现任何接口
- 在 `init()` 里注册
- 定义 `func main()`
- 导入框架包

## 4. `Run` 函数编写标准

### 4.1 最小骨架

```go
package example_tool

import (
	"context"
	"flag"
	"fmt"
	"io"
)

// Run 是工具的唯一入口，由 builder 生成的 wrapper 调用。
func Run(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("example_tool", flag.ContinueOnError)
	fs.SetOutput(out)

	var input string
	var output string

	fs.StringVar(&input, "input", "", "输入路径")
	fs.StringVar(&output, "output", "", "输出路径（可选，默认自动生成）")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if input == "" {
		return fmt.Errorf("错误：必须指定 -input 参数")
	}
	if output == "" {
		output = input + "_out"
	}

	// 业务逻辑...
	fmt.Fprintf(out, "处理完成: %s -> %s\n", input, output)
	return nil
}
```

### 4.2 参数解析必须使用 `flag.FlagSet` 直接解析

新模型下 `args` 已经是 `[]string`，直接传给 `flag.FlagSet.Parse()` 即可。不再需要 `framework.ParseArgs`。

```go
fs := flag.NewFlagSet("tool_id", flag.ContinueOnError)
fs.SetOutput(out)

var input string
fs.StringVar(&input, "input", "", "")

if err := fs.Parse(args); err != nil {
	return err
}
```

关键约束：

- `flag.FlagSet` 的输出必须写到 `out`（`fs.SetOutput(out)`），这样参数错误会进入宿主日志面板。
- 使用 `flag.ContinueOnError` 而非 `flag.ExitOnError`，避免内部调用 `os.Exit()`。

### 4.3 必须返回 `error`

不要在工具内部直接 `os.Exit()`。宿主通过返回值判断任务是 `success`、`error` 还是 `canceled`。

### 4.4 必须接受 `context.Context`

长循环、批处理、并发 worker 应检查 `ctx.Done()`，支持取消：

```go
select {
case <-ctx.Done():
	return ctx.Err()
default:
}
```

如果不检查 `ctx.Done()`，取消体验会很差。

### 4.5 默认值必须在 Go 端可独立成立

不要把默认值只写在前端表单或 manifest 中。正确做法是：

- 前端默认值可以有
- manifest 默认值可以有
- Go 工具本身也必须能在参数缺失时补齐默认行为

原因是本地执行、远程执行和导出后的二进制都是直接调用工具，不经过前端。

### 4.6 模式互斥必须在 Go 端校验

如果工具支持多种互斥模式（如 `-input`、`-convert`、`-merge`），必须在 Go 端做校验，不能依赖前端避免非法组合。

## 5. Manifest 标准

每个 Go 原生工具必须有一个对应的 YAML manifest，放在：

```text
libs/catalog/builtin/manifests/<tool_id>.yaml
```

### 5.1 最小完整模板

```yaml
id: example_tool
name: 示例工具
kind: go
category: 示例分类 > 子类
icon: go
description: 一句话说明工具用途。
docs:
  summary: 用于列表页和详情区的短说明。
  usage: |
    完整的帮助文本，可作为原始参数模式的说明。
params:
  - key: input
    argKey: input
    type: path
    label: 输入路径
    required: true
    placeholder: 选择输入文件或目录
  - key: output
    argKey: output
    type: path
    label: 输出路径
execution:
  local:
    adapter: go-binary
  remote:
    strategy: upload-binary-and-run
export:
  strategy: export-binary
source:
  entry: tools/go_tools/example_tool/tool.go
```

### 5.2 字段约束

#### `id`

必须全局唯一。作为 tool_id 用于查找、缓存和执行。

#### `kind`

Go 原生工具固定写 `go`。

#### `category`

使用 `A > B > C` 格式的分层路径，宿主会拆分成分类数组。

#### `params`

定义桌面前端结构化表单。需要与 Go 代码里的 flag 保持一致：

| manifest `type` | 对应 |
|---|---|
| `path` | 路径参数，前端提供文件选择器 |
| `string` | 文本参数 |
| `number` | 整数参数 |
| `boolean` | 布尔开关 |
| `select` | 固定枚举值（需搭配 `options`） |
| `textarea` | 多行文本 |

每个参数的 `argKey` 必须与 Go 代码中的 flag 名一致（如 `argKey: input` 对应 `-input`）。

manifest 不会自动验证 Go 代码里的 flag 是否真的存在，必须人工保持一致。

#### `params` 的可选字段

- `pathMode: directory`：路径参数限定为目录
- `default`：默认值（仅前端表单使用，不能替代 Go 端默认值逻辑）
- `placeholder`：输入提示，只放最短示例
- `help`：复杂字段的多行说明
- `required: true`：必填

#### `execution.local.adapter`

Go 原生工具固定为 `go-binary`。

#### `execution.remote.strategy`

固定为 `upload-binary-and-run`。

#### `export.strategy`

固定为 `export-binary`。支持导出二进制和导出源码。

#### `source.entry`

**最关键的字段之一**。必须填写源码入口文件的相对路径（相对于仓库根目录）：

```yaml
source:
  entry: tools/go_tools/example_tool/tool.go
```

约束：
- 路径必须是相对仓库根目录的路径
- 必须指向该工具包中的 `.go` 文件
- 该路径所在目录必须能被推导为合法 Go import path

Builder 会将其转换为 `my_tools/<source.entry 所在目录>`，再生成 wrapper 编译。

如果 `source.entry` 写错，远程执行一定会在构建阶段失败。

### 5.3 参数类型示例

#### 路径参数

```yaml
- key: input
  argKey: input
  type: path
  pathMode: directory
  label: 输入目录
  required: true
  placeholder: 选择输入目录
```

#### 文本参数

```yaml
- key: label
  argKey: label
  type: string
  label: 标签
  default: echo
```

#### 数字参数

```yaml
- key: workers
  argKey: workers
  type: number
  label: 并发数
  default: 4
  help: 默认 4；可按机器性能增减。
```

#### 布尔参数

```yaml
- key: noProgress
  argKey: no-progress
  type: boolean
  label: 禁用进度输出
  default: false
```

#### 多行文本

```yaml
- key: ignoreRules
  argKey: ignore
  type: textarea
  label: 忽略规则
  placeholder: |
    logs
    cache
  help: |
    支持逗号、分号或换行分隔多条规则。
```

## 6. Wrapper 与构建链路

### 6.1 Builder 生成的 wrapper

Builder 根据 manifest 的 `source.entry` 推导 import path，生成如下 wrapper：

```go
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"my_tools/tools/go_tools/example_tool"
)

func main() {
	err := example_tool.Run(context.Background(), os.Args[1:], io.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

然后用 `CGO_ENABLED=0 GOOS=... GOARCH=... go build` 编译为单工具二进制。

### 6.2 build.go 预置构建

`scripts/build.go` 中的 `buildBundledGoTools` 在桌面宿主构建时：

1. 遍历 `bundledGoTools` 列表
2. 为每个工具生成 wrapper（同 Builder 逻辑）
3. 编译为二进制，输出到 `assets/go/<os>_<arch>/<tool_id>[.exe]`
4. 产物随宿主分发

这意味着用户安装桌面宿主后，Go 工具即可直接本地执行，无需安装 Go SDK。

### 6.3 `bundledGoTools` 注册

在 `scripts/build.go` 中每新增一个工具，需要在 `bundledGoTools` 列表中添加：

```go
var bundledGoTools = []goTool{
	{ID: "example_tool", SourceEntry: "tools/go_tools/example_tool/tool.go"},
	// ...其他工具
}
```

如果忘记添加，本地执行时宿主会尝试从源码现场构建（需要 Go 环境），而非使用预置二进制。

## 7. 新增 Go 原生工具的标准步骤

1. 在 `tools/go_tools/<tool_id>/` 新建工具包，至少包含 `tool.go`。
2. 实现 `func Run(ctx context.Context, args []string, out io.Writer) error`。
3. 用 `flag.FlagSet` 解析参数，不在内部调用 `os.Exit()`。
4. 在 `libs/catalog/builtin/manifests/` 新增对应的 `<tool_id>.yaml`。
5. 确保 manifest 中 `kind: go`、`source.entry` 正确、`params` 与 Go flag 语义一致。
6. 在 `scripts/build.go` 的 `bundledGoTools` 列表中添加该工具。
7. 启动宿主，确认工具出现在工作台列表中。
8. 分别验证本地执行和远程执行。

## 8. 最低验收清单

### 8.1 发现与展示

- 宿主启动后工具出现在工作台列表中
- 名称、分类、说明与 manifest 一致
- 参数表单能正常渲染

### 8.2 本地执行

- 必填参数缺失时能返回清晰错误
- 默认值逻辑按预期生效
- 正常输入时可跑通主流程
- 日志写入任务输出面板

### 8.3 远程执行

- 能成功构建单工具产物
- 能上传到远端并执行
- 参数在远端行为与本地一致

### 8.4 代码约束

- 工具包只有 `Run` 作为入口，无 `func main()`
- 无框架 import（`framework`、`AppContext` 等）
- 无 `init()` 注册
- 未使用 `os.Exit()` 破坏宿主任务生命周期
- `flag.FlagSet.SetOutput(out)` 已设置

## 9. 常见接入错误

### 忘记在 `bundledGoTools` 中注册

表现：
- 工具目录和 manifest 都存在
- 工作台里能看到工具
- 本地执行可能仍然成功（现场构建），但启动时没有预置二进制

根因：
- `scripts/build.go` 的 `bundledGoTools` 列表中缺少该工具

### manifest `source.entry` 写错

表现：
- 本地执行可能仍然成功
- 远程执行在构建阶段失败

根因：
- Builder 无法生成正确的 import path，或无法找到源码入口

### Go flag 与 manifest `params` 不一致

表现：
- 表单提交后参数行为不对
- 原始参数模式和结构化表单结果不一致

根因：
- manifest 只是描述，不会自动同步到 Go flag 定义。`argKey` 必须与 flag 名一致。

### 核心逻辑内部调用 `os.Exit()`

表现：
- 参数错误或业务失败时宿主直接退出
- 任务面板无法显示错误信息

根因：
- 正确做法是 `return err`，让宿主根据返回值判断状态。

## 10. 推荐模板

### 工具代码模板

```go
package example_tool

import (
	"context"
	"flag"
	"fmt"
	"io"
)

// Run 是工具的唯一入口，由 builder 生成的 wrapper 调用。
func Run(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("example_tool", flag.ContinueOnError)
	fs.SetOutput(out)

	var input string
	var output string

	fs.StringVar(&input, "input", "", "输入路径")
	fs.StringVar(&output, "output", "", "输出路径（可选）")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if input == "" {
		return fmt.Errorf("错误：必须指定 -input 参数")
	}
	if output == "" {
		output = input + "_out"
	}

	// 检查取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 业务逻辑...
	fmt.Fprintf(out, "处理完成: %s -> %s\n", input, output)
	return nil
}
```

### Manifest 模板

```yaml
id: example_tool
name: 示例工具
kind: go
category: 示例分类 > 子类
icon: go
description: 一句话说明工具用途。
docs:
  summary: 短说明。
  usage: |
    完整的帮助文本。
params:
  - key: input
    argKey: input
    type: path
    label: 输入路径
    required: true
  - key: output
    argKey: output
    type: path
    label: 输出路径
execution:
  local:
    adapter: go-binary
  remote:
    strategy: upload-binary-and-run
export:
  strategy: export-binary
source:
  entry: tools/go_tools/example_tool/tool.go
```

### `bundledGoTools` 注册

```go
var bundledGoTools = []goTool{
	// ...已有工具
	{ID: "example_tool", SourceEntry: "tools/go_tools/example_tool/tool.go"},
}
```

## 11. 一句话结论

在当前仓库里，Go 原生工具的“完整接入”就是同时完成：

- 纯库包的 `Run(ctx, args, out)` 入口
- 内置 manifest（含正确的 `source.entry`）
- `scripts/build.go` 的 `bundledGoTools` 注册

不再需要任何框架接口、注册副作用或匿名导入。manifest 是元数据的唯一来源，Build 链路自动打通本地、远程和导出。
