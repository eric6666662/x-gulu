<script setup lang="ts">
/**
 * 侧栏会话列表：纯展示，不碰 API（02-前端设计 §2）。
 * 任务 1 只到「标题 + 选中态 + 空/加载/错误态」；
 * 相对时间、`⋯` 菜单（重命名/导出/删除）归任务 7 的 ConversationListItem。
 */
import type { Conversation } from '@/types'

defineProps<{
  items: Conversation[]
  activeId: string | null
  loading: boolean
  error: string | null
}>()

const emit = defineEmits<{
  select: [id: string]
  retry: []
}>()
</script>

<template>
  <nav class="conversations" aria-label="会话列表">
    <p v-if="loading" class="conversations__hint">正在加载会话…</p>

    <div v-else-if="error" class="conversations__error" role="alert">
      <p class="conversations__error-text">{{ error }}</p>
      <button type="button" class="conversations__retry" @click="emit('retry')">重试</button>
    </div>

    <p v-else-if="items.length === 0" class="conversations__hint">
      还没有会话，点上方「新会话」开始
    </p>

    <ul v-else class="conversations__list">
      <li v-for="item in items" :key="item.id">
        <a
          class="conversation"
          :class="{ 'conversation--active': item.id === activeId }"
          :href="`/c/${item.id}`"
          :aria-current="item.id === activeId ? 'page' : undefined"
          @click.prevent="emit('select', item.id)"
        >
          <span class="conversation__title">{{ item.title }}</span>
        </a>
      </li>
    </ul>
  </nav>
</template>

<style scoped>
.conversations {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: var(--space-sm);
  scrollbar-gutter: stable;
}

.conversations__hint {
  margin: var(--space-md) var(--space-sm);
  color: var(--color-text-secondary);
  font-size: var(--text-meta);
}

.conversations__error {
  margin: var(--space-md) var(--space-sm);
}

.conversations__error-text {
  margin: 0 0 var(--space-sm);
  color: var(--color-danger-text);
  font-size: var(--text-meta);
}

.conversations__retry {
  padding: var(--space-xs) var(--space-md);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: var(--color-surface);
  font-size: var(--text-meta);
  cursor: pointer;
  transition: background var(--ease-standard);
}

.conversations__retry:hover {
  background: var(--color-surface-hover);
}

.conversations__list {
  margin: 0;
  padding: 0;
  list-style: none;
}

.conversation {
  display: block;
  position: relative;
  padding: var(--space-sm) var(--space-md);
  border-radius: var(--radius-md);
  color: var(--color-text-primary);
  text-decoration: none;
  transition: background var(--ease-standard);
}

.conversation:hover {
  background: var(--color-surface-hover);
}

/* 选中态：左侧 2px 靛蓝竖条 + 底色，标题不加粗（02-ui设计 §3） */
.conversation--active {
  background: var(--color-surface-active);
}

.conversation--active::before {
  content: '';
  position: absolute;
  inset-block: var(--space-sm);
  inset-inline-start: 0;
  width: 2px;
  border-radius: 1px;
  background: var(--color-primary);
}

.conversation__title {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--text-body);
}
</style>
