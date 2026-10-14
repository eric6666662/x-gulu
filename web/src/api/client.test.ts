import { afterEach, describe, expect, it, vi } from 'vitest'

import { ApiError, messageOf, request } from './client'

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('request', () => {
  it('200 时返回解析后的 JSON', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { ok: true, version: 'dev' }))
    vi.stubGlobal('fetch', fetchMock)

    await expect(request<{ ok: boolean; version: string }>('GET', '/api/health')).resolves.toEqual({
      ok: true,
      version: 'dev',
    })
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/health')
    expect(init.method).toBe('GET')
  })

  it('带 body 时发 JSON 并声明 Content-Type', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { id: 'c1' }))
    vi.stubGlobal('fetch', fetchMock)

    await request('POST', '/api/conversations', { title: '写周报' })

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(init.method).toBe('POST')
    expect(init.body).toBe(JSON.stringify({ title: '写周报' }))
    expect(new Headers(init.headers).get('Content-Type')).toBe('application/json')
  })

  it('把后端错误体翻成带 status/code/field 的 ApiError', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        jsonResponse(400, {
          error: { code: 'invalid_request', message: '标题不能为空', field: 'title' },
        }),
      ),
    )

    const err = await request('GET', '/api/conversations').catch((e: unknown) => e)

    expect(err).toBeInstanceOf(ApiError)
    const apiErr = err as ApiError
    expect(apiErr.status).toBe(400)
    expect(apiErr.code).toBe('invalid_request')
    expect(apiErr.message).toBe('标题不能为空')
    expect(apiErr.field).toBe('title')
  })

  it('非 JSON 的失败响应也给得出可读错误', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('前端资源未构建', { status: 503 })))

    const err = (await request('GET', '/').catch((e: unknown) => e)) as ApiError

    expect(err).toBeInstanceOf(ApiError)
    expect(err.status).toBe(503)
    expect(err.code).toBe('http_503')
    expect(err.message).toContain('503')
  })

  it('网络不通时归为 network_error 而不是抛原生异常', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))

    const err = (await request('GET', '/api/health').catch((e: unknown) => e)) as ApiError

    expect(err).toBeInstanceOf(ApiError)
    expect(err.code).toBe('network_error')
  })
})

describe('messageOf', () => {
  it('取 ApiError 与普通 Error 的 message', () => {
    expect(messageOf(new ApiError({ status: 500, code: 'internal_error', message: '服务出错' }))).toBe(
      '服务出错',
    )
    expect(messageOf(new Error('boom'))).toBe('boom')
  })

  it('认不出的东西给兜底文案，不吐 [object Object]', () => {
    expect(messageOf({ weird: true })).toBe('未知错误')
    expect(messageOf(undefined)).toBe('未知错误')
  })
})
