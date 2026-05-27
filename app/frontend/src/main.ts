import { createApp } from 'vue'
import { createPinia } from 'pinia'

import App from './App.vue'
import './style.css'
import { vPress } from './directives/press'

const app = createApp(App)

app.use(createPinia())
app.directive('press', vPress)
app.mount('#app')
