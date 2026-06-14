# Rust 工具完整接入标准

本文档定义当前仓库里 Rust 工具接入桌面宿主的完整标准。

范围只覆盖当前已经真实落地的接入方式，不讨论未来可能存在但尚未闭环的理想方案。本文以现有 Rust 工具 `tools/rust_tools/las_voxelizer/` 为准。

## 1. 目标

一个新的 Rust 工具要被桌面宿主正确接入，必须同时满足以下 5 件事：

1. 工具源码以标准 Rust crate 形式存在于 `tools/rust_tools/` 下。
2. 内置 manifest 能为前端提供名称、参数、说明、执行策略、导出策略和源码入口。
3. 本地执行能找到随宿主分发的本地产物，或在源码工作区下按当前 Rust 环境现场构建本机产物。
4. 远程执行链路能依据 manifest 的 `source.entry` 构建目标平台单工具产物。
5. 单工具导出和产物中心都能通过统一 `builder.BuildPackage(...)` 产出 Rust 二进制。

只完成其中一部分都不算“完整接入”。

## 2. 当前真实接入模型

当前 Rust 工具采用的是“manifest 驱动 + builder 构建 + 专用本地执行适配层”的模型。

### 2.1 工具源码层

Rust 工具当前放在：

```text
tools/rust_tools/<tool_id>/
```

最低要求：

- 必须是一个标准 Rust crate
- 必须包含 `Cargo.toml`
- 必须能从 manifest 的 `source.entry` 追溯到 crate 根目录
- 当前二进制名必须与 `tool_id` 保持一致

当前不要求 Rust 工具实现 Go 侧的 `framework.Tool`，也不要求像 Go 工具那样走 `legacy.go` 的桥接注册。

### 2.2 Manifest 层

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
- `source.entry` 不是可选增强项，而是本地现场构建、远程执行和导出的必要输入。

### 2.3 本地执行层

本地执行入口当前在 `app/rust_tools.go`。

当前逻辑是：

1. 先尝试从随宿主分发的本地产物目录中解析当前平台二进制。
2. 如果当前运行在源码工作区且未找到已打包产物，则转入 `builder.BuildPackage(...)`。
3. 使用 manifest 的 `source.entry` 定位 crate，并基于当前选中的 Rust / Zig 环境构建宿主平台产物。
4. 直接执行得到的本机二进制。

这意味着：

- Rust 工具本地执行不是 Go 工具那种“宿主内 bridge 闭包”。
- Rust 工具本地执行也不是 Python 工具那种“解释器 + 托管 venv”。
- 它是“优先使用打包好的宿主平台二进制，源码工作区下允许现场构建”的模型。

### 2.4 远程执行与导出

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

推荐最小结构：

```text
tools/rust_tools/<tool_id>/
├── Cargo.toml
├── Cargo.lock
└── src/
    └── main.rs
```

约束如下：

- `Cargo.toml` 中 crate 名建议直接与 `tool_id` 一致
- 产出的二进制名必须和 `tool_id` 一致，否则宿主找不到构建结果
- `target/` 必须继续忽略，不得把 Rust 构建产物提交进仓库

### 3.2 Manifest 标准

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
execution:
  local:
    adapter: rust-binary
  remote:
    strategy: upload-binary-and-run
export:
  strategy: export-binary
source:
  entry: tools/rust_tools/your_tool/src/main.rs
```

约束如下：

- `id` 必须全局唯一
- `id` 必须与 crate 产出的二进制名一致
- `kind` 必须是 `rust`
- `source.entry` 必须指向真实存在的 `.rs` 入口文件
- `category`、`icon`、`description` 不能留空，否则前端体验会明显退化

### 3.3 CLI 参数标准

当前 Rust 工具的参数输入来自桌面宿主表单和原始参数字符串，因此必须满足：

- 所有对外暴露的参数都能映射到 manifest `params`
- CLI 入口必须支持稳定的非交互模式
- 参数错误要通过标准输出 / 标准错误明确暴露，不要做静默失败
- 不能依赖交互式多轮输入才能完成主流程

当前 `app/rust_tools.go` 会把形如 `-voxel-size` 的单短横线长参数自动归一化为 `--voxel-size`，这是为了兼容现有 manifest 参数串装配方式；但新增 Rust 工具仍应优先按照标准 GNU long flag 设计 CLI。

### 3.4 构建标准

Rust 构建当前统一走 `app/internal/builder/pack_rust.go`。

当前约束如下：

- 原生宿主平台构建使用：

```bash
cargo build --release
```

- 跨平台构建使用：

```bash
cargo zigbuild --release --target <triple>
```

- 当前支持的目标平台映射为：
  - `linux/amd64` -> `x86_64-unknown-linux-musl`
  - `linux/arm64` -> `aarch64-unknown-linux-musl`
  - `darwin/amd64` -> `x86_64-apple-darwin`
  - `darwin/arm64` -> `aarch64-apple-darwin`
  - `windows/amd64` -> `x86_64-pc-windows-gnu`
  - `windows/arm64` -> `aarch64-pc-windows-gnullvm`

这意味着新增 Rust 工具必须满足以下隐含要求：

- 能在 musl 目标下完成 Linux 交叉编译
- 能在 `cargo-zigbuild` 语义下通过链接
- 不要偷偷依赖只在宿主平台存在的外部文件布局

### 3.5 缓存标准

Rust 构建缓存会根据以下输入计算：

- `tool_id`
- 目标平台
- 目标 triple
- native / cross 模式
- crate 下的 Rust 相关输入文件

当前纳入缓存 key 的输入包括：

- `Cargo.toml`
- `Cargo.lock`
- `.cargo/config.toml`
- `*.rs`

因此：

- 修改源码、锁文件或 cargo 配置，都会使缓存失效
- `target/` 目录不会参与缓存 key，也不会被扫描

### 3.6 本地执行验收标准

一个 Rust 工具接入后，至少要验证：

- 当前平台存在随宿主分发的本地产物时，本地执行可直接成功
- 删除本地产物、保留源码工作区时，本地执行可触发现场构建并成功运行
- 参数错误时，日志面板能看到明确失败信息
- 取消任务时，不会把宿主任务状态卡死

### 3.7 远程执行与导出验收标准

至少要分别验证：

- 远程执行前能够成功构建目标平台单工具二进制
- 单工具导出能产出目标平台二进制
- 产物中心能命中或写入 Rust 构建缓存
- 缺少 Rust / Zig / `cargo-zigbuild` / targets 时，错误会明确指向环境问题，而不是泛化成“执行失败”

## 4. 最小 Rust 工具接入骨架

### 4.1 目录骨架

```text
tools/rust_tools/example_tool/
├── Cargo.toml
└── src/
    └── main.rs
```

### 4.2 `Cargo.toml`

```toml
[package]
name = "example_tool"
version = "0.1.0"
edition = "2024"

[dependencies]
clap = { version = "4", features = ["derive"] }
```

### 4.3 `src/main.rs`

```rust
use clap::Parser;

#[derive(Parser, Debug)]
struct Args {
    #[arg(long)]
    input: String,
}

fn main() {
    let args = Args::parse();
    println!("input = {}", args.input);
}
```

### 4.4 Manifest 骨架

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
  entry: tools/rust_tools/example_tool/src/main.rs
```

## 5. 当前不支持的做法

- 不支持只提供预编译二进制而没有 crate 源码入口
- 不支持 manifest 缺少 `kind: rust`
- 不支持 Rust 工具接入 `legacy.go` 的 Go bridge 模型
- 不支持远程执行时在远端现场跑 `cargo build`
- 不支持把 `cargo-zigbuild` 或 targets 当成独立 SDK 让用户手动切换

## 6. 相关文档

- [Rust / Zig 环境使用说明](./RUST_ENVIRONMENT.md)
- [当前架构](./ARCHITECTURE.md)
- [开发说明](./DEVELOPMENT_SETUP.md)
- [ADR 0024: Rust 工具链由宿主管理](./adr/0024-rust-toolchain-managed-by-host.md)
