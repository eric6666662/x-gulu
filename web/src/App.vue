<script setup lang="ts">
/** 根组件只管一件事：把路由出口放进常驻的页面骨架里。 */
import ChatShell from '@/components/ChatShell.vue'
</script>

<template>
  <ChatShell>
    <RouterView />
  </ChatShell>
</template>
