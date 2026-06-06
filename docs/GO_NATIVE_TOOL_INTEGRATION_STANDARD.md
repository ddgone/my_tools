# Go 原生工具完整接入标准

本文档定义当前仓库里 Go 原生工具接入桌面宿主的完整标准。

范围只覆盖当前已经真实落地的接入方式，不讨论未来可能存在但尚未闭环的理想方案。本文以现有 3 个 Go 原生工具为准：

- `tools/go_tools/geojson_to_shapefile/tool.go`
- `tools/go_tools/pos_trajectory_to_gis/tool.go`
- `tools/go_tools/utm_extract_to_gis/tool.go`

## 1. 目标

一个新的 Go 原生工具要被桌面宿主正确接入，必须同时满足以下 4 件事：

1. 工具包本身能作为旧 `framework.Tool` 被注册。
2. 宿主能通过 bridge 捕获到 `ShowTerminal` 暴露的运行闭包。
3. 内置 manifest 能为前端提供名称、参数、说明、执行策略和源码入口。
4. 远程执行链路能依据 manifest 的 `source.entry` 构建单工具产物。

只完成其中一部分都不算“完整接入”。

## 2. 当前真实接入模型

当前仓库采用的是“旧工具桥接 + manifest 补充元数据”的双轨模型。

### 2.1 工具实现层

Go 原生工具继续实现 `libs/framework/framework.go` 中定义的 `framework.Tool`：

```go
type Tool interface {
	ID() string
	Name() string
	Category() string
	Execute(ctx AppContext)
}
```

工具并不是直接实现桌面宿主的新接口，而是沿用旧模型，在 `Execute()` 中调用：

- `ctx.ShowTerminal(...)`：Go 工具执行入口
- `ctx.ShowPythonTerminal(...)`：Python 工具执行入口

对 Go 原生工具来说，必须使用 `ShowTerminal(...)`。

### 2.2 宿主桥接层

`app/legacy.go` 会遍历 `framework.Registry`，执行每个工具的 `Execute()`，并通过一个假的 `AppContext` 把以下信息捕获出来：

- 标题
- usage 文本
- `run func(ctx context.Context, args string, out io.Writer) error`

也就是说，桌面宿主并不会主动分析你的业务逻辑，而是依赖工具在 `Execute()` 里通过 `ShowTerminal()` 暴露的运行闭包。

### 2.3 Manifest 层

`libs/catalog/builtin/manifests/*.yaml` 提供结构化元数据，供前端和远程执行使用：

- 基本信息：`id`、`name`、`category`、`description`
- 文档信息：`docs.summary`、`docs.usage`
- 参数表单：`params`
- 执行策略：`execution`
- 导出策略：`export`
- 源码入口：`source.entry`

对于 Go 原生工具，`source.entry` 不是可选增强项，而是远程执行构建链路的必要输入。

### 2.4 本地执行与远程执行的差异

当前实现里：

- 本地执行：直接调用 bridge 捕获到的 `run` 闭包，不重新编译
- 远程执行：根据 manifest 的 `source.entry` 临时生成 wrapper，再交给 Go 工具链编译成单工具产物

因此，“本地能跑”不代表“完整接入成功”。如果缺少 manifest 或 `source.entry` 错误，远程执行会失败。

同时要注意一个容易误解的点：

- 本地执行不依赖用户在设置页里额外配置 Go 环境
- 用户侧 Go 环境只影响远程执行、Go 单工具导出和构建缓存准备
- 当用户显式选择 `<无 SDK>` 时，宿主不会再自动回退到 PATH 中的 Go

## 3. 必须满足的接入标准

## 3.1 目录与文件标准

新的 Go 原生工具应放在：

```text
tools/go_tools/<tool_dir>/tool.go
```

建议一个工具一个独立目录，`tool.go` 作为清晰入口。若还有其它业务文件，可与 `tool.go` 同目录组织。

最低要求：

- 必须存在一个可导入的 Go package
- 必须存在一个实现 `framework.Tool` 的类型
- 必须在该包的 `init()` 中调用 `framework.Register(...)`

## 3.2 `framework.Tool` 实现标准

工具类型必须实现以下 4 个方法：

```go
func (t *YourTool) ID() string
func (t *YourTool) Name() string
func (t *YourTool) Category() string
func (t *YourTool) Execute(ctx framework.AppContext)
```

约束如下：

- `ID()`：必须全局唯一，且要与 manifest 中的 `id` 完全一致
- `Name()`：用户可见名称，应直接用于前端展示
- `Category()`：使用 `A > B > C` 形式的分层路径，宿主会拆分成分类数组
- `Execute()`：必须通过 `ctx.ShowTerminal(...)` 暴露实际运行入口

不允许的做法：

- 只实现业务函数，不注册为 `framework.Tool`
- `ID()` 与 manifest `id` 不一致
- `Execute()` 中不调用 `ShowTerminal()`
- 依赖交互式 `PromptInput()` 才能运行核心流程

当前桌面宿主的主路径是“结构化表单/原始参数 -> 直接执行”，而不是旧 TUI 的多轮交互式输入。

## 3.3 `Execute()` 编写标准

Go 原生工具的标准写法是：

1. 准备一段 usage 文本。
2. 调用 `ctx.ShowTerminal(name, usage, runFunc)`。
3. 在 `runFunc` 中完成参数解析、默认值补齐、模式校验和业务执行。

参考骨架如下：

```go
type ExampleTool struct{}

func (t *ExampleTool) ID() string       { return "example_tool" }
func (t *ExampleTool) Name() string     { return "示例工具" }
func (t *ExampleTool) Category() string { return "示例分类 > 子类" }

func (t *ExampleTool) Execute(ctx framework.AppContext) {
	usage := `这里写工具说明、参数解释和示例`

	ctx.ShowTerminal(t.Name(), usage, func(runCtx context.Context, args string, out io.Writer) error {
		parsedArgs, err := framework.ParseArgs(args)
		if err != nil {
			return err
		}

		fs := flag.NewFlagSet("example_tool", flag.ContinueOnError)
		fs.SetOutput(out)

		var input string
		var output string

		fs.StringVar(&input, "input", "", "")
		fs.StringVar(&output, "output", "", "")

		if err := fs.Parse(parsedArgs); err != nil {
			return err
		}
		if input == "" {
			return fmt.Errorf("错误：必须指定 -input 参数")
		}
		if output == "" {
			output = filepath.Join(filepath.Dir(input), "output")
		}

		return runBusiness(runCtx, input, output, out)
	})
}

func init() {
	framework.Register(&ExampleTool{})
}
```

其中有几个关键约束。

### 参数解析必须使用 `framework.ParseArgs`

桌面宿主当前传入的是一整段参数字符串，而不是 `[]string`。因此在 `flag.FlagSet` 前，必须先执行：

```go
parsedArgs, err := framework.ParseArgs(args)
if err != nil {
	return err
}
```

否则带空格的路径、引号参数和命令行模式都可能解析错误。

当前 `framework.ParseArgs` 会严格校验参数字符串：

- 支持单双引号
- 支持空字符串值（如 `-input ""`）
- 支持常见转义（如 `\"`、`\\`、`\ `）
- 对未闭合引号或不完整转义直接返回错误

这意味着工具作者不需要自己再写一套命令行拆分逻辑，只要正确接收并返回这个错误即可。

### `FlagSet` 输出必须写到 `out`

标准写法：

```go
fs := flag.NewFlagSet("example", flag.ContinueOnError)
fs.SetOutput(out)
```

这样参数错误会进入宿主日志面板，而不是丢到不可见的标准错误流里。

### 运行闭包必须返回 `error`

不要在工具内部直接 `os.Exit()`。宿主通过返回值来判断任务是：

- `success`
- `error`
- `canceled`

如果工具内部直接退出进程，会破坏任务生命周期管理。

### 业务函数必须接受 `context.Context`

推荐把核心业务做成：

```go
func runBusiness(ctx context.Context, ..., out io.Writer) error
```

原因：

- 本地执行支持取消
- 远程单工具 wrapper 也会直接调用同一条 `run` 闭包
- 后续如果需要增强中断能力，`context.Context` 是唯一稳定接口

如果你的长循环、批处理、并发 worker 完全不检查 `ctx.Done()`，那工具虽然“能运行”，但取消体验会很差。

## 3.4 注册标准

每个工具包必须在 `init()` 中注册：

```go
func init() {
	framework.Register(&ExampleTool{})
}
```

这是当前仓库的硬约束，因为 `framework.Registry` 是宿主发现旧工具的唯一入口。

仅仅写了 `init()` 还不够，宿主进程还必须实际导入该包。

## 3.5 宿主导入标准

新增 Go 原生工具后，必须在 `app/legacy.go` 中添加匿名导入：

```go
import (
	_ "my_tools/tools/go_tools/example_tool"
)
```

这是当前接入里最容易漏掉的一步。

如果忘记导入：

- `init()` 不会执行
- `framework.Registry` 不会出现该工具
- 本地执行和远程执行都找不到该工具

因此，**“工具目录已存在”不等于“工具已接入”**。

## 3.6 Manifest 标准

每个 Go 原生工具必须有一个对应的 YAML manifest，放在：

```text
libs/catalog/builtin/manifests/<tool_id>.yaml
```

最小完整模板如下：

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
    这里写桌面宿主里的使用说明。
params:
  - key: input
    argKey: input
    type: path
    label: 输入路径
    required: true
    help: 对应命令行的 -input。
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

字段约束如下。

### `id`

必须与 `tool.ID()` 完全一致。

### `kind`

Go 原生工具固定写 `go`。

### `category`

建议与 `tool.Category()` 保持同义，使用 `A > B > C` 格式。

### `params`

这里定义的是桌面前端结构化表单，不是 Go 代码里的 flag 定义。

因此你需要自己保证两边语义对齐：

- `argKey: input` 对应命令行 `-input`
- `type: number` 对应整数参数
- `type: boolean` 对应布尔开关
- `type: select` 对应固定枚举值

manifest 不会自动验证你 Go 代码里的 flag 是否真的存在，所以这部分必须人工保持一致。

### `execution.local.adapter`

当前 Go 原生工具固定为：

```yaml
local:
  adapter: go-binary
```

注意，这里的名字代表工具类型，而不是本地执行一定会先编译二进制。当前实际本地执行仍然是 bridge 直调 `run` 闭包。

### `execution.remote.strategy`

当前 Go 原生工具固定为：

```yaml
remote:
  strategy: upload-binary-and-run
```

### `export.strategy`

当前 Go 原生工具固定为：

```yaml
export:
  strategy: export-binary
```

当前单工具导出链路已经接通：

- Go 工具支持导出二进制
- Go 工具支持导出源码
- 导出目标平台由工作台记录为工具级偏好

因此 manifest 里的导出策略不再是“提前预留”，而是会直接影响工作台里的真实导出行为。

### `source.entry`

这是 Go 原生工具最关键的字段之一，必须填写源码入口，例如：

```yaml
source:
  entry: tools/go_tools/example_tool/tool.go
```

约束如下：

- 路径必须是相对仓库根目录的路径
- 必须指向该工具包中的 `.go` 文件
- 该路径所在目录必须能被推导为合法 import path

当前 Builder 会把它转换成：

```text
my_tools/<source.entry 所在目录>
```

例如：

- `tools/go_tools/geojson_to_shapefile/tool.go`
- `tools/go_tools/pos_trajectory_to_gis/tool.go`
- `tools/go_tools/utm_extract_to_gis/tool.go`

都会被推导成对应的 Go import path，再生成临时 wrapper 编译。

如果 `source.entry` 写错：

- 本地执行可能仍然成功
- 远程执行一定会在构建阶段失败

## 4. 参数设计标准

现有 3 个 Go 工具已经形成了比较稳定的参数设计约束，新增工具建议保持一致。

### 4.1 参数名优先复用旧 CLI 语义

如果旧工具已经用 `-input`、`-output`、`-workers`、`-full-extract` 这类命名，就不要为了前端表单去重命名 Go flag。

更稳妥的做法是：

- Go 层保留原有 CLI 参数
- manifest 通过 `label`、`help`、`placeholder` 提升可读性

这样可以同时兼容：

- 原始参数模式
- 桌面结构化表单
- 远程执行同构调用
- 单工具产物导出后的 CLI 使用方式

### 4.2 默认值必须在 Go 端可独立成立

不要把默认值只写在前端表单或 manifest 中。

正确做法是：

- 前端默认值可以有
- manifest 默认值可以有
- Go 工具本身也必须能在参数缺失时补齐默认行为

原因是当前存在多种调用入口：

- 结构化表单
- 原始参数
- 本地 bridge 运行
- 远程 wrapper 运行

真正可靠的默认行为必须落在 Go 工具本身。

### 4.3 模式互斥必须在 Go 端校验

像 `utm_geojson_converter` 这种同时支持：

- `-input`
- `-convert`
- `-merge`

的工具，必须在 Go 端做模式互斥校验，而不是依赖前端避免非法组合。

原因是远程 wrapper 和未来导出产物都会绕过前端直接调用。

### 4.4 表单提示信息分层

manifest 里的 `placeholder`、`help` 和 `docs.usage` 不应写成重复的三份说明，而应该分层承担不同职责。

推荐约束如下：

- `placeholder`：只放最短示例或输入格式提示，不写大段解释
- `help`：只给复杂字段使用，适合放多行规则、注意事项或示例
- `docs.usage`：放完整说明，作为工具详情页里的主文案

当前前端的推荐展示策略是：

- 简单字段不显示 tooltip，说明写进 `docs.usage` 即可
- 复杂字段才显示 tooltip，且 tooltip 按钮放在“字段名 -> 必填星号”之后

例如：

- `HDFS 路径列表` 这类复杂字段，`placeholder` 只放 `/user/test/file_001,/user/test/dir_001`
- 多路径分隔规则、绝对路径要求、文件/目录说明写进 `help`
- 同样内容再在 `docs.usage` 中给出更完整版本，方便用户在“工具说明”页查看

## 5. Usage 文本标准

虽然桌面端现在已经有结构化表单，但 `usage` 仍然有实际价值：

- 作为详情说明和帮助文本的后备来源
- 作为没有 manifest 时的兜底说明来源
- 作为原始参数模式的说明文案

建议 `usage` 至少包含：

1. 工具用途
2. 参数解释
3. 常用示例
4. 模式说明或注意事项

注意：

- `legacy.go` 会移除旧 TUI 颜色标记
- 但仍建议保持正文可读，不要完全依赖颜色语义

## 6. 远程执行与构建约束

Go 原生工具之所以比 Python 工具多一层约束，是因为远程执行依赖 Builder 现场构建单工具二进制。

当前流程如下：

1. 宿主收到远程执行请求。
2. 根据 manifest 取到 `kind=go` 和 `source.entry`。
3. Builder 根据 `source.entry` 推导 import path。
4. Builder 生成一个临时 `main.go` wrapper。
5. wrapper 匿名导入工具包，遍历 `framework.Registry`，按 `tool.ID()` 找工具。
6. wrapper 执行工具的 `Execute()`，再次捕获 `ShowTerminal()` 暴露的 `run` 闭包。
7. wrapper 用命令行参数重组后的字符串调用 `run(context.Background(), args, os.Stdout)`。

这意味着新增 Go 工具必须满足以下隐含要求：

- 工具包可以被正常 import
- `init()` 注册副作用必须可靠
- `tool.ID()` 必须稳定
- `Execute()` 每次调用都能稳定暴露出同一个 `run` 闭包
- 核心执行逻辑不能强依赖桌面 UI 上下文

换句话说，Go 原生工具当前本质上是“可被宿主捕获的 CLI 风格函数”，而不是“只能运行在某个交互式 UI 里的页面工具”。

### 6.1 用户侧 Go 环境前置条件

对 Go 原生工具来说，用户最终会看到两层不同的前置条件：

- 本地执行前置条件
  - 工具已经被编译进桌面宿主
  - 不需要用户额外安装或配置 Go SDK

- 远程执行 / Go 导出 / 构建缓存前置条件
  - 宿主能解析到一个可用的 Go 环境
  - 这个 Go 环境可以来自：
    - 用户手动选择的本地 Go
    - 桌面宿主下载并托管的官方 Go SDK
  - 如果用户显式选择 `<无 SDK>`，这三类能力会被阻断，并提示重新配置

所以在验收一个 Go 工具是否“完整接入成功”时，至少要分别验证：

- 本地执行是否正常
- 远程执行前是否能成功准备单工具产物
- Go 单工具导出是否正常

## 7. 现有三个工具的参考模式

## 7.1 `geojson_to_shp`

特点：

- 典型的单/目录双模式输入工具
- 参数简单，只有 `-input` 和 `-output`
- 输出目录默认值在 Go 端补齐

适合参考：

- 最小 Go 工具接入骨架
- path 参数表单设计
- 输出目录默认值策略

## 7.2 `pos2gis_converter`

特点：

- 目录批处理模式
- 输入输出都较直接
- 是最清晰的“表单字段和 CLI flag 一一对应”的例子

适合参考：

- 标准目录型批处理工具
- 最朴素的本地/远程兼容模式

## 7.3 `utm_geojson_converter`

特点：

- 多模式工具
- 同时有数字、布尔、选择型参数
- 需要在 Go 端做强校验

适合参考：

- 复杂参数工具的 manifest 写法
- 模式互斥校验
- 原始参数模式与结构化表单并存

## 8. 新增 Go 原生工具的标准接入步骤

建议严格按下面顺序操作。

1. 在 `tools/go_tools/<tool_dir>/` 新建工具包。
2. 实现 `framework.Tool`，并在 `Execute()` 中通过 `ShowTerminal()` 暴露运行闭包。
3. 在包内 `init()` 中调用 `framework.Register(...)`。
4. 在 `app/legacy.go` 中加入匿名导入。
5. 在 `libs/catalog/builtin/manifests/` 新增对应 YAML。
6. 确保 manifest 中 `id` 与 `tool.ID()` 一致。
7. 确保 manifest 中 `source.entry` 指向正确的 `tool.go`。
8. 确保 `params` 与 Go flag 语义一致。
9. 启动宿主，确认工具能在工作台中出现。
10. 分别验证本地执行和远程执行。

## 9. 最低验收清单

一个 Go 原生工具要算接入完成，至少要通过下面检查。

### 9.1 发现与展示

- 宿主启动后工具能出现在工作台列表中
- 名称、分类、说明与预期一致
- 参数表单能正常渲染

### 9.2 本地执行

- 必填参数缺失时能返回清晰错误
- 默认值逻辑按预期生效
- 正常输入时可跑通主流程
- 日志能写入任务输出面板

### 9.3 远程执行

- 能成功构建单工具产物
- 能上传到远端并执行
- 参数在远端行为与本地一致

### 9.4 代码约束

- `tool.ID()` 与 manifest `id` 一致
- `source.entry` 正确
- 已在 `app/legacy.go` 匿名导入
- 未使用 `os.Exit()` 破坏宿主任务生命周期

## 10. 常见接入错误

### 忘记在 `app/legacy.go` 匿名导入

表现：

- 工具目录存在
- manifest 也存在
- 但工作台里看不到工具，或执行时报“未找到工具”

根因：

- `init()` 没触发，工具未进入 `framework.Registry`

### Manifest `id` 与 `tool.ID()` 不一致

表现：

- 工作台展示与执行行为错位
- 本地 bridge 和 manifest 元数据关联异常

根因：

- 当前系统是按 `tool.ID()` 关联 runtime，按 manifest `id` 关联元数据，两边必须一致

### `source.entry` 写错

表现：

- 本地执行成功
- 远程执行在构建阶段失败

根因：

- Builder 无法生成正确 import path 或无法找到源码入口

### Go flag 与 manifest `params` 不一致

表现：

- 表单提交后参数行为不对
- 原始参数模式和结构化表单结果不一致

根因：

- manifest 只是描述，不会自动同步到 Go flag 定义

### 核心逻辑强依赖交互式 UI

表现：

- 在旧 TUI 里能跑
- 在桌面宿主 bridge、远程 wrapper 或未来导出链路里行为异常

根因：

- 当前桌面宿主要求工具的核心执行逻辑可由单个 `run(ctx, args, out)` 闭包直接驱动

## 11. 推荐模板

下面这个模板可以直接作为新增 Go 原生工具的起点：

```go
package example_tool

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"my_tools/libs/framework"
)

type Tool struct{}

func (t *Tool) ID() string       { return "example_tool" }
func (t *Tool) Name() string     { return "示例工具" }
func (t *Tool) Category() string { return "示例分类 > 子类" }

func (t *Tool) Execute(ctx framework.AppContext) {
	usage := `示例工具

说明:
用于演示 Go 原生工具如何接入当前桌面宿主。

参数:
  -input <路径>   输入路径
  -output <路径>  输出路径
`

	ctx.ShowTerminal(t.Name(), usage, func(runCtx context.Context, args string, out io.Writer) error {
		parsedArgs := framework.ParseArgs(args)

		fs := flag.NewFlagSet("example_tool", flag.ContinueOnError)
		fs.SetOutput(out)

		var input string
		var output string

		fs.StringVar(&input, "input", "", "")
		fs.StringVar(&output, "output", "", "")

		if err := fs.Parse(parsedArgs); err != nil {
			return err
		}
		if input == "" {
			return fmt.Errorf("错误：必须指定 -input 参数")
		}
		if output == "" {
			output = filepath.Join(filepath.Dir(input), "output")
		}

		return runBusiness(runCtx, input, output, out)
	})
}

func init() {
	framework.Register(&Tool{})
}
```

对应 manifest 模板：

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
    这里写桌面宿主里的帮助文本。
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

## 12. 一句话结论

在当前仓库里，Go 原生工具的“完整接入”不是只写一个 `tool.go`，而是同时完成：

- `framework.Tool` 实现
- `init()` 注册
- `app/legacy.go` 匿名导入
- 内置 manifest
- 正确的 `source.entry`

少任意一环，都只能算“部分可用”，不算完整接入。
