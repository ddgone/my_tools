# GSAP 动画架构与分批策略

我们决定使用 GSAP（GreenSock Animation Platform）作为桌面应用交互动画的统一方案，通过 `npm install gsap` 直接引入，不通过额外的 Vue 封装层。动画风格采用混合策略：常规交互（按钮 hover、侧边栏宽度过渡、tab 切换）使用克制、快速的微动效（duration ≤ 300ms）；特殊场景（任务执行完成、导出成功）可使用霓虹发光、缩放弹跳等炫酷效果。

动画实施按三个批次推进（已全部完成）：
- 第一批：主窗口揭示动画（opacity + scale） + 页面切换过渡（页签切换时的淡入/滑入动效）
- 第二批：侧边栏展开折叠弹性动画 + 全局按钮 hover/press 微交互
- 第三批：文件树节点展开收起 + 执行终端日志滚动平滑 + Modal 弹入弹出

GSAP 动画通过 Vue composables 封装为可复用单元，所有动画时长和缓动函数通过全局配置常量 `src/utils/animation.ts` 统一控制，便于后续调整或关闭。

## Naive UI 动画冲突解决

为避免 Naive UI 组件内置动画与 GSAP 产生视觉冲突，采取以下策略：

1. **全局禁用 Naive UI 动画**：`NConfigProvider` 设置 `:transition-disabled="true"`
2. **移除 CSS transform transition**：`.ui-interactive` 和 `.ui-surface-hover` 的 CSS transition 中移除 `transform` 属性，改为由 GSAP 控制缩放
3. **Vue Transition 替换**：侧边栏的 `Transition name="slide"` 改为 GSAP JavaScript 钩子（`@enter` / `@leave`），通过 `gsap.fromTo` 控制 `x` 位移和 `opacity`
4. **按钮按压反馈**：通过 `v-press` 指令实现 pointerdown 缩放 0.96、pointerup 回弹至 1.0 的微交互
5. **NTree 展开动画**：`v-model:expanded-keys` 替换为手动 `:expanded-keys` + `@update:expanded-keys` 绑定，在展开事件中通过 GSAP stagger 动画子节点（`opacity: 0, y: -6` → `opacity: 1, y: 0`）
6. **终端滚动平滑化**：自动滚动从 `el.scrollTop = el.scrollHeight` 改为 `gsap.to(el, { scrollTop, duration: 0.4s, overwrite: 'auto' })`
7. **Modal 保留 CSS 过渡**：`HotkeyHelpModal`、`SettingsModal`、搜索命令面板的 `Transition name="fade-scale"` 使用自定义 CSS（非 Naive UI 组件），不受 `transition-disabled` 影响，保留不动

## Splash 禁用

前端 Splash 方案（`index.html` 静态覆盖层 + `App.vue` GSAP 淡出）因加载时机与主界面基本同步、实际起到的是"延迟展示"而非"提前遮挡"的效果，已禁用。相关代码已移除，未来可在 Wails v3 原生 Splash 或启动性能优化后重新评估。
