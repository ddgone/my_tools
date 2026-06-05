# 前端工作台

当前前端是火蜥蜴工具箱的单页工作台，不是 Vite 模板示例。

## 技术栈

- Vue 3
- TypeScript
- Pinia
- Naive UI
- GSAP

## 关键目录

- `src/App.vue`：前端根组件
- `src/components/`：工作台组件
- `src/stores/workspace.ts`：工作台状态管理
- `src/styles.css`：全局样式入口

## 常用命令

```bash
npm install
npm run dev
npm run lint
npm run typecheck
npm run build
```

## 约束

- 当前前端是单页工作台，不使用 Vue Router。
- SSH、收藏夹、最近使用、产物中心、任务快照页、执行终端都挂在同一工作区模型下。
- 若文档与实现冲突，以 `src/components/` 和 `src/stores/workspace.ts` 的当前代码为准。
