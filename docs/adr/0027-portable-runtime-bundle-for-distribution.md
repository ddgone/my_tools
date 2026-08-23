# ADR 0027: 单层交付形态与预编译产物的便携捆绑分发

## 日期

2026-08-23

## 状态

已采纳（已实现，经 `go run scripts/build.go` + `go run ./scripts/package` 实机验证）

## 背景

应用定位是把「点云/LiDAR 领域的 Go 与 Rust 工具」连同宿主一起交付给同事使用。构建已具备预编译能力，但**工具的执行路径依赖源码工作区与本机工具链**，交付目标（让不会交叉编译的同事开箱即用）仍未达成。同时存在「构建」与「交付」耦合、开发时目录像开发态产物而非正式软件的问题。

梳理出的关键问题：

1. **执行路径不消费预编译产物**。
   宿主执行编译型工具的统一入口是 `resolveLocalBinary`（`app/internal/execution/binary.go`），逻辑是「调用 `builder.BuildPackage` → 命中缓存则用缓存，未命中就**在源码工作区现场交叉编译**」。预置产物即使随包分发，宿主也不会优先直接使用。

2. **无仓库工作区时直接报错**。
   `resolveLocalBinary` 先 `LocateRepoRoot()`，失败返回「未找到源码工作区」。交付机器上没有源码仓库。

## 决策

采用**单层交付形态**：`build/` 目录本身即交付安装包展开后的样子——**「本地 build 什么，交付就是什么」**，保证一致性与可复现。

- **构建（`go run scripts/build.go`，开发者日常跑）**：只产出可运行交付目录，**不打任何安装包**。
- **打包（`go run ./scripts/package <version>`，交付时跑）**：把整个 `build/` 目录打成一个 zip（交付脚本独立的新脚本）。

不追求「单文件大 exe」（Wails 宿主 + 独立工具二进制无法真正塞进一个可执行文件；做到单文件也只是自解压，徒增杀软误报与权限问题）。

### 分发形态（单层，build/ 即交付根）

```
build/                                ← 交付根，整体拷走/压缩即用
├── fire-salamander-desktop(.exe)     ← Wails 桌面宿主（从 exe 位置推导根目录）
├── assets/                           ← 只读程序资产（splash 等）
│   └── splash.png
├── program/                          ← 只读：预编译工具产物
│   └── tools/<toolID>/<platform>/artifact/<toolID>_<os>_<arch>[.exe]
└── data/                             ← 可写用户数据（细分明确，交付包仅保留顶层空目录）
    ├── config/                       # 默认 app.json / ssh_connections.json / execution.db
    ├── cache/scripts/                # 运行时产生的脚本副本
    ├── cache/builds/                 # 现场构建缓存（工具缺失时回退构建的落点）
    ├── logs/
    └── exports/
```

### 程序区/数据区分区的依据

- **program/ 为只读**：预编译工具产物。升级 = 替换 exe 与 program/，不动 data/。
- **data/ 为可写**：config / logs / exports / 脚本副本 / 现场构建缓存 / 托管工具链。备份与迁移只关心 data/。
- 二者在目录上肉眼可分，规避「程序文件与用户数据同目录混乱」的开发态气味。
- 关键特性：**不带工具链、开箱即用**。工具全部预编译进 `program/tools`，宿主执行不依赖本机 go/cargo/zig；需要现场构建新工具时由用户在应用内安装托管工具链（落在 `data/toolchains`）。
- **预编译目标组合按宿主平台**（`resolveToolTargets`）：Windows 宿主编译 `windows(宿主架构) + linux/amd64`；macOS 宿主编译 `darwin(宿主架构) + linux/amd64`。linux/amd64 产物供远程 Linux 执行使用；`--all` 可显式全平台。

## 实现要点

### 1. 目录解析：宿主从 exe 位置自动推导便携根

`app/internal/runtimeenv/layout.go` 的 `ResolveLayout` 已新增**便携部署态**最高优先级，并把仓库模式同步指向单层根：

```
ResolveLayout()（优先级从高到低）
1. 便携部署态：exe 旁存在 program/、data/ → Root 即 exe 所在目录
   - Layout.Root      = exeDir/data（可变数据根）
   - Layout.ProgramRoot = exeDir/program（只读根）
2. FIRE_SALAMANDER_RUNTIME_DIR（保留，一次性兜底/升级兼容）
3. 仓库内：Root = build/data，ProgramRoot = build/program（开发者模式，与交付根同构）
4. 用户目录 ~/.fire-salamander（兜底）
```

- `.Root` 与 `ProgramRoot` 解耦：只读预置（program）与可写数据（data）分开。
- `ProgramToolsDir()` 指向 `program/tools`；`BuildCacheDir()` 指向 `data/cache/builds`（现场构建缓存落点）。

### 2. 执行路径优先用预置产物（已实现）

修改 `app/internal/execution/binary.go` 的 `resolveLocalBinary`：

1. **不再把 `LocateRepoRoot()` 作为硬性前置**。先尝试 `builder.ResolveProgramToolPath(layout.ProgramToolsDir(), ...)` 按「工具ID + 目标平台」取预编译产物命中即返回。
2. 未命中 → 才回退到 `builder.BuildPackage` 现场构建（`LocateRepoRoot` + 工具链），落点 `data/cache/builds`。

```
resolveLocalBinary(manifest):
    layout = ResolveLayout()
    if kind = ManifestKindToBuilderKind(manifest.Kind):
        if path, ok = resolveProgramPrebuilt(layout, manifest.ID, kind):  # program/tools/<id>/<platform>/...
            return path
    # 回退：需要仓库源码 + 工具链（开发者模式）
    return builder.BuildPackage(OutputDir=layout.BuildCacheDir(), ...)
```

- `builder.ResolveProgramToolPath`（`pack.go`）按 ToolID+平台直接取件，**不读 SourceEntry、不依赖仓库**。
- `builder.ProbeBuildCache` 虽存在，但其 `probeGoCache`/`probeRustCache` 会 `resolveSourceEntryPath` 隐依赖源码路径，故预置命中走 `ResolveProgramToolPath` 绕开。

### 3. 构建产出即单层交付形态（已实现）

`scripts/build.go`：
- 产物直接落到 `build/` 根：exe + `assets/` + `program/` + `data/`。
- `buildToolCache` 直接把内置工具预编译到 `build/program/tools`（结构 `<toolID>/<platform>/artifact/…`，与 `ResolveProgramToolPath` 一致），**无二次拷贝**。
- 清理逻辑 `cleanBuildRootStale` 只清可重建的程序区（exe/assets/program），**保留 `data/` 用户数据**。
- 不再生成 `build/image`、`build/runtime` 双层；不再内嵌打包逻辑。

### 4. 纯净打包（已实现，scripts/package/main.go）

`go run ./scripts/package <version>` 新增 `packageZipDir`：把整个 `build/` 打成 zip：
- zip 根即 `build/` 内容，交付解压展开 = 本地 `build/` 形态。
- 排除输出目录 `exports/`、打包输出文件自身（避免把 zip 打进 zip）。
- `data/` 整体不进包（设置、使用记录、缓存、托管工具链均为运行时数据），仅保留顶层空目录占位；应用启动时 `Ensure()` 自建子目录，配置缺失走内置默认值。
- 排除运行时 sqlite 临时文件（`-shm` / `-wal` / `.tmp`）。

### 5. Rust 交叉编译的 zig 发现与缓存隔离（已实现）

`scripts/build.go` 的 `rustBuildEnv`：交叉编译（`cargo zigbuild`）时自动发现**托管 zig**（`build/data/toolchains/zig/<version>/zig.exe`，经 `CARGO_ZIGBUILD_ZIG_PATH` 显式指定，系统 PATH 已装则优先），并把 cargo-zigbuild 与 zig 的缓存（`CARGO_ZIGBUILD_CACHE_DIR` / `ZIG_GLOBAL_CACHE_DIR`）指到 `tools/rust_tools/zigbuild_cache/`——不写用户全局目录（`%LOCALAPPDATA%`），构建可重复且不污染系统。

## 影响

- **正面**：交付机器无需源码仓库、无需交叉编译、无需本机工具链即可运行内置工具。
- **正面**：宿主 exe 双击即用，不依赖 `.bat` 与 `FIRE_SALAMANDER_RUNTIME_DIR` 环境变量。
- **正面**：构建与交付解耦——`build.go` 不吐包，交付脚本独立触发 zip。
- **正面**：`program/`（只读）与 `data/`（可写）分区清晰，升级=替换 exe+program，备份=整个目录拷走。
- **正面**：dev 与交付共用同一单层 `build/` 形态，本地测什么交付就是什么，杜绝「本地能跑、交付缺东西」。
- **代价**：需要现场构建新工具的机器必须自装工具链（应用内可装托管 Go / Rust+zig），交付包不含工具链是体积与灵活性的取舍（不带工具链的包约 60+ MB）。
- **代价**：便携自包含意味着数据留在安装目录，不写入系统 `%APPDATA%`——刻意的绿色/可迁移取舍。
- **代价**：预置产物与源码版本可能不同步——用 cache key 标注产物对应的源码版本；增量更新时整体刷新 program/。

## 验证方式（已实测通过）

1. `go run scripts/build.go`：产物落在 `build/` = `exe + assets + program/tools/<id>/<platform>/artifact/* + data/{config,logs,exports,cache}`，无 `image/runtime` 双层；Windows 宿主下 14 个内置工具均有 `windows_amd64` 与 `linux_amd64` 双平台产物。
2. `go run ./scripts/package 1.0.0`：生成 `build/exports/fire-salamander-*.zip`（约 63 MB），zip 内 = exe + assets + program/tools（全预编译工具双平台）+ 空目录 `data/`，无 `exports/`、无 zip 自身、无任何 toolchain 内容。
3. `go test ./app/internal/runtimeenv ./app/internal/execution ./app/internal/builder` 覆盖便携根推导、预置产物命中优先、未命中回退构建三分支，全部通过。
4. 干净的交付机器（无 go/cargo/zig、无仓库）解压 zip → 双击 exe → 执行内置 Go 工具与内置 Rust 工具均成功。

## 关联

- [ADR 0014: 开发态运行时目录镜像安装态目录形态](./0014-runtime-layout-mirrors-installed-shape.md)
- [ADR 0015: 安装镜像目录与运行时数据分离](./0015-separate-install-image-from-runtime-data.md)
- [ADR 0024: Rust 工具链由宿主托管](./0024-rust-toolchain-managed-by-host.md)
- [ADR 0025: 统一编译型工具集成](./0025-unified-compiled-tool-integration.md)