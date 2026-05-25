import { createRouter, createWebHashHistory } from 'vue-router'

import HomeView from '@/views/HomeView.vue'
import ExecuteView from '@/views/ExecuteView.vue'

export const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      path: '/',
      name: 'home',
      component: HomeView,
    },
    {
      path: '/execute/:toolId',
      name: 'execute',
      component: ExecuteView,
      props: true,
    },
  ],
})
