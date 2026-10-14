import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { Conversation } from '@/types'

import { useChatStore } from './chat'

const conversation: Conversation = {
  id: 'c1',
  title: '第一次对话',
  serviceId: null,
  modelId: null,
  roleId: null,
  messageCount: 2,
  createdAt: 1_760_000_000,
  updatedAt: 1_760_000_100,
}

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
}

beforeEach(() => {
  setActivePinia(createPinia())
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('useChatStore.load', () => {
  it('加载成功后把会话放进 store 并清掉 loading', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse([conversation])))
    const store = useChatStore()

    await store.load()

    expect(store.conversations).toEqual([conversation])
    expect(store.loading).toBe(false)
    expect(store.error).toBeNull()
  })

  it('空列表就是空列表，不是 null', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse([])))
    const store = useChatStore()

    await store.load()

    expect(store.conversations).toEqual([])
    expect(store.error).toBeNull()
  })

  it('请求期间 loading 为 true', async () => {
    let release: (r: Response) => void = () => {}
    vi.stubGlobal(
      'fetch',
      vi.fn().mockReturnValue(
        new Promise<Response>((resolve) => {
          release = resolve
        }),
      ),
    )
    const store = useChatStore()

    const pending = store.load()
    expect(store.loading).toBe(true)

    release(jsonResponse([]))
    await pending
    expect(store.loading).toBe(false)
  })

  it('失败时记下可读错误、保留空列表，不把异常抛给调用方', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))
    const store = useChatStore()

    await expect(store.load()).resolves.toBeUndefined()

    expect(store.error).toBeTruthy()
    expect(store.error).not.toContain('[object')
    expect(store.conversations).toEqual([])
    expect(store.loading).toBe(false)
  })
})
