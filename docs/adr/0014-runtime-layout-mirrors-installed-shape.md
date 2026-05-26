# ADR 0014: 开发态运行时目录镜像安装态目录形态

## 日期

2026-05-26

## 状态

已采纳

## 背景

项目早期没有统一的运行时目录概念：构建产物散落在 `build/` 和 `app/build/bin/`，SSH 配置写在 `~/.fire-salamander/`，Python 脚本释放到系统 `TempDir`，临时构建目录用完后即删。同时用户表达了"开发阶段用项目内本地目录快速迭代，稳定后再打安装包发布"的诉求。

## 决策

**采用安装镜像目录与运行时目录分离的布局**，开发态在项目内完整模拟安装后的目录形态。

### 运行时目录（可变数据）

存放缓存、脚本副本、日志、导出与连接配置：

```
build/runtime/
├── cache/
│   ├── builds/          # 单工具编译缓存
│   └── scripts/         # Python 脚本副本
├── config/
│   ├── ssh_connections.json  # SSH 连接配置
│   └── app.json              # 应用首选项
├── logs/
└── exports/
```

- 开发态自动使用 `build/runtime/`
- 安装态回退到 `~/.fire-salamander/`
- `build/` 整个目录已在 `.gitignore` 中
- 构建脚本 `scripts/build.go` 在 Wails 编译后自动初始化上述子目录并写入默认 `app.json`

### 安装镜像目录（只读资产）

只包含随安装包分发的只读资产：

```
build/image/
└── host/               # Wails 桌面宿主构建产物
```

- 构建脚本输出到 `build/image/host/`
- 打包脚本 `scripts/package/main.go` 只消费 `build/image/`，不进缓存/日志/配置
- 将来可扩展 `build/image/toolchains/` 等子目录放置 Go 工具链等平台资产

### SSH 配置独立子目录

SSH 连接配置从运行时目录根级移到 `config/` 子目录，与应用首选项 `app.json` 共存，统一维护。

### 密码编码规范

SSH 配置文件的 JSON 编码使用 `SetEscapeHTML(false)`，避免 `&`、`<`、`>` 等字符被转义为 `\u0026` 等形式，保持人工可读性。

## 影响

- **正面**：开发态与发布态目录同构，开发期即可验证安装后的行为
- **正面**：安装包只含只读资产，不会误打包缓存、日志和用户配置
- **正面**：`build/` 既是构建产物落点也是开发态运行时数据根，查找直观
- **代价**：`scripts/build.go` 复杂度有所增加，需在编译后初始化目录和配置

## 关联

- [ADR 0015: 安装镜像目录与运行时数据分离](./0015-separate-install-image-from-runtime-data.md)
- [ADR 0016: SSH 连接管理与工具享有同等 Tab 页体验](./0016-ssh-connection-management-tab-parity.md)
- [ADR 0003: 本地统一调度 SSH 透传执行](./0003-local-orchestrated-ssh-execution.md)
