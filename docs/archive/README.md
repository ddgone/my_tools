# 文档归档说明

`docs/archive/` 用于存放对当前开发仍有历史参考价值、但已经不能再作为现状依据的文档。

## 归档规则

- 当前有效文档放在仓库根 `README.md`、`CONTEXT.md` 与 `docs/` 根目录。
- 历史计划、旧版开发说明、阶段性复盘、模板 README 一律进入归档目录。
- 归档文档允许保留旧结论，但不能再被 README 作为主导航直接引用。

## 本次归档文件

- `2026-05-28-readme-before-doc-refresh.md`
  - 旧入口 README，包含过时的文档位置和 ADR 数量描述。
- `2026-05-28-architecture-before-doc-refresh.md`
  - 旧架构说明，混入 Vue Router、旧样板数量和过期能力结论。
- `2026-05-28-development-setup-before-doc-refresh.md`
  - 旧开发说明，包含 `app/desktop` 等失效路径和历史脚本名。
- `2026-05-28-project-overview-before-doc-refresh.md`
  - 旧项目总览，混入旧页面结构和阶段性结论。
- `2026-05-28-desktop-rebuild-plan-historical.md`
  - 早期桌面化重构计划稿，保留历史参考，不再视为当前方案。
- `2026-05-28-frontend-polish-plan-historical.md`
  - 前端打磨阶段计划，多数事项已落地，转为历史记录。
- `2026-05-28-dev-ctrl-c-retrospective-tui.md`
  - 旧 TUI 专项复盘，与当前 Wails 主线已脱节。
- `2026-05-28-frontend-readme-vite-template.md`
  - Vite 模板默认 README，对项目本身没有持续指导价值。

## 使用约定

- 需要了解“为什么曾经这么设计”时才查归档。
- 需要判断“现在怎么开发”时，只看当前文档，不看归档。
