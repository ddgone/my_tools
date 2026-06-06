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
go test ./...
go vet ./libs/... ./tools/... ./scripts/...
cd app && go vet ./...
cd app/frontend && npm run lint
cd app/frontend && npm run typecheck
```

## 6. Go 环境配置现状

- 开发者构建这个仓库仍然需要本机 Go、Node、Wails CLI。
- 桌面宿主运行后，用户可以在系统首选项的 Go 页签里：
  - 选择本地 Go
  - 下载官方 Go SDK
  - 切换已检测到的 Go
  - 选择 `<无 SDK>` 禁用当前 Go 环境
- Go 工具的本地执行不依赖这套运行时 Go 环境配置；真正依赖它的是：
  - 远程执行前的单工具构建
  - Go 单工具导出
  - 构建缓存准备

用户视角的说明、默认下载位置和 `<无 SDK>` 语义，统一见 [GO_ENVIRONMENT.md](./GO_ENVIRONMENT.md)。

## 7. 常用入口文件

### 宿主入口

- `app/main.go`
- `app/app.go`
- `app/execution.go`
- `app/legacy.go`

### 后端内部包

- `app/internal/ssh/store.go`
- `app/internal/runtime/remote.go`
- `app/internal/builder/pack.go`
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

## 8. 开发注意事项

- 不要再引用 `app/desktop/`、`HomeView.vue`、`ExecuteView.vue` 等旧路径或旧页面名。
- 当前前端是单页工作台，不再使用 Vue Router。
- 不要把 `go build ./...` 生成的普通可执行文件当成正式桌面产物。
- 不要把运行时目录下的 `toolchains/` 当成源码工作区的一部分，也不要再用仓库根的 `go vet ./...` 直接扫描整个仓库。
- 新发现的问题先登记到 `docs/ISSUES_AND_REMEDIATION_PLAN.md`，再决定是否进入实现排期。
