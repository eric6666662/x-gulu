/**
 * 路由表见 docs/deliverables/chat-mvp/02-前端设计.md §4。
 * 无登录 → 无鉴权边界（安全性由后端只监听 127.0.0.1 承担）。
 * 刷新可恢复：会话 id 在 URL 里（`/c/:id`）。
 */

import type { RouteRecordRaw } from 'vue-router'
import { createRouter, createWebHistory } from 'vue-router'

export const routes: RouteRecordRaw[] = [
  { path: '/', name: 'home', component: () => import('@/views/HomeView.vue') },
  { path: '/c/:id', name: 'conversation', component: () => import('@/views/ChatView.vue') },
  { path: '/settings', name: 'settings', component: () => import('@/views/SettingsView.vue') },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

export function createAppRouter() {
  return createRouter({ history: createWebHistory(), routes })
}
