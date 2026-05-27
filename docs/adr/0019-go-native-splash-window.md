# Go 原生 Splash 窗口显示启动画面 → 前端 Splash 方案

我们决定在 Wails v2 主窗口的 WebView 加载期间，由前端静态 HTML 展示启动画面（Splash）。

实现方案：`index.html` 中放置一个固定覆盖层 `#splash`，使用内联 CSS 居中展示 `/splash.png`（位于 `frontend/public/`，随 Vite 构建打包进 dist，由 `go:embed` 嵌入 exe）。Vue 应用挂载后，`App.vue` 的 `onMounted` 中通过 GSAP 将 `#splash` 淡出（duration 0.35s）并从 DOM 中移除。`WorkspaceLayout.vue` 在启动数据加载完成后播放主界面揭示动画（opacity + scale），形成"静态图片 → 淡出 → 界面揭示"的流畅过渡。

此前尝试的 Win32 原生窗口方案（GDI+ + `golang.org/x/sys/windows`）因 API 兼容性问题被放弃：`x/sys/windows` 包的命名体系与 Win32 宏存在差异，且手写 GDI+ 绑定引入过多不必要复杂度。前端模拟方案零 Go 依赖，跨平台天然可用，维护成本低。

注意：Wails v3 有原生 `Splash` 配置项，升级后可用内置能力替代此前端方案。
