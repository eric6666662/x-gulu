/**
 * 聊天域的可变状态（ADR-0002）。任务 1 只有会话列表；
 * messages / stream 等随任务 3 起逐步进这个 store，组件不直接碰 API。
 */

import { defineStore } from 'pinia'
import { ref } from 'vue'

import { messageOf } from '@/api/client'
import { listConversations } from '@/api/endpoints'
import type { Conversation } from '@/types'

export const useChatStore = defineStore('chat', () => {
  const conversations = ref<Conversation[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  /** 拉会话列表。失败不抛给调用方：错误留在 store 里由侧栏渲染错误态。 */
  async function load(): Promise<void> {
    loading.value = true
    error.value = null
    try {
      conversations.value = await listConversations()
    } catch (err) {
      error.value = messageOf(err)
    } finally {
      loading.value = false
    }
  }

  return { conversations, loading, error, load }
})
