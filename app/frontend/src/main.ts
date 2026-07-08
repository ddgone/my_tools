import { createApp } from 'vue'
import { createPinia } from 'pinia'

import App from './App.vue'
import './style.css'
import { vPress } from './directives/press'

const app = createApp(App)

app.use(createPinia())
app.directive('press', vPress)
app.mount('#app')

// ------------------------------------------------------------------
// WebView2 修复：点击按钮后自动 blur，防止 focus/hover 状态残留
// WebView2 存在 :hover 伪类在点击后卡住的已知 bug，
// 通过 blur + 强制回流来清除残留的交互状态。
// ------------------------------------------------------------------
document.addEventListener(
  'click',
  (e) => {
    const target = e.target as HTMLElement
    // 不干扰输入框内的操作
    if (target.closest('input, textarea, [contenteditable="true"]')) return

    // 1) 找到最近的可聚焦控件并 blur
    const focusable = target.closest(
      'button, [role="button"], [role="checkbox"], [role="switch"], [role="radio"], .n-tree-node',
    ) as HTMLElement | null
    if (focusable) {
      requestAnimationFrame(() => {
        if (document.activeElement === focusable || focusable.contains(document.activeElement)) {
          ;(focusable as HTMLElement).blur()
        }
      })
    }

    // 2) 强制回流清除 WebView2 中卡住的 :hover 伪类
    void document.body.offsetHeight
  },
  true,
)
