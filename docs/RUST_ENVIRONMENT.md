# Rust / Zig 环境使用说明

本文档只说明当前已经落地的 Rust / Zig 环境能力，以及它在桌面宿主中的真实影响范围。

## 1. 先说结论

- Rust 环境当前主要服务于 Rust 工具的构建链路，不影响 Go / Python 工具。
- Rust 工具的本地执行优先复用随宿主分发的本地产物；如果当前运行在源码工作区且未找到本地产物，宿主才会基于当前选中的 Rust / Zig 环境现场构建宿主平台二进制。
- Rust 环境当前影响四类能力：
  - Rust 工具在源码工作区下的本机现场构建
  - Rust 工具远程执行前的单工具构建
  - Rust 单工具导出
  - Rust 构建缓存准备
- 系统首选项里的 Rust 页签当前支持：
  - `<无 SDK>` / 自动探测 / 手动选择 三种模式
  - 选择 Rust 环境目录
  - 选择 Zig SDK
  - 下载托管 Rust
  - 下载托管 Zig
  - 下载 Rust + Zig
  - 为当前 Rust 补齐 `cargo-zigbuild`
  - 为当前 Rust 补齐常用交叉编译 targets

## 2. 为什么不是选四个二进制

当前 Rust 设置页不再让用户分别选择：

- `cargo`
- `rustup`
- `rustc`
- `cargo-zigbuild`

而是收口成两个主对象：

- Rust SDK
- Zig SDK

原因是：

- Rust 环境本质上是一个目录布局，而不是单独一个 `cargo` 路径。
- `cargo`、`rustc`、`rustup` 是同一 Rust 环境里的派生成员。
- `cargo-zigbuild` 与常用 targets 也不适合作为主 SDK；它们应该依附于当前 Rust 环境检测和补齐。

所以当前产品模型是：

- 模式：`<无 SDK>` / 自动探测 / 手动选择
- 主配置：Rust 环境目录 + Zig SDK
- 派生能力：`cargo-zigbuild` + 常用交叉编译 targets

## 3. 三种模式分别是什么意思

### 3.1 `<无 SDK>`

- 这是用户主动清除当前 Rust 环境，而不是检测失败。
- 进入这个状态后：
  - Rust 工具的源码工作区现场构建会被阻断
  - Rust 远程执行前构建会被阻断
  - Rust 导出会被阻断
  - Go / Python 工具不受影响
- 宿主不会自动回退到 PATH 或已发现的 Rust。

### 3.2 自动探测

- 宿主会从已配置路径、历史已知路径、PATH、常见系统位置和托管目录里收集候选。
- 当前会优先使用托管 Rust 作为自动探测候选，避免系统 Rust 抢占当前环境。
- Zig 也会独立探测，不要求和 Rust 来自同一个目录。

### 3.3 手动选择

- Rust 选择的是“Rust 环境目录”，不是某个单独的 `cargo` 可执行文件。
- Zig 选择的是 `zig` 可执行文件。
- Rust 环境目录既可以是托管目录，也可以是本机已有的 `.cargo` / 自定义工具链目录。

## 4. 用户能做什么

### 4.1 选择 Rust 环境目录

- 选择后，宿主会尝试解析这个目录里的：
  - `cargo`
  - `rustup`
  - `rustc`
  - `cargo-zigbuild`
- 如果目录布局可识别，会立即作为当前 Rust SDK 生效。
- 如果目录不可识别，会在设置页和任务卡片里给出明确错误。

### 4.2 选择 Zig SDK

- Zig 独立于 Rust 环境目录选择。
- 这允许你使用：
  - 托管 Rust + 系统 Zig
  - 系统 Rust + 托管 Zig
  - 托管 Rust + 托管 Zig

### 4.3 下载托管 Rust / Zig

- 当前支持三种下载动作：
  - 下载 Rust
  - 下载 Zig
  - 下载 Rust + Zig
- Rust 托管下载会优先尝试官方源，失败后自动回退到镜像源。
- Zig 托管下载也会优先官方，再按内置镜像策略回退。
- 安装完成后，新下载的托管环境会自动进入已知候选，并切到手动选择状态。

### 4.4 补齐 `cargo-zigbuild`

- 如果当前 Rust 缺少 `cargo-zigbuild`，设置页会显示补齐入口。
- 它依附当前 Rust 环境，而不是单独让用户切换某个 `cargo-zigbuild` 路径。
- 当前默认只对托管 Rust 开放自动补齐；系统 Rust 默认受保护，不会再被宿主无提示修改。

### 4.5 补齐常用 targets

- 当前默认关注 5 个常用交叉编译目标：
  - `x86_64-unknown-linux-musl`
  - `aarch64-unknown-linux-musl`
  - `x86_64-apple-darwin`
  - `x86_64-pc-windows-gnu`
  - `aarch64-pc-windows-gnullvm`
- 当前宿主平台对应的原生 target 会自动视为已具备，不要求额外安装。
- 首次构建时如果缺少 target，构建链路也会尝试自动执行 `rustup target add`。
- 设置页里的“补齐 targets”会逐个安装并在结束后复查，不再把半成功状态误报成完成。

## 5. 默认安装到哪里

Rust / Zig 的托管安装路径跟运行时目录绑定。

### 开发态

- 默认放在仓库内：

```text
build/runtime/toolchains/rust/
build/runtime/toolchains/zig/
```

- Rust 会按版本落子目录，例如：

```text
build/runtime/toolchains/rust/stable/
```

- Zig 会按版本落子目录，例如：

```text
build/runtime/toolchains/zig/0.16.0/
```

### 安装态

- 默认回退到用户目录运行时位置：

```text
~/.fire-salamander/toolchains/rust/
~/.fire-salamander/toolchains/zig/
```

## 6. 状态栏怎么看

底部状态栏里的 Rust 状态块是当前唯一的轻量环境入口。

### 已就绪

- 显示类似：

```text
Rust 已就绪 · cargo 1.96.0 · zig 0.16.0
```

- hover 会显示：
  - 当前 Rust 版本
  - 当前 Rust 环境目录
  - 当前 Zig 路径
  - `cargo-zigbuild` 与 targets 的就绪情况

### 未就绪

- 显示类似：

```text
Rust 未配置 · 仅导出/远程受影响
```

或：

```text
Rust 待补齐 · 点击查看
```

- 前者表示当前没有可用 Rust 环境。
- 后者表示已有 Rust，但 Zig、`cargo-zigbuild` 或常用 targets 仍未补齐。

## 7. 常见使用场景

### 只想本地运行已经随宿主打包的 Rust 工具

- 通常不需要单独配置 Rust 环境。
- 只要宿主内已经带了当前平台产物，本地执行可以直接工作。

### 想在源码工作区下调试 Rust 工具

- 需要配置一个可用的 Rust 环境。
- 如果是跨平台导出或远程执行，还需要 Zig、`cargo-zigbuild` 和相应 targets。

### 想远程执行 Rust 工具

- 需要可用 Rust 环境。
- 需要 Zig。
- 需要 `cargo-zigbuild`。
- 需要目标平台对应的 target。

### 想导出 Rust 工具

- 宿主平台原生导出通常只需要可用 Rust 环境。
- 跨平台导出还需要 Zig、`cargo-zigbuild` 与目标平台 target。

## 8. 排障建议

### 任务卡在“正在下载 rustup-init”

- 当前实现已经改为流式显示 `rustup-init` 后续安装输出。
- 如果还长时间停留在同一状态，优先看任务详情里是否已经切到“正在安装 Rust toolchain”。

### 设置页显示 targets 缺失，但你怀疑托管 Rust 其实已经装好了

- 当前读取 installed targets 已经基于当前激活 Rust 的完整环境布局执行。
- 如果仍然异常，优先确认当前 `Rust SDK` 选中的真的是托管目录，而不是系统 Rust。

### 为什么系统 Rust 不能直接点“补齐”

- 这是当前的安全保护。
- 之前系统 Rust 存在被误操作的风险，现在默认只对托管 Rust 开放自动补齐。

### 为什么下载失败后目录是空的

- 当前安装链路已经补了安装后验收，不会再把“空目录”误当成可用 Rust 环境继续往下补齐。

## 9. 验收清单

- 模式切到自动探测后，托管 Rust 优先成为当前 Rust
- 手动选择 Rust 环境目录后，状态栏和设置页同步更新
- 只下载 Rust、只下载 Zig、下载 Rust + Zig 三条路径都可用
- 托管 Rust 缺少 `cargo-zigbuild` 时能补齐
- 托管 Rust 缺少常用 targets 时能补齐并在结束后复查
- 切到 `<无 SDK>` 后，Rust 导出和远程执行前构建会被阻断
- 系统 Rust 当前不会再被宿主无提示补齐 `cargo-zigbuild` 或 targets

## 10. 相关文档

- [项目总览](./PROJECT_OVERVIEW.md)
- [当前架构](./ARCHITECTURE.md)
- [开发说明](./DEVELOPMENT_SETUP.md)
- [Rust 工具接入标准](./RUST_TOOL_INTEGRATION_STANDARD.md)
- [配置持久化统一说明](./CONFIG_PERSISTENCE_UNIFICATION_NOTES.md)
