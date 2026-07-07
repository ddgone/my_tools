# Rust 工具完整接入标准

本文档定义当前仓库里 Rust 工具接入桌面宿主的完整标准。范围只覆盖当前已经真实落地的接入方式，本文以现有 Rust 工具 `tools/rust_tools/bxn_delivery_point_cloud_qc/` 为准。

## 1. 目标

一个新的 Rust 工具要被桌面宿主正确接入，必须同时满足以下 5 件事：

1. 工具源码以标准 Rust library crate 形式存在于 `tools/rust_tools/` 下。
2. 内置 manifest 能为前端提供名称、参数、说明、执行策略、导出策略和源码入口。
3. 本地执行能找到随宿主分发的本地产物，或在源码工作区下按当前 Rust 环境现场构建本机产物。
4. 远程执行链路能依据 manifest 的 `source.entry` 构建目标平台单工具产物。
5. 单工具导出和产物中心都能通过统一 `builder.BuildPackage(...)` 产出 Rust 二进制。

只完成其中一部分都不算“完整接入”。

## 2. 当前真实接入模型

当前 Rust 工具采用的是“library crate 源码 + 构建时 wrapper crate + 专用本地执行适配层”的模型。

### 2.1 工具源码层

Rust 工具当前放在：

```text
tools/rust_tools/<tool_id>/
```

工具本身必须是 **library crate**，不是 binary crate：

- `Cargo.toml` 必须包含 `[lib]` 段，**不能**包含 `[[bin]]`。
- 入口文件是 `src/lib.rs`，**不能**有 `src/main.rs`。
- 库 crate 必须导出 `pub fn run(args: &[String]) -> Result<(), Box<dyn std::error::Error>>`（推荐用 `anyhow::Result<()>` 简化签名）。
- 不能在 lib.rs 里定义 `fn main()`。
- lib.rs 是纯逻辑入口，把 CLI 参数解析和主流程放在这个 `run` 函数里。

不要求 Rust 工具实现 Go 侧的 `framework.Tool`，也不要求像 Go 工具那样走 `legacy.go` 的桥接注册。

### 2.2 构建层：wrapper crate 机制

Rust 工具本身是 library crate，不能直接 cargo build 出二进制。构建系统会**自动生成临时 wrapper crate**，把 library crate 包成可执行文件。

wrapper crate 的结构：

```text
<tmpdir>/
├── Cargo.toml
└── src/
    └── main.rs
```

生成的 `Cargo.toml` 通过 path dependency 指向工具 library crate：

```toml
[package]
name = "wrapper_<tool_id>"
version = "0.0.0"
edition = "2024"

[dependencies]
<tool_id> = { path = "<crate路径>" }
```

生成的 `src/main.rs` 调用 library crate 的 `run` 函数：

```rust
fn main() {
    let args: Vec<String> = std::env::args().skip(1).collect();
    if let Err(e) = <tool_id>::run(&args) {
        eprintln!("{e}");
        std::process::exit(1);
    }
}
```

构建产物命名规则：
- 非 Windows：`wrapper_<tool_id>`
- Windows：`wrapper_<tool_id>.exe`

这一机制封装在 `app/internal/builder/pack_rust.go` 的 `writeRustWrapperCrate(...)` 中，工具开发者无需手动创建 wrapper。

### 2.3 Manifest 层

`libs/catalog/builtin/manifests/*.yaml` 提供结构化元数据，供前端、本地执行适配层、远程执行和导出使用。

对于 Rust 工具，必须提供：

- `id`
- `name`
- `kind: rust`
- `category`
- `icon`
- `description`
- `docs.summary`
- `docs.usage`
- `params`
- `execution.local.adapter: rust-binary`
- `execution.remote.strategy: upload-binary-and-run`
- `export.strategy: export-binary`
- `source.entry`

其中：

- `kind: rust` 是宿主把它纳入 Rust 执行/构建分支的关键开关。
- `source.entry` **必须指向 `src/lib.rs`**，不是 main.rs，因为工具是 library crate。
- `source.entry` 不是可选增强项，而是本地现场构建、远程执行和导出的必要输入。

### 2.4 本地执行层

本地执行入口当前在 `app/internal/adapter/` 下的 rust-binary adapter。

当前逻辑是：

1. 先尝试从随宿主分发的本地产物目录中解析当前平台二进制。
2. 如果当前运行在源码工作区且未找到已打包产物，则转入 `builder.BuildPackage(...)`。
3. 使用 manifest 的 `source.entry` 定位 crate，生成 wrapper crate 并基于当前选中的 Rust / Zig 环境构建宿主平台产物。
4. 直接执行得到的本机二进制。

这意味着：

- Rust 工具本地执行不是 Go 工具那种“宿主内 bridge 闭包”。
- Rust 工具本地执行也不是 Python 工具那种“解释器 + 托管 venv”。
- 它是“优先使用打包好的宿主平台二进制，源码工作区下允许现场构建”的模型。

### 2.5 远程执行与导出

当前实现里：

- 远程执行：先由宿主本地构建目标平台单工具产物，再上传远端执行
- 单工具导出：由宿主后台构建目标平台单工具产物，再复制到导出位置
- 产物中心：复用同一条 Rust 构建链路准备缓存和最终文件

Rust 工具不允许绕过 builder 直接在远端执行源码，也不允许把整套 Rust 环境一起塞进导出产物。

## 3. 必须满足的接入标准

### 3.1 目录与文件标准

新的 Rust 工具应放在：

```text
tools/rust_tools/<tool_id>/
```

必需的最小结构：

```text
tools/rust_tools/<tool_id>/
├── Cargo.toml
├── Cargo.lock
└── src/
    └── lib.rs
```

约束如下：

- **不能有 `src/main.rs`**，不能有 `fn main()`。
- `Cargo.toml` 中必须包含 `[lib]` 段，**不能**包含 `[[bin]]`。
- `Cargo.toml` 中 crate 名建议直接与 `tool_id` 一致。
- `target/` 必须继续忽略，不得把 Rust 构建产物提交进仓库。

### 3.2 `Cargo.toml` 标准

```toml
[package]
name = "your_tool"
version = "0.1.0"
edition = "2024"

[lib]
name = "your_tool"
path = "src/lib.rs"

[dependencies]
anyhow = "1"
clap = { version = "4", features = ["derive"] }
```

约束：

- `[lib]` 段必须存在。
- `[lib].name` 与 `[package].name` 建议保持一致，与 `tool_id` 一致。
- 不允许出现 `[[bin]]`。
- 推荐使用 `anyhow::Result` 作为 `run` 的返回类型。

### 3.3 `src/lib.rs` 标准

```rust
use anyhow::Result;
use clap::Parser;

#[derive(Parser, Debug)]
#[command(author, version, about = "你的工具说明")]
struct Args {
    #[arg(long)]
    input: String,
}

pub fn run(args: &[String]) -> Result<()> {
    let cli = Args::parse_from(args);
    // 你的工具主逻辑
    println!("input = {}", cli.input);
    Ok(())
}
```

约束：

- 必须导出 `pub fn run(args: &[String]) -> Result<()>`，其中 `Result<()>` 建议为 `anyhow::Result<()>`。
- **不能**定义 `fn main()`。
- 参数解析使用 `clap::Parser::parse_from(args)`，从 `&[String]` 切片解析。
- 参数错误应通过 `Result::Err` 返回，不能 `panic!` 或做静默失败。
- 进度和日志输出到 `stderr`，最终结果输出到 `stdout`。

### 3.4 Manifest 标准

Rust 工具 manifest 至少要满足：

```yaml
id: your_tool
name: 你的工具名
kind: rust
category: Rust工具 > 你的分类
icon: rust
description: 一句话描述
docs:
  summary: 简短摘要
  usage: |
    使用说明
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
  entry: tools/rust_tools/your_tool/src/lib.rs
```

约束如下：

- `id` 必须全局唯一，且与 crate 名一致。
- `kind` 必须是 `rust`。
- `source.entry` **必须指向 `src/lib.rs`**（不是 `src/main.rs`），文件真实存在且是 library crate 入口。
- `category`、`icon`、`description` 不能留空。
- `execution.local.adapter` 必须是 `rust-binary`。
- `execution.remote.strategy` 必须是 `upload-binary-and-run`。
- `export.strategy` 必须是 `export-binary`。

### 3.5 CLI 参数标准

当前 Rust 工具的参数输入来自桌面宿主表单和原始参数字符串，因此必须满足：

- 所有对外暴露的参数都能映射到 manifest `params`。
- CLI 入口必须支持稳定的非交互模式。
- 参数错误要通过 `Result::Err` 返回，由 wrapper crate 的主函数输出到 stderr 并以非零码退出。
- 不能依赖交互式多轮输入才能完成主流程。

当前 `app/internal/adapter/` 的 rust-binary adapter 会把形如 `-voxel-size` 的单短横线长参数自动归一化为 `--voxel-size`，这是为了兼容现有 manifest 参数串装配方式；但新增 Rust 工具仍应优先按照标准 GNU long flag 设计 CLI。

### 3.6 构建标准

Rust 构建当前统一走 `app/internal/builder/pack_rust.go`，也用于 `scripts/build.go` 的分发产物构建。

构建流程：

1. 根据 `source.entry`（指向 `src/lib.rs`）向上查找 crate 根目录（包含 `Cargo.toml` 的目录）。
2. 创建临时 wrapper crate 目录，生成 `Cargo.toml`（path dependency 指向工具 crate）和 `src/main.rs`（调用 `::run()`）。
3. 在 wrapper 目录下执行构建。

构建命令：

- 原生宿主平台构建：

```bash
cargo build --release
```

- 跨平台构建使用 `cargo-zigbuild`（在 wrapper crate 目录执行）：

```bash
cargo zigbuild --release --target <triple>
```

当前支持的目标平台映射为：

- `linux/amd64` -> `x86_64-unknown-linux-musl`
- `linux/arm64` -> `aarch64-unknown-linux-musl`
- `darwin/amd64` -> `x86_64-apple-darwin`
- `darwin/arm64` -> `aarch64-apple-darwin`
- `windows/amd64` -> `x86_64-pc-windows-gnu`
- `windows/arm64` -> `aarch64-pc-windows-gnullvm`

交叉编译前会自动执行 `rustup target add <triple>` 安装目标。

构建产物默认输出到 `assets/rust/<os>_<arch>/` 目录下。`build.go` 中的 `bundledRustTools` 列表声明哪些工具随宿主分发。

这意味着新增 Rust 工具必须满足以下隐含要求：

- 能在 musl 目标下完成 Linux 交叉编译
- 能在 `cargo-zigbuild` 语义下通过链接
- 不要偷偷依赖只在宿主平台存在的外部文件布局
- library crate 的依赖必须能通过 wrapper crate 的 path dependency 正确解析

### 3.7 缓存标准

Rust 构建缓存会根据以下输入计算：

- `tool_id`
- 目标平台
- 目标 triple
- native / cross 模式
- wrapper crate 的 `Cargo.toml` 和 `main.rs` 生成内容（保证 wrapper 版本变更也能使缓存失效）
- 工具 library crate 下的 Rust 相关输入文件

当前纳入缓存 key 的输入包括：

- `Cargo.toml`
- `Cargo.lock`
- `.cargo/config.toml`
- `*.rs`

因此：

- 修改源码、锁文件或 cargo 配置，都会使缓存失效
- `target/` 目录不会参与缓存 key，也不会被扫描

### 3.8 本地执行验收标准

一个 Rust 工具接入后，至少要验证：

- 当前平台存在随宿主分发的本地产物时，本地执行可直接成功
- 删除本地产物、保留源码工作区时，本地执行可触发现场构建并成功运行
- 参数错误时，日志面板能看到明确失败信息
- 取消任务时，不会把宿主任务状态卡死

### 3.9 远程执行与导出验收标准

至少要分别验证：

- 远程执行前能够成功构建目标平台单工具二进制
- 单工具导出能产出目标平台二进制
- 产物中心能命中或写入 Rust 构建缓存
- 缺少 Rust / Zig / `cargo-zigbuild` / targets 时，错误会明确指向环境问题，而不是泛化成“执行失败”

## 4. 最小 Rust 工具接入骨架

### 4.1 目录结构

```text
tools/rust_tools/example_tool/
├── Cargo.toml
├── Cargo.lock
└── src/
    └── lib.rs
```

注意：没有 `src/main.rs`。

### 4.2 `Cargo.toml`

```toml
[package]
name = "example_tool"
version = "0.1.0"
edition = "2024"

[lib]
name = "example_tool"
path = "src/lib.rs"

[dependencies]
anyhow = "1"
clap = { version = "4", features = ["derive"] }
```

### 4.3 `src/lib.rs`

```rust
use anyhow::{Context, Result};
use clap::Parser;

#[derive(Parser, Debug)]
#[command(author, version, about = "示例 Rust 工具")]
struct Args {
    #[arg(long)]
    input: String,
}

pub fn run(args: &[String]) -> Result<()> {
    let cli = Args::parse_from(args);

    // 工具主逻辑
    let path = std::path::Path::new(&cli.input);
    if !path.exists() {
        anyhow::bail!("输入文件不存在: {}", cli.input);
    }

    println!("input = {}", cli.input);
    Ok(())
}
```

### 4.4 Manifest 模板

```yaml
id: example_tool
name: 示例 Rust 工具
kind: rust
category: Rust工具 > 示例
icon: rust
description: 示例 Rust 工具
docs:
  summary: 示例 Rust 工具
  usage: |
    用于说明最小 Rust 工具如何接入宿主。
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

关键点：`source.entry` 指向 `src/lib.rs`，不是 `src/main.rs`。

### 4.5 如何添加新工具

1. 创建目录 `tools/rust_tools/<tool_id>/`。
2. 编写 `Cargo.toml`（含 `[lib]` 段，不含 `[[bin]]`）。
3. 编写 `src/lib.rs`（实现 `pub fn run(args: &[String]) -> Result<()>`，无 `fn main()`）。
4. 在 `libs/catalog/builtin/manifests/` 下创建 `<tool_id>.yaml`，`source.entry` 指向 `tools/rust_tools/<tool_id>/src/lib.rs`。
5. 如需随宿主分发，在 `scripts/build.go` 的 `bundledRustTools` 列表中添加：

```go
var bundledRustTools = []rustTool{
    {
        ID:       "bxn_delivery_point_cloud_qc",
        CrateDir: filepath.Join("tools", "rust_tools", "bxn_delivery_point_cloud_qc"),
    },
    {
        ID:       "example_tool",
        CrateDir: filepath.Join("tools", "rust_tools", "example_tool"),
    },
}
```

6. 重新运行 `go run scripts/build.go` 完成全量构建验证。

### 4.6 验证步骤

```bash
# 1. 检查 library crate 可正常编译
cd tools/rust_tools/example_tool && cargo build

# 2. 验证 run 函数签名
grep -n "pub fn run" src/lib.rs

# 3. 确认没有 main.rs
test ! -f src/main.rs || echo "错误: 不能有 src/main.rs"

# 4. 确认 Cargo.toml 没有 [[bin]]
grep -c "\[\[bin\]\]" Cargo.toml || true

# 5. 通过宿主构建验证
cd <repo_root> && go run scripts/build.go

# 6. 检查产物
ls -la build/image/host/assets/rust/<os>_<arch>/
```

## 5. 当前不支持的做法

- 不支持 Rust 工具使用 binary crate（`[[bin]]` + `src/main.rs`）
- 不支持 `src/lib.rs` 中定义 `fn main()`
- 不支持只提供预编译二进制而没有 crate 源码入口
- 不支持 manifest 缺少 `kind: rust`
- 不支持 `source.entry` 指向 `src/main.rs`
- 不支持 Rust 工具接入 `legacy.go` 的 Go bridge 模型
- 不支持远程执行时在远端现场跑 `cargo build`
- 不支持把 `cargo-zigbuild` 或 targets 当成独立 SDK 让用户手动切换

## 6. 相关文档

- [Rust / Zig 环境使用说明](./RUST_ENVIRONMENT.md)
- [当前架构](./ARCHITECTURE.md)
- [开发说明](./DEVELOPMENT_SETUP.md)
- [ADR 0024: Rust 工具链由宿主管理](./adr/0024-rust-toolchain-managed-by-host.md)
