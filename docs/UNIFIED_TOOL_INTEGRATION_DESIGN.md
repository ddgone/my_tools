# 统一工具接入设计

本文档定义桌面宿主工具接入层的统一架构，目标是让编译型工具（Go / Rust）和脚本型工具（Python / 未来 Node）都遵循一致的接入模型，同时保持每个工具源码的完全独立性。

## 1. 背景与动机

### 1.1 当前问题

当前仓库存在三套接入路径，体验差异明显：

| 维度 | Go 工具 | Python 工具 | Rust 工具 |
|------|---------|-------------|-----------|
| 元数据来源 | manifest + `framework.Tool` 双轨 | manifest + adapter 双轨 | manifest 单一来源 |
| 本地执行 | 宿主内 bridge 闭包直调 | 托管 venv + 解释器子进程 | 预编译二进制子进程 |
| 工具接口 | 实现 `framework.Tool` + `init()` 注册 | 实现 `framework.Tool` + `init()` 注册 | 无（纯 CLI 二进制） |
| 需要改宿主代码 | 是（`legacy.go` 匿名导入） | 是（`adapter.go` 编译进宿主） | 否 |
| 工具包形态 | 库包，但耦合 `framework` 包 | 库包，但耦合 `framework` 包 | binary crate |

Go 工具的"双轨制"是最大摩擦点：

1. 工具必须实现 `framework.Tool` 接口，在 `init()` 中注册，并在 `app/legacy.go` 添加匿名导入——三处手工步骤容易出错。
2. 本地执行走宿主内闭包直调，远程执行走 wrapper 编译，两条路径分叉。
3. `framework.Tool` 的元数据（ID/Name/Category）与 manifest 元数据是两套真相源，必须人工保持一致。
4. 工具源码对 `libs/framework` 包有强依赖，不够独立。

### 1.2 设计目标

- **源码独立性**：每个工具是一个自包含的目录，不依赖任何框架包，不包含入口胶水代码。
- **接入简单性**：新增工具只需写业务代码 + manifest，不需要改宿主代码。
- **执行统一性**：编译型工具的本地/远程执行、缓存、导出共用同一套外壳逻辑。
- **语言可扩展**：编译型轨道（Go/Rust）和脚本型轨道（Python/未来 Node）各自统一，新增语言时按轨道对号入座。

### 1.3 约束

- 远程执行、构建缓存、单工具导出已有成熟实现，本次重构不能破坏这三条链路。
- 前端表单已由 manifest 驱动自动渲染，本次不涉及前端改动。
- 现有工具的业务逻辑不变，只改接入形态。

## 2. 核心决策

**所有编译型工具都是纯库包，入口 main 全部由 builder 用共享模板生成，工具源码里不存在任何入口胶水代码。**

具体含义：

1. Go 工具是纯库包（`package <tool>`），导出约定函数 `Run`，不含 `func main()`。
2. Rust 工具是纯 library crate（`src/lib.rs`），导出约定函数 `run`，不含 `src/main.rs`。
3. builder 构建时为每个工具生成一个临时 wrapper（Go 是 `main.go`，Rust 是 wrapper crate），wrapper 调用工具的约定函数。
4. 工具源码对 `libs/framework` 零依赖。
5. manifest 是唯一元数据来源。

脚本型工具（Python）保持独立轨道，后续可泛化为"脚本类工具"供 Node 等复用。

## 3. 工具接口约定

整个系统对编译型工具的唯一约定是一个函数签名。不需要接口、不需要注册、不需要框架。

### 3.1 Go 工具约定

```go
// tools/go_tools/<tool_id>/tool.go
package <tool_id>

import (
    "context"
    "io"
)

// Run 是工具的唯一入口，由 builder 生成的 wrapper 调用。
// ctx 支持取消，args 是命令行参数（已拆分），out 是日志输出。
func Run(ctx context.Context, args []string, out io.Writer) error {
    // 业务逻辑
}
```

约束：

- 函数名固定为 `Run`，签名固定。
- `args` 是已拆分的参数数组，工具内部自行解析（可用 `flag.FlagSet`）。
- `out` 接收所有日志输出（stdout + stderr 合并到这一个 writer）。
- 返回 `nil` 表示成功，返回 `error` 表示失败。
- 不允许调用 `os.Exit()`——会破坏宿主任务生命周期管理。
- 长循环、批处理、并发 worker 应检查 `ctx.Done()` 以支持取消。

### 3.2 Rust 工具约定

```rust
// tools/rust_tools/<tool_id>/src/lib.rs

/// 工具的唯一入口，由 builder 生成的 wrapper crate 调用。
/// args 是命令行参数（已拆分，不含程序名）。
pub fn run(args: &[String]) -> Result<(), Box<dyn std::error::Error>> {
    // 业务逻辑
}
```

约束：

- 函数名固定为 `run`，签名固定。
- `args` 不含程序名，等价于 Go 的 `args`。
- 返回 `Ok(())` 表示成功，返回 `Err` 表示失败。
- 不允许在库函数中调用 `std::process::exit()`。

### 3.3 脚本型工具约定（Python）

Python 工具当前保持现有模型（脚本文件 + 解释器子进程），暂不强制约定函数签名。后续泛化为脚本轨道时再统一。

## 4. 各语言接入标准

### 4.1 Go 工具

#### 4.1.1 目录结构

```text
tools/go_tools/<tool_id>/
├── tool.go          # package <tool_id>，包含 func Run
├── tool_test.go     # 测试
└── (其他业务文件)    # 按需组织
```

约束：

- 一个工具一个独立目录。
- 包名与目录名一致。
- 不含 `func main()`，不调用 `framework.Register()`，不导入 `libs/framework`。
- 参数解析使用 `flag.FlagSet`，`SetOutput(out)` 让错误进入日志面板。

#### 4.1.2 最小骨架

```go
package example_tool

import (
    "context"
    "flag"
    "fmt"
    "io"
)

func Run(ctx context.Context, args []string, out io.Writer) error {
    fs := flag.NewFlagSet("example_tool", flag.ContinueOnError)
    fs.SetOutput(out)

    var input string
    var output string

    fs.StringVar(&input, "input", "", "输入路径")
    fs.StringVar(&output, "output", "", "输出路径")

    if err := fs.Parse(args); err != nil {
        return err
    }
    if input == "" {
        return fmt.Errorf("必须指定 -input 参数")
    }

    return runBusiness(ctx, input, output, out)
}

func runBusiness(ctx context.Context, input, output string, out io.Writer) error {
    // 业务逻辑
    return nil
}
```

#### 4.1.3 manifest 模板

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
    桌面宿主里的使用说明。
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

#### 4.1.4 与旧标准的差异

| 旧标准 | 新标准 |
|--------|--------|
| 实现 `framework.Tool` 接口 | 导出 `func Run(ctx, args, out) error` |
| `init()` 中 `framework.Register()` | 不需要注册 |
| `app/legacy.go` 匿名导入 | 不需要改宿主代码 |
| `Execute()` 中调用 `ctx.ShowTerminal()` | 不需要，`Run` 就是入口 |
| `framework.ParseArgs(args)` 拆分参数 | `args` 已是 `[]string`，直接用 `flag.FlagSet.Parse` |
| 依赖 `libs/framework` 包 | 零依赖 |
| 元数据双轨（framework.Tool + manifest） | manifest 唯一来源 |

### 4.2 Rust 工具

#### 4.2.1 目录结构

```text
tools/rust_tools/<tool_id>/
├── Cargo.toml       # [lib] crate，不含 [[bin]]
├── Cargo.lock
└── src/
    ├── lib.rs       # pub fn run + 业务逻辑
    ├── lib_test.rs  # 测试（或内联 #[cfg(test)] mod tests）
    └── (其他业务模块)
```

约束：

- 一个工具一个独立目录。
- Cargo.toml 中 crate 名与 `tool_id` 一致。
- 不含 `src/main.rs`，不是 binary crate。
- 产出的二进制名（由 wrapper 决定）与 `tool_id` 一致。

#### 4.2.2 最小骨架

```rust
// src/lib.rs

pub fn run(args: &[String]) -> Result<(), Box<dyn std::error::Error>> {
    let mut input = String::new();
    let mut output = String::new();

    let mut i = 0;
    while i < args.len() {
        match args[i].as_str() {
            "--input" if i + 1 < args.len() => { input = args[i + 1].clone(); i += 2; }
            "--output" if i + 1 < args.len() => { output = args[i + 1].clone(); i += 2; }
            _ => i += 1,
        }
    }

    if input.is_empty() {
        return Err("必须指定 --input 参数".into());
    }

    run_business(&input, &output)
}

fn run_business(input: &str, output: &str) -> Result<(), Box<dyn std::error::Error>> {
    // 业务逻辑
    Ok(())
}
```

推荐使用 `clap` 进行参数解析，但不强求。约定函数签名固定，内部实现自由。

#### 4.2.3 Cargo.toml 模板

```toml
[package]
name = "example_tool"
version = "0.1.0"
edition = "2024"

[lib]
name = "example_tool"
path = "src/lib.rs"

[dependencies]
# 按需声明，如 clap、anyhow、rayon 等
```

#### 4.2.4 manifest 模板

```yaml
id: example_tool
name: 示例 Rust 工具
kind: rust
category: Rust工具 > 示例
icon: rust
description: 一句话描述。
docs:
  summary: 简短摘要。
  usage: |
    使用说明。
params:
  - key: input
    argKey: input
    type: path
    pathMode: file
    label: 输入文件
    required: true
execution:
  local:
    adapter: rust-binary
  remote:
    strategy: upload-binary-and-run
export:
  strategy: export-binary
source:
  entry: tools/rust_tools/example_tool/src/lib.rs
```

注意：`source.entry` 指向 `src/lib.rs` 而非 `src/main.rs`，builder 由此推导 crate 根目录。

#### 4.2.5 与旧标准的差异

| 旧标准 | 新标准 |
|--------|--------|
| binary crate（`src/main.rs`） | library crate（`src/lib.rs`） |
| `fn main()` 含业务逻辑 | `pub fn run(args) -> Result<()>` 含业务逻辑 |
| builder 直接在 crate 目录编译 | builder 生成 wrapper crate 再编译 |
| `source.entry` 指向 `src/main.rs` | `source.entry` 指向 `src/lib.rs` |

### 4.3 Python 工具

当前保持不变：脚本文件放在 `tools/python_tools/scripts/`，通过 adapter 编译进宿主执行。后续可泛化为脚本轨道。

## 5. 构建层设计（builder）

builder 是编译型工具的统一构建外壳，位于 `app/internal/builder/`。

### 5.1 统一入口

`BuildPackage(req BuildRequest)` 按 `Kind` 分发到各语言的构建函数，三种语言共用同一套缓存结构、缓存 key 计算框架和产物输出逻辑。这部分不变。

### 5.2 Go 构建分支简化

#### 当前流程

1. 根据 `source.entry` 推导 import path。
2. 生成临时 `main.go` wrapper（`renderGoWrapper`），wrapper 匿名导入工具包，遍历 `framework.Registry` 找到工具，执行 `Execute()` 捕获 `ShowTerminal()` 暴露的闭包。
3. `go build` 编译 wrapper。

#### 之后流程

1. 根据 `source.entry` 推导 import path。
2. 生成临时 `main.go` wrapper，wrapper 导入工具包，直接调用 `Run` 函数。
3. `go build` 编译 wrapper。

wrapper 模板从 ~60 行缩减到 ~15 行：

```go
// builder 生成的 wrapper，对所有 Go 工具完全一样，只有 import 路径和工具 ID 不同
package main

import (
    "context"
    "fmt"
    "io"
    "os"

    "<tool_import_path>"  // 唯一变量
)

func main() {
    err := <tool_package>.Run(context.Background(), os.Args[1:], io.Stdout)
    if err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
```

`renderGoWrapper` 函数大幅简化，不再引用 `framework.Registry`、`captureAppContext`、`ShowTerminal` 等概念。

#### cache key 变化

`computeGoCacheKey` 当前把 `renderGoWrapper` 的输出纳入 hash。新 wrapper 模板更简单，但同样需要纳入 hash，保证模板变更时缓存自动失效。缓存输入文件扫描逻辑（`collectGoRelevantFiles`）不变。

### 5.3 Rust 构建分支改造

#### 当前流程

1. 根据 `source.entry` 定位 crate 根目录（`resolveRustCrateRoot`）。
2. 在 crate 根目录直接 `cargo build --release`（或 `cargo zigbuild`）。
3. 产物名等于 crate 名（即 `tool_id`）。

#### 之后流程

1. 根据 `source.entry` 定位 crate 根目录（逻辑不变）。
2. 生成临时 wrapper crate：
   - `Cargo.toml`：path dependency 指向工具 crate。
   - `src/main.rs`：固定模板，调用 `run` 函数。
3. 在 wrapper crate 目录 `cargo build --release`（或 `cargo zigbuild`）。
4. 产物名等于 wrapper crate 名（约定为 `wrapper_<tool_id>`），但最终复制到缓存时重命名为 `tool_id`。

wrapper crate 的 `Cargo.toml`：

```toml
[package]
name = "wrapper_<tool_id>"
version = "0.0.0"
edition = "2024"

[dependencies]
<tool_id> = { path = "<crate_root_abs_path>" }
```

wrapper crate 的 `src/main.rs`（所有工具同一个模板，只有 crate 名不同）：

```rust
fn main() {
    let args: Vec<String> = std::env::args().skip(1).collect();
    if let Err(e) = <tool_id>::run(&args) {
        eprintln!("{e}");
        std::process::exit(1);
    }
}
```

#### cache key 变化

`computeRustCacheKey` 当前扫描工具 crate 下的 `.rs`/`Cargo.toml`/`Cargo.lock`/`.cargo/config.toml`。之后还需要把 wrapper 的 `Cargo.toml` 和 `main.rs` 模板内容纳入 hash，保证模板变更时缓存自动失效。工具 crate 内部的扫描逻辑不变。

#### 交叉编译

交叉编译逻辑不变：native 构建用 `cargo build`，cross 构建用 `cargo zigbuild --target <triple>`。区别只是 `cmd.Dir` 从工具 crate 目录变为 wrapper crate 目录。Rust 工具的依赖（`anyhow`、`clap` 等）通过 path dependency 自动传递给 wrapper，不需要在 wrapper 的 `Cargo.toml` 中重复声明。

#### target dir

Rust 构建使用临时 `CARGO_TARGET_DIR`（当前已如此），避免污染工具 crate 的 `target/` 目录。wrapper crate 的构建产物在临时 target 目录下，路径为 `<target_dir>/release/wrapper_<tool_id>`（native）或 `<target_dir>/<triple>/release/wrapper_<tool_id>`（cross）。builder 找到产物后复制到缓存路径并重命名为 `tool_id`。

## 6. 本地执行

### 6.1 统一执行函数

当前 `app/rust_tools.go` 的 `executeLocalRustTool` 泛化为 `executeLocalBinaryTool`，Go 和 Rust 共用：

```go
func executeLocalBinaryTool(ctx context.Context, writer io.Writer, manifest toolspec.ToolManifest) func(args string) error {
    return func(args string) error {
        // 1. 查找随宿主打包的产物
        binaryPath, err := resolveBundledBinary(manifest)
        if err != nil {
            // 2. 找不到 -> 现场构建本平台产物
            binaryPath, err = buildLocalBinary(ctx, writer, manifest)
            if err != nil {
                return err
            }
        }

        // 3. 参数归一化
        parsedArgs, err := framework.ParseArgs(args)
        if err != nil {
            return err
        }
        if manifest.Kind == toolspec.ToolKindRust {
            parsedArgs = normalizeRustCLIArgs(parsedArgs)
        }

        // 4. 子进程执行
        cmd := exec.CommandContext(ctx, binaryPath, parsedArgs...)
        cmd.Stdout = writer
        cmd.Stderr = writer
        return cmd.Run()
    }
}
```

### 6.2 产物查找

随宿主打包的产物按 kind 分目录：

```text
<assets_dir>/
├── go/
│   ├── windows_amd64/
│   │   ├── geojson_to_shapefile.exe
│   │   ├── hdfs_download.exe
│   │   └── ...
│   └── linux_amd64/
│       ├── geojson_to_shapefile
│       └── ...
├── rust/
│   ├── windows_amd64/
│   │   ├── las_voxelizer.exe
│   │   └── ...
│   └── linux_amd64/
│       ├── las_voxelizer
│       └── ...
```

`resolveBundledBinary` 按 `manifest.Kind` 和 `runtime.GOOS_GOARCH` 拼路径查找。

### 6.3 现场构建兜底

当打包产物不存在且运行在源码工作区时，调用 `builder.BuildPackage` 构建本平台产物。当前 `resolveLocalRustBinary` 已实现此逻辑，泛化后 Go 工具复用同一路径。

### 6.4 execution.go 的 switch 简化

```go
// 当前
switch {
case manifestOK && manifest.Kind == toolspec.ToolKindRust:
    execErr = executeLocalRustTool(runCtx, writer, manifest)(req.Args)
case legacyTool != nil && legacyTool.runPython != nil:
    execErr = legacyTool.runPython(...)
case legacyTool != nil && legacyTool.run != nil:
    execErr = legacyTool.run(runCtx, req.Args, writer)
}

// 之后
switch {
case manifest.Kind == toolspec.ToolKindGo || manifest.Kind == toolspec.ToolKindRust:
    execErr = executeLocalBinaryTool(runCtx, writer, manifest)(req.Args)
case manifest.Kind == toolspec.ToolKindPython:
    execErr = executeLocalPythonTool(runCtx, writer, manifest, req)(...)
}
```

Go 工具不再有"本地执行不需要 Go SDK"的优势。本地执行要么用打包产物，要么源码工作区下现场构建（需要 Go SDK）。这是可接受的代价，因为：

1. 交付安装包会包含所有平台产物，用户不需要 Go SDK。
2. 远程执行、导出本来就需要 Go SDK，这不是新增依赖。
3. 消除了闭包直调与子进程执行的行为差异。

## 7. 远程执行

远程执行链路**零改动**。

当前流程（`execution_remote.go`）：

1. SSH 连接目标机器。
2. `builder.BuildPackage` 构建目标平台产物。
3. 上传产物到远端临时目录。
4. 远端执行二进制。
5. 探测结果、按需保留工作目录。

builder 的改造（Go wrapper 简化、Rust wrapper 新增）对远程执行透明——`BuildPackage` 的输入输出接口不变。远程执行只依赖 `manifest.Source.Entry`、`manifest.Kind` 和目标平台，不关心工具内部是闭包还是 `Run` 函数。

## 8. 构建缓存

构建缓存**零改动**。

缓存结构不变：

```text
<cache_dir>/<tool_id>/<os>_<arch>/
├── artifact/
│   └── <tool_id>_<os>_<arch>(.exe)
├── source/
│   └── <source_entry_filename>
└── .cachekey
```

缓存 key 计算逻辑的变化仅限于 wrapper 模板内容纳入 hash：

- Go：`renderGoWrapper` 输出变简单了，但仍纳入 hash。
- Rust：新增 wrapper `Cargo.toml` + `main.rs` 模板内容纳入 hash。

工具源码文件的扫描逻辑不变（Go 扫 `.go`/`.mod`/`.sum`/`.work`，Rust 扫 `.rs`/`Cargo.toml`/`Cargo.lock`/`.cargo/config.toml`）。

## 9. 单工具导出

单工具导出**零改动**。

当前流程（`app/export.go`）调用 `builder.BuildPackage` 构建目标平台产物，复制到导出目录。builder 改造对导出透明。

导出策略映射不变：

- Go / Rust：`export-binary`（导出编译后的二进制）
- Python：`export-script`（导出脚本文件）

## 10. 产物打包与分发

发布构建时需要把所有编译型工具的二进制放进 assets 目录。当前 `scripts/build.go` 负责整体构建，需要增加一步：

1. 遍历 `libs/catalog/builtin/manifests/*.yaml`。
2. 对 `kind: go` 和 `kind: rust` 的工具，按当前平台调用 `builder.BuildPackage`。
3. 产物复制到 `assets/<kind>/<os>_<arch>/<tool_id>(.exe)`。

这步可以复用 builder 的缓存——如果之前已构建过，直接命中缓存。

## 11. 桥接层退役

### 11.1 删除的内容

| 文件/符号 | 说明 |
|-----------|------|
| `libs/framework/framework.go` 的 `Tool` 接口 | 不再需要 |
| `libs/framework/framework.go` 的 `Registry` / `Register` | 不再需要 |
| `libs/framework/framework.go` 的 `AppContext` 接口 | 不再需要 |
| `libs/framework/framework.go` 的 `ShowTerminal` / `ShowPythonTerminal` | 不再需要 |
| `app/legacy.go` 的 `loadLegacyTools` | 不再需要 |
| `app/legacy.go` 的 `captureAppContext` | 不再需要 |
| `app/legacy.go` 的 `legacyTool` 结构 | 不再需要 |
| `app/legacy.go` 的匿名导入 | 不再需要 |
| `app/legacy.go` 的 `buildToolManifests` 中的 legacy 补元数据逻辑 | manifest 成为唯一来源 |

### 11.2 保留的内容

| 文件/符号 | 说明 |
|-----------|------|
| `libs/framework/utils.go` 的 `ParseArgs` | 仍在本地执行和远程执行中使用 |
| `tools/python_tools/adapter.go` | Python 轨道暂不改动 |

### 11.3 `ParseArgs` 的归属

`ParseArgs` 当前在 `libs/framework/` 下。framework 包退役后，`ParseArgs` 可以：

- **方案 A**：移到 `libs/core/procutil/`（与命令执行相关的公共能力放一起）。
- **方案 B**：移到 `libs/core/toolspec/`（与工具规格相关的公共能力放一起）。

推荐方案 A，因为 `ParseArgs` 的职责是"把参数字符串拆分为 `[]string`"，属于命令行处理范畴，不是工具规格。

### 11.4 `buildToolManifests` 简化

当前 `buildToolManifests` 会从 `framework.Registry` 补充缺失的 manifest 元数据。之后 manifest 是唯一来源，这个函数简化为：

```go
func loadToolManifests() (map[string]toolspec.ToolManifest, error) {
    loaded, err := builtin.Load()
    if err != nil {
        return nil, err
    }
    manifests := make(map[string]toolspec.ToolManifest, len(loaded))
    for _, manifest := range loaded {
        manifests[manifest.ID] = manifest
    }
    return manifests, nil
}
```

### 11.5 App 结构变化

`App` 结构体中的 `legacy map[string]*legacyTool` 字段删除。`manifests map[string]toolspec.ToolManifest` 保留并成为唯一工具表。所有通过 `a.legacy[toolID]` 查找工具的逻辑改为通过 `a.manifests[toolID]` 查找。

## 12. 脚本型工具轨道（Python / 未来 Node）

### 12.1 当前 Python 模型

Python 工具当前通过 `tools/python_tools/adapter.go` 接入：

1. 脚本内嵌到 Go 二进制（`go:embed`）。
2. `adapter.go` 实现 `framework.Tool`，在 `init()` 中注册。
3. 本地执行：释放脚本到临时目录，用托管 venv 的解释器子进程执行。
4. 远程执行：builder 复制脚本到产物，上传后用远端解释器执行。

### 12.2 后续泛化方向

当 framework 退役后，Python adapter 也需要脱离 `framework.Tool`。但 Python 的执行模型（内嵌脚本 + 解释器子进程）与编译型工具差异较大，短期内保持独立实现即可。

未来引入 Node 工具时，可以抽象出"脚本轨道"的通用逻辑：

```go
type ScriptToolExecutor struct {
    Interpreter string    // "python3" / "node"
    ScriptPath  string    // 脚本文件路径
    EnvSetup    func() (string, error)  // 环境准备（如 venv 激活）
}
```

当前不提前做这个抽象，等真正有第二个脚本语言时再提取。

### 12.3 Python adapter 的过渡方案

framework 退役时，Python adapter 的最小改动：

1. 不再实现 `framework.Tool`，不再 `framework.Register()`。
2. 在 `app/legacy.go`（或新的 `app/python_tools.go`）中硬编码 Python 工具的元数据或从 manifest 读取。
3. 本地执行逻辑（释放脚本 + 解释器子进程）不变。

## 13. manifest 生成器

### 13.1 目标

新增工具时，通过一个命令生成目录骨架 + manifest 模板，减少手工步骤和出错概率。

### 13.2 使用方式

```bash
go run ./scripts/new-tool/ -name "示例工具" -id example_tool -kind go -category "示例>子类"
```

### 13.3 生成内容

#### Go 工具

```text
tools/go_tools/example_tool/
├── tool.go                              # Run 函数骨架 + 示例参数
└── tool_test.go                         # 测试骨架
libs/catalog/builtin/manifests/
└── example_tool.yaml                    # manifest 模板
```

`tool.go` 骨架：

```go
package example_tool

import (
    "context"
    "flag"
    "fmt"
    "io"
)

func Run(ctx context.Context, args []string, out io.Writer) error {
    fs := flag.NewFlagSet("example_tool", flag.ContinueOnError)
    fs.SetOutput(out)

    var input string
    var output string

    fs.StringVar(&input, "input", "", "输入路径")
    fs.StringVar(&output, "output", "", "输出路径")

    if err := fs.Parse(args); err != nil {
        return err
    }
    if input == "" {
        return fmt.Errorf("必须指定 -input 参数")
    }

    fmt.Fprintf(out, "输入: %s\n输出: %s\n", input, output)
    return nil
}
```

#### Rust 工具

```text
tools/rust_tools/example_tool/
├── Cargo.toml                           # [lib] crate 骨架
└── src/
    └── lib.rs                           # run 函数骨架
libs/catalog/builtin/manifests/
└── example_tool.yaml                    # manifest 模板
```

`src/lib.rs` 骨架：

```rust
pub fn run(args: &[String]) -> Result<(), Box<dyn std::error::Error>> {
    let mut input = String::new();
    let mut output = String::new();

    let mut i = 0;
    while i < args.len() {
        match args[i].as_str() {
            "--input" if i + 1 < args.len() => { input = args[i + 1].clone(); i += 2; }
            "--output" if i + 1 < args.len() => { output = args[i + 1].clone(); i += 2; }
            _ => i += 1,
        }
    }

    if input.is_empty() {
        return Err("必须指定 --input 参数".into());
    }

    println!("输入: {}\n输出: {}", input, output);
    Ok(())
}
```

#### manifest 模板

```yaml
id: example_tool
name: 示例工具
kind: <go|rust>
category: 示例 > 子类
icon: <go|rust>
description: 一句话说明工具用途。
docs:
  summary: 简短摘要。
  usage: |
    使用说明。
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
    adapter: <go-binary|rust-binary>
  remote:
    strategy: upload-binary-and-run
export:
  strategy: export-binary
source:
  entry: <tools/go_tools/example_tool/tool.go|tools/rust_tools/example_tool/src/lib.rs>
```

### 13.4 实现方式

生成器本身是一个 Go 程序，放在 `scripts/new-tool/`：

```text
scripts/new-tool/
├── main.go              # 入口，解析参数
└── templates/
    ├── go_tool.go.tmpl   # Go 工具骨架模板
    ├── go_test.go.tmpl   # Go 测试骨架模板
    ├── rust_lib.rs.tmpl  # Rust lib.rs 模板
    ├── rust_cargo.toml.tmpl # Rust Cargo.toml 模板
    └── manifest.yaml.tmpl # manifest 模板
```

不引入额外依赖，使用 Go 标准库 `text/template`。

### 13.5 生成后的验证

生成器在创建文件后自动执行基本检查：

- Go：`go vet ./tools/go_tools/<tool_id>/...`
- Rust：`cargo check`（在工具 crate 目录）
- manifest：YAML 语法校验 + 必填字段检查

## 14. 迁移计划

### 阶段一：基础设施统一 ✅ 已完成

不改动现有工具，只改 builder 和执行层，让新模型可用。

| 步骤 | 改动文件 | 说明 |
|------|----------|------|
| 1 | `app/internal/builder/pack.go` | 简化 `renderGoWrapper`，不再引用 framework |
| 1 | `app/internal/builder/pack_rust.go` | 新增 wrapper crate 生成逻辑 |
| 2 | `app/rust_tools.go` → `app/binary_tools.go` | `executeLocalRustTool` 泛化为 `executeLocalBinaryTool` |
| 3 | `app/execution.go` | switch 分支收敛 |
| 4 | `libs/core/procutil/` 或 `libs/core/toolspec/` | `ParseArgs` 迁移 |

验证：用一个新建的 Go 工具（纯库包 + `Run` 函数）走通本地执行、远程执行、导出。

### 阶段二：迁移现有 Go 工具

5 个 Go 工具逐一迁移，每个工具的改动是机械的：

| 工具 | 改动 |
|------|------|
| `geojson_to_shapefile` | `Execute` 闭包 → `Run` 函数，删 `framework` 导入和 `init()` |
| `hdfs_download` | 同上 |
| `pos_trajectory_to_gis` | 同上 |
| `recursive_content_dir_diff` | 同上 |
| `utm_extract_to_gis` | 同上 |

每迁移一个工具：

1. 删除 `framework.Tool` 实现、`init()` 注册。
2. 把 `Execute()` 中的闭包逻辑搬为 `func Run(ctx, args, out) error`。
3. 参数解析从 `framework.ParseArgs(args string)` + `flag.Parse` 改为 `flag.Parse(args)`。
4. 删除 `app/legacy.go` 中对应的匿名导入。
5. 确认 manifest 的 `source.entry` 指向正确。
6. 分别验证本地执行和远程执行。

### 阶段三：迁移 Rust 工具 ✅ 已完成

| 工具 | 改动 |
|------|------|
| `las_voxelizer` | `src/main.rs` → `src/lib.rs`，`fn main()` → `pub fn run()`，`Cargo.toml` 确认 `[lib]` |
| `sdk_demo` | 同上，或如果是纯演示则删除 |

每迁移一个工具：

1. `src/main.rs` 重命名为 `src/lib.rs`。
2. `fn main()` 逻辑搬为 `pub fn run(args: &[String]) -> Result<...>`。
3. `Cargo.toml` 确认是 library crate。
4. manifest 的 `source.entry` 从 `src/main.rs` 改为 `src/lib.rs`。
5. 验证本地执行、远程执行、导出。

### 阶段四：退役桥接层

| 步骤 | 改动 |
|------|------|
| 1 | 删除 `libs/framework/framework.go` 的 `Tool`/`Registry`/`Register`/`AppContext` |
| 2 | 删除 `app/legacy.go` 的 `loadLegacyTools`/`captureAppContext`/`legacyTool`/匿名导入 |
| 3 | `buildToolManifests` 简化为 `loadToolManifests` |
| 4 | `App` 结构删除 `legacy` 字段 |
| 5 | Python adapter 脱离 framework（最小改动过渡） |

### 阶段五：manifest 生成器

| 步骤 | 改动 |
|------|------|
| 1 | 创建 `scripts/new-tool/` |
| 2 | 编写模板文件 |
| 3 | 实现生成逻辑 |
| 4 | 用生成器创建一个测试工具验证 |

### 阶段六：产物打包 ✅ 已完成

| 步骤 | 改动 |
|------|------|
| 1 | `scripts/build.go` 增加遍历 manifest 编译产物的步骤 |
| 2 | 产物输出到 `assets/<kind>/<os>_<arch>/` |

### 阶段七：文档更新 ✅ 已完成

| 步骤 | 改动 |
|------|------|
| 1 | 更新 `docs/ARCHITECTURE.md` |
| 2 | 更新 `docs/GO_NATIVE_TOOL_INTEGRATION_STANDARD.md` |
| 3 | 更新 `docs/RUST_TOOL_INTEGRATION_STANDARD.md` |
| 4 | 新增 ADR 记录决策 |

## 15. 验收标准

### 15.1 编译型工具通用验收

每个迁移后的工具必须通过：

- **发现与展示**：宿主启动后工具出现在工作台列表，名称/分类/说明与 manifest 一致，参数表单正常渲染。
- **本地执行**：必填参数缺失时返回清晰错误；默认值逻辑生效；正常输入跑通主流程；日志写入任务输出面板；取消任务不卡死。
- **远程执行**：能构建目标平台产物；能上传远端执行；参数行为与本地一致。
- **构建缓存**：第二次构建命中缓存；修改源码后缓存失效。
- **单工具导出**：能导出目标平台二进制。
- **源码独立性**：工具目录不导入 `libs/framework`，不含 `func main()`，不含 `init()` 注册。

### 15.2 系统级验收

- `go vet ./libs/...` 通过。
- `go vet ./tools/...` 通过。
- `cd app && go vet ./...` 通过。
- `cd app && go test ./...` 通过。
- `cd app/frontend && npm run lint` 通过。
- `cd app/frontend && npm run typecheck` 通过。
- `go run scripts/build.go` 构建成功。

## 16. 风险与缓解

### 16.1 Go 本地执行不再零依赖

**风险**：旧模型下 Go 工具本地执行不需要 Go SDK（闭包直调），新模型需要打包产物或现场构建。

**缓解**：

- 交付安装包包含所有平台产物，用户侧不需要 Go SDK。
- 开发态源码工作区下现场构建需要 Go SDK，但这与远程执行/导出的前置条件一致，不是新增依赖。
- 可以在产物缺失且无 Go SDK 时给出明确提示，引导用户安装或使用安装包。

### 16.2 Rust wrapper crate 增加构建时间

**风险**：每个 Rust 工具构建时多了一个 wrapper crate 的编译开销。

**缓解**：

- wrapper crate 极简（只有 `main.rs` 几行），编译开销可忽略。
- 工具 crate 的依赖通过 path dependency 复用，不会重复编译。
- 构建缓存命中时跳过编译。

### 16.3 现有工具迁移的回归风险

**风险**：5 个 Go 工具 + 2 个 Rust 工具的接口迁移可能引入行为变化。

**缓解**：

- 迁移是机械的——闭包内容原封不动搬到命名函数。
- 参数解析从 `framework.ParseArgs(string)` + `flag.Parse([]string)` 变为 `flag.Parse([]string)`，语义等价（`ParseArgs` 本质就是把字符串拆成 `[]string`）。
- 每个工具迁移后单独验证本地执行和远程执行。

### 16.4 Python adapter 过渡期

**风险**：framework 退役后 Python adapter 需要适配，可能影响 Python 工具可用性。

**缓解**：

- Python adapter 的改动最小化——只脱掉 `framework.Tool` 接口，执行逻辑不变。
- 在阶段四之前确保 Python 工具仍可正常执行。

## 17. 不做的事情

- **不重写前端**：前端表单已由 manifest 驱动，本次不涉及前端改动。
- **不提前抽象脚本轨道**：等真正有第二个脚本语言（如 Node）时再提取。
- **不改变 builder 的公共接口**：`BuildPackage` / `ProbeBuildCache` 的签名不变。
- **不改变缓存目录结构**：现有的缓存不需要清理重建（cache key 会因 wrapper 模板变化而自然失效）。
- **不改变远程执行协议**：SSH 上传 + 远端执行的流程不变。
- **不引入新的框架或 SDK**：工具接入零框架依赖，不需要引入任何新的抽象层。

## 18. 对应关系速查

| 关注点 | 文件 | 说明 |
|--------|------|------|
| 工具规格定义 | `libs/core/toolspec/types.go` | 不变 |
| manifest 加载 | `libs/catalog/builtin/service.go` | 不变 |
| Go 构建 | `app/internal/builder/pack.go` | 简化 `renderGoWrapper` |
| Rust 构建 | `app/internal/builder/pack_rust.go` | 新增 wrapper crate 生成 |
| 本地执行 | `app/execution.go` + `app/binary_tools.go` | 统一为 `executeLocalBinaryTool` |
| 远程执行 | `app/execution_remote.go` | 不变 |
| 单工具导出 | `app/export.go` | 不变 |
| 桥接层（退役） | `app/legacy.go` | 大幅简化或删除 |
| 框架（退役） | `libs/framework/framework.go` | 删除接口/注册，保留 `ParseArgs` |
| Python adapter | `tools/python_tools/adapter.go` | 最小改动过渡 |
| manifest 生成器 | `scripts/new-tool/` | 新增 |
| 产物打包 | `scripts/build.go` | 增加产物编译步骤 |
