# 0025-unified-compiled-tool-integration.md

## 决策：统一编译型工具接入模型——纯库包 + 模板生成入口

### 状态

已采纳（2026-06）

### 背景

当前仓库存在三套工具接入路径：

- **Go 工具**：实现 `framework.Tool` 接口 + `init()` 注册 + `app/legacy.go` 匿名导入，本地执行走宿主内闭包直调，远程执行走 wrapper 编译，两条路径分叉。
- **Rust 工具**：binary crate (`src/main.rs`)，builder 直接在 crate 目录 `cargo build`。
- **Python 工具**：脚本 + adapter + `framework.Tool` + `init()` 注册。

主要摩擦点：

1. `framework.Tool` 的元数据（ID/Name/Category）与 manifest 元数据是两套真相源，必须人工保持一致。
2. 新增 Go/Rust 工具需要改宿主代码（`app/legacy.go` 匿名导入），违背工具独立性。
3. Go 工具本地执行（闭包直调）与远程执行（wrapper 编译）行为不一致。
4. 工具源码对 `libs/framework` 包有强依赖。

### 决策

**所有编译型工具（Go / Rust）都是纯库包，入口 `main` 全部由 builder 用共享模板生成，工具源码里不存在任何入口胶水代码。**

具体含义：

1. **Go 工具**：纯库包（`package <tool_id>`），导出约定函数 `func Run(ctx context.Context, args []string, out io.Writer) error`。不含 `func main()`，不调用 `framework.Register()`，不导入 `libs/framework`。
2. **Rust 工具**：纯 library crate（`src/lib.rs`），导出约定函数 `pub fn run(args: &[String])`。不含 `src/main.rs`。
3. **Builder** 构建时为每个工具生成临时 wrapper（Go: `main.go`，Rust: wrapper crate），wrapper 调用工具的约定函数。
4. **manifest 是唯一元数据来源**，不再有 `framework.Tool` 双轨。
5. **本地执行统一为子进程执行**，Go 和 Rust 共用 `executeLocalBinaryTool`。

脚本型工具（Python）后续可按相同思路泛化为"脚本轨道"，当前保持独立实现。

### 替代方案

| 方案 | 优点 | 缺点 |
|------|------|------|
| 保持 `framework.Tool` 接口 | 现有工具不需要改动 | 元数据双轨、入口胶水代码、框架依赖 |
| wrapper 模板由工具自己提供 | 灵活性更高 | 每个工具多一个模板文件，打破"纯库包"的简洁约定 |
| **builder 统一生成 wrapper（采纳）** | 工具源码极简、约定清晰、一条模板覆盖所有同语言工具 | wrapper 模板变更时需要更新 cache key |

### 影响

- **工具源码**：不再依赖任何框架包，真正独立。
- **接入流程**：新增工具只需写业务代码 + manifest，不需要改宿主代码。
- **本地执行**：Go 工具不再有"零 Go SDK 依赖"的优势（交付安装包包含产物，用户侧不受影响）。
- **cache key**：wrapper 模板内容纳入 hash，模板变更自动触发缓存失效。
- **桥接层退役**：`libs/framework/` 目录完整删除，`app/legacy.go` 大幅简化。

### 相关文档

- [统一工具接入设计](../UNIFIED_TOOL_INTEGRATION_DESIGN.md)
- [Go 工具接入标准](../GO_NATIVE_TOOL_INTEGRATION_STANDARD.md)
- [Rust 工具接入标准](../RUST_TOOL_INTEGRATION_STANDARD.md)
