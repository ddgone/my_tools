# 开发说明

本文档只保留当前可执行、已验证的开发命令和入口路径。

补充说明：本文说的“开发环境要求”是指开发这个仓库本身所需的本机环境；它不等同于桌面宿主运行时面向用户暴露的 Go 环境配置能力。

## 1. 环境要求

- Go 1.26+
- Node.js 22+
- npm 10+
- Windows 需可用 WebView2 运行时
- 已安装 Wails CLI

安装 Wails CLI：

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

## 2. 前端依赖安装

```bash
cd app/frontend
npm install
```

## 3. 启动开发模式

```bash
cd app
"$(go env GOPATH)/bin/wails" dev
```

如果 `wails` 没有加入环境变量，可直接调用 Go bin 目录中的可执行文件。

## 4. 构建桌面应用

推荐使用仓库根目录构建脚本：

```bash
go run scripts/build.go
```

构建脚本会：

- 产出宿主程序到 `build/image/host/`
- 初始化开发态运行时目录 `build/runtime/`
- 写入默认配置文件
- 为随宿主分发的 Rust 工具准备本地产物资源
- 执行 Go 静态检查，但只扫描项目源码目录，不再把 `build/runtime/toolchains/` 下的托管 SDK 源码一起纳入 `go vet`

如果你直接启动 `build/image/host/*.app`，运行时目录仍会优先识别回仓库内的 `build/runtime/`，因此托管 Go SDK 的默认下载位置会落在：

```text
build/runtime/toolchains/
```

如需打安装包：

```bash
go run scripts/package/main.go
go run scripts/package/main.go 1.0.0
```

## 5. 质量检查

以下命令已经在本仓库复核通过：

```bash
cd app && go test ./...
go vet ./libs/... ./tools/... ./scripts/...
cd app && go vet ./...
cd app/frontend && npm run lint
cd app/frontend && npm run typecheck
```

- `go test` 请在 `app/` 模块内执行。
- 不要在仓库根直接运行 `go test ./...`，否则会把 `build/runtime/toolchains/` 下托管 Go SDK 自带的测试源码一起扫进去，产生与项目无关的噪音。

## 6. Go 环境配置现状

- 开发者构建这个仓库仍然需要本机 Go、Node、Wails CLI。
- 桌面宿主运行后，用户可以在系统首选项的 Go 页签里：
  - 选择本地 Go
  - 下载官方 Go SDK
  - 查看下载任务进度、速度，并在下载中停止
  - 切换已检测到的 Go
  - 检查当前 Go 环境
  - 删除当前托管 Go 环境
  - 选择 `<无 SDK>` 禁用当前 Go 环境
- Go 工具的本地执行不依赖这套运行时 Go 环境配置；真正依赖它的是：
  - 远程执行前的单工具构建
  - Go 单工具导出
  - 构建缓存准备

用户视角的说明、默认下载位置和 `<无 SDK>` 语义，统一见 [GO_ENVIRONMENT.md](./GO_ENVIRONMENT.md)。

## 7. Python 环境配置现状

- 当前桌面宿主已经支持在系统首选项的 Python 页签里：
  - 选择本地 Python 3 作为基础解释器
  - 切换已检测到的基础 Python
  - 创建或重建托管虚拟环境
  - 选择 `<无 Python>` 禁用当前 Python 环境
  - 按内置 Python 工具脚本的动态扫描结果一键安装 Python 包
- 当前暂不支持托管 Python 下载。
- Python 工具的本地执行会直接依赖这套运行时 Python 环境配置。
- 当前实现使用“基础 Python + 托管 venv + 动态依赖扫描”模型，依赖安装统一通过托管虚拟环境执行 `python -m pip install ...`。
- 动态依赖扫描的映射与标准库数据当前维护在 `app/internal/toolchain/pythondata/` 下：
  - `mapping.txt`：导入名到 pip 包名映射
  - `stdlib.txt`：标准库模块列表
- 这两份数据当前来源于 `pipreqs`，建议后续定期同步更新，尤其是在接入新 Python 工具或遇到新的包名映射问题之后。

用户视角的说明、动态依赖口径和 `<无 Python>` 语义，统一见 [PYTHON_ENVIRONMENT.md](./PYTHON_ENVIRONMENT.md)。

## 8. Rust 环境配置现状

- 当前桌面宿主已经支持在系统首选项的 Rust 页签里：
  - 在 `<无 SDK>`、自动探测、手动选择 之间切换
  - 选择 Rust 环境目录
  - 选择 Zig SDK
  - 下载托管 Rust
  - 下载托管 Zig
  - 一次性下载 Rust + Zig
  - 为当前托管 Rust 补齐 `cargo-zigbuild`
  - 为当前托管 Rust 补齐常用交叉编译 targets
- Rust 环境配置当前影响：
  - Rust 工具在源码工作区下的本机现场构建
  - Rust 工具的远程执行前单工具构建
  - Rust 单工具导出
  - Rust 构建缓存准备
- Rust 工具的本地执行优先使用随宿主分发的本地产物；开发态如果没有打包产物，才会现场调用当前选中的 Rust / Zig 环境构建宿主平台二进制。
- 当前托管 Rust / Zig 默认落在运行时目录下：

```text
build/runtime/toolchains/rust/
build/runtime/toolchains/zig/
```

- 系统 Rust 的 `cargo-zigbuild` 与 targets 补齐默认受保护；当前宿主优先引导用户切到托管 Rust 后再执行补齐。

用户视角的说明、默认安装路径和能力补齐语义，统一见 [RUST_ENVIRONMENT.md](./RUST_ENVIRONMENT.md)。

## 9. 常用入口文件

### 宿主入口

- `app/main.go`
- `app/app.go`
- `app/execution.go`
- `app/rust_tools.go`
- `app/legacy.go`

### 后端内部包

- `app/internal/ssh/store.go`
- `app/internal/runtime/remote.go`
- `app/internal/builder/pack.go`
- `app/internal/builder/pack_rust.go`
- `app/internal/toolchain/rustenv.go`
- `app/internal/toolchain/rustinstall.go`
- `app/internal/runtimeenv/layout.go`

### 前端入口

- `app/frontend/src/main.ts`
- `app/frontend/src/App.vue`
- `app/frontend/src/components/WorkspaceLayout.vue`
- `app/frontend/src/components/WorkspaceTabs.vue`
- `app/frontend/src/stores/workspace.ts`

### 工具规格与清单

- `libs/core/toolspec/types.go`
- `libs/catalog/builtin/service.go`
- `libs/catalog/builtin/manifests/*.yaml`

## 10. 开发注意事项

- 不要再引用 `app/desktop/`、`HomeView.vue`、`ExecuteView.vue` 等旧路径或旧页面名。
- 当前前端是单页工作台，不再使用 Vue Router。
- 不要把 `go build ./...` 生成的普通可执行文件当成正式桌面产物。
- 不要把运行时目录下的 `toolchains/` 当成源码工作区的一部分，也不要再用仓库根的 `go vet ./...` 直接扫描整个仓库。
- Python 环境配置当前已经切到托管虚拟环境；如果后续要引入托管 Python 下载，需要先更新当前文档和用户说明。
- Rust 工具的源码资产当前位于 `tools/rust_tools/`；如新增 Rust 工具，需要同步补 manifest、图标/分类、必要时补随宿主构建资源与接入说明。
- Rust 交叉编译链当前依赖 `rustup`、`cargo-zigbuild`、Zig 与常用 targets；如果调整目标平台矩阵或安装模型，必须同步更新 `RUST_ENVIRONMENT.md` 与 `RUST_TOOL_INTEGRATION_STANDARD.md`。
- 新发现的问题先登记到 `docs/ISSUES_AND_REMEDIATION_PLAN.md`，再决定是否进入实现排期。
