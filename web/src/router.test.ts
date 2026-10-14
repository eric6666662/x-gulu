import { describe, expect, it } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { routes } from './router'

async function at(path: string) {
  const router = createRouter({ history: createMemoryHistory(), routes })
  await router.push(path)
  await router.isReady()
  return router
}

describe('routes', () => {
  it('首页落在聊天壳里', async () => {
    const router = await at('/')

    expect(router.currentRoute.value.name).toBe('home')
  })

  it('/c/:id 把会话 id 带进路由参数', async () => {
    const router = await at('/c/c2')

    expect(router.currentRoute.value.name).toBe('conversation')
    expect(router.currentRoute.value.params.id).toBe('c2')
  })

  it('设置页在 /settings', async () => {
    const router = await at('/settings')

    expect(router.currentRoute.value.name).toBe('settings')
  })

  it('认不出的路径回落到首页', async () => {
    const router = await at('/不存在的页面')

    expect(router.currentRoute.value.path).toBe('/')
  })
})
