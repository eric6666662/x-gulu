/**
 * 每个接口一个具名函数，路径集中在这里——组件与 store 里不写裸 path。
 * 契约表见 docs/deliverables/x-gulu/02-前端设计.md §5。
 */

import type { Conversation, Health } from '@/types'

import { request } from './client'

export function getHealth(): Promise<Health> {
  return request<Health>('GET', '/api/health')
}

export function listConversations(): Promise<Conversation[]> {
  return request<Conversation[]>('GET', '/api/conversations')
}
