# 桌面宿主开发说明

> **注意：本文档中的路径引用已过时。当前项目结构见 [README.md](../README.md)**

本文档只描述当前已经落地的桌面化重构骨架如何启动、构建和继续开发。

## 1. 环境要求

- Go `1.26+`
- Node.js `22+`
- npm `10+`
- Windows 需可用 WebView2 运行时

当前本机已验证：

- `go version` -> `go1.26.3`
- `node -v` -> `v22.21.0`
- `npm -v` -> `10.9.4`
- `wails` -> `v2.12.0`

## 2. 安装 Wails CLI

如果本机未安装：

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

Windows 下如果 `wails` 未自动加入环境变量，可直接调用：

```powershell
& "$env:USERPROFILE\go\bin\wails.exe" version
```

## 3. 仓库结构

- 根目录是平台模块：`my_tools`
- 桌面宿主是子模块：`app/desktop`
- `go.work` 已把两者连成一个工作区

这意味着：

- 平台级包可以放在根模块
- Wails 宿主可以直接引用根模块中的 `core` 和 `catalog`

## 4. 前端依赖安装

在 `app/desktop/frontend` 下：

```bash
npm install
```

当前前端栈：

- Vue 3
- TypeScript
- Pinia
- Vue Router
- Naive UI
- Tailwind CSS

## 5. 启动开发

### 5.1 仅构建前端

```bash
cd app/desktop/frontend
npm run build
```

### 5.2 Go 代码级校验

```bash
cd app/desktop
go test ./...
```

说明：

- 不要在 `app/desktop` 目录直接执行 `go build ./...` 作为桌面程序产物构建方式。
- 这样会在 `app/desktop/` 根目录生成一个看起来像正式产物、实际上缺少 Wails 正确构建标签的 `.exe`。
- 桌面应用的可运行产物只能来自 `wails build`。

### 5.3 Wails 开发模式

```powershell
cd app/desktop
& "$env:USERPROFILE\go\bin\wails.exe" dev
```

### 5.4 Wails 生产构建

```powershell
cd app/desktop
& "$env:USERPROFILE\go\bin\wails.exe" build -clean
```

也可以直接在仓库根目录运行：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build_desktop.ps1
```

产物默认输出到：

```text
build/image/host/fire-salamander-desktop.exe
```

这是当前应该双击运行的唯一正式桌面产物。

## 6. 当前已验证通过的命令

以下命令已在当前仓库执行通过：

```bash
go mod tidy
cd app/desktop && go mod tidy
cd app/desktop/frontend && npm run build
cd app/desktop && go test ./...
cd app/desktop && wails build -clean
```

## 7. 当前开发入口

### 前端入口

- `app/desktop/frontend/src/main.ts`
- `app/desktop/frontend/src/views/HomeView.vue`
- `app/desktop/frontend/src/views/ExecuteView.vue`

### Go 宿主入口

- `app/desktop/main.go`
- `app/desktop/app.go`
- `app/desktop/execution.go`
- `app/desktop/legacy_tools.go`

### 平台模型入口

- `core/toolspec/types.go`
- `catalog/builtin/service.go`
- `catalog/builtin/manifests/*.yaml`

## 8. 继续开发时的推荐顺序

1. 在 `runtime/` 加入 SSH 连接与远程执行链路
2. 在 `builder/` 建立 Go 单工具构建缓存
3. 在 `builder/` 整理 Python 脚本导出策略
4. 在 `catalog/custom/` 实现复制为自定义工具
5. 在执行页加入任务阶段展示、结果文件入口和参数历史
6. 将“导出工具”从按钮占位升级为真实导出流程

## 9. 注意事项

- 不要继续在旧 `internal/tui` 上做新开发
- 不要给旧 `Tool` 接口补新功能
- 不要把 `app/desktop/*.exe` 根目录误产物当成正式桌面程序
- 当前桌面已经进入可用的 Phase 1：主页模式 + 执行页面模式 + 本地执行
- 当前前端 chunk 体积略大，后续可在路由和组件层做拆包优化
