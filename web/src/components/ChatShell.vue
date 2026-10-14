<script setup lang="ts">
/**
 * 页面骨架的唯一入口（02-前端设计 §2）：侧栏常驻，主区交给路由。
 * 「引导视图分流」（无服务商时挡在聊天前）归任务 2。
 */
import { storeToRefs } from 'pinia'
import { onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { useChatStore } from '@/stores/chat'

import ConversationList from './ConversationList.vue'

const route = useRoute()
const router = useRouter()
const chat = useChatStore()
const { conversations, loading, error } = storeToRefs(chat)

onMounted(() => {
  void chat.load()
})

function openConversation(id: string): void {
  void router.push(`/c/${id}`)
}
</script>

<template>
  <div class="shell">
    <aside class="shell__sidebar">
      <div class="shell__sidebar-head">
        <!-- 建会话接口（POST /api/conversations）属于任务 7，先占位禁用 -->
        <button
          type="button"
          class="new-conversation"
          disabled
          title="新建会话将在任务 7 开放"
        >
          新会话
        </button>
      </div>

      <ConversationList
        :items="conversations"
        :active-id="(route.params.id as string | undefined) ?? null"
        :loading="loading"
        :error="error"
        @select="openConversation"
        @retry="chat.load()"
      />
    </aside>

    <main class="shell__main">
      <slot />
    </main>
  </div>
</template>

<style scoped>
.shell {
  display: flex;
  height: 100%;
  background: var(--color-canvas);
}

.shell__sidebar {
  display: flex;
  flex: 0 0 var(--layout-sidebar-w);
  flex-direction: column;
  min-height: 0;
  border-inline-end: 1px solid var(--color-border);
  background: var(--color-surface);
}

.shell__sidebar-head {
  padding: var(--space-md);
}

.new-conversation {
  display: block;
  width: 100%;
  padding: var(--space-sm) var(--space-md);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-surface);
  font-size: var(--text-body);
  text-align: center;
  cursor: pointer;
  transition: background var(--ease-standard);
}

.new-conversation:hover:not(:disabled) {
  background: var(--color-surface-hover);
}

.new-conversation:disabled {
  color: var(--color-text-muted);
  cursor: not-allowed;
}

.shell__main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
</style>
