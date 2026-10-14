import { fireEvent, render, screen, waitFor } from '@testing-library/vue'
import { createPinia } from 'pinia'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import ChatShell from './ChatShell.vue'

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

async function renderShell() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<div>首页</div>' } },
      { path: '/c/:id', component: { template: '<div>会话</div>' } },
    ],
  })
  await router.push('/')
  await router.isReady()

  const utils = render(ChatShell, { global: { plugins: [createPinia(), router] } })
  return { ...utils, router }
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('ChatShell', () => {
  it('挂载即拉会话列表，空列表时侧栏显示空态', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse([])))

    await renderShell()

    await waitFor(() => {
      expect(screen.getByText('还没有会话，点上方「新会话」开始')).toBeTruthy()
    })
  })

  it('「新会话」按钮在位但禁用——建会话接口属于任务 7', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse([])))

    await renderShell()

    const button = screen.getByRole('button', { name: '新会话' }) as HTMLButtonElement
    expect(button.disabled).toBe(true)
    expect(button.getAttribute('title')).toContain('任务 7')
  })

  it('点侧栏会话跳到 /c/:id', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        jsonResponse([
          {
            id: 'c2',
            title: '今天聊点什么',
            serviceId: null,
            modelId: null,
            roleId: null,
            messageCount: 4,
            createdAt: 1_760_000_000,
            updatedAt: 1_760_000_200,
          },
        ]),
      ),
    )

    const { router } = await renderShell()

    const link = await screen.findByRole('link', { name: /今天聊点什么/ })
    await fireEvent.click(link)

    await waitFor(() => {
      expect(router.currentRoute.value.path).toBe('/c/c2')
    })
  })
})
