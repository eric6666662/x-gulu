<script setup lang="ts">
/** 会话视图占位：消息列表与输入框归任务 3。 */
import { useRoute } from 'vue-router'

const route = useRoute()
</script>

<template>
  <div class="view-empty">
    <h1 class="view-empty__title">这个会话还没有消息</h1>
    <p class="view-empty__text">会话 {{ route.params.id }} 已在地址栏中，刷新可回到这里。</p>
  </div>
</template>
