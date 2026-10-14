import { afterEach, describe, expect, it, vi } from 'vitest'

import { getHealth, listConversations } from './endpoints'

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function lastCall(fetchMock: ReturnType<typeof vi.fn>): [string, RequestInit] {
  return fetchMock.mock.calls.at(-1) as [string, RequestInit]
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('getHealth', () => {
  it('打 GET /api/health', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { ok: true, version: 'dev' }))
    vi.stubGlobal('fetch', fetchMock)

    await expect(getHealth()).resolves.toEqual({ ok: true, version: 'dev' })
    expect(lastCall(fetchMock)[0]).toBe('/api/health')
  })
})

describe('listConversations', () => {
  it('打 GET /api/conversations 并原样返回列表', async () => {
    const rows = [
      {
        id: 'c1',
        title: '第一次对话',
        serviceId: null,
        modelId: null,
        roleId: null,
        messageCount: 0,
        createdAt: 1_760_000_000,
        updatedAt: 1_760_000_100,
      },
    ]
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, rows))
    vi.stubGlobal('fetch', fetchMock)

    await expect(listConversations()).resolves.toEqual(rows)
    expect(lastCall(fetchMock)[0]).toBe('/api/conversations')
  })
})
