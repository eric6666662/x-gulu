/**
 * 所有网络细节的唯一出口：组件与 store 不直接碰 fetch。
 * 接口边界见 docs/deliverables/chat-mvp/02-前端设计.md §2「api/client」。
 * 后续 SSE 流（任务 3）也加在这里，上层只换一个入口函数。
 */

export interface ApiErrorInit {
  status: number
  code: string
  message: string
  field?: string
}

/**
 * 归一化后的接口错误。调用方只看 code / message / field，
 * 不必再关心 HTTP 状态码或响应体形状。
 */
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly field?: string

  constructor(init: ApiErrorInit) {
    super(init.message)
    this.name = 'ApiError'
    this.status = init.status
    this.code = init.code
    this.field = init.field
  }
}

/** 后端产出的中文 message 直接展示，这里只兜底网络层失败与无法解析的响应。 */
const NETWORK_MESSAGE = '连不上服务，请确认后端已启动'
const FALLBACK_MESSAGE = '未知错误'

/** status 为 0 表示请求根本没发出去（后端没启动 / 断网）。 */
export async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers = new Headers({ Accept: 'application/json' })
  const init: RequestInit = { method, headers }
  if (body !== undefined) {
    headers.set('Content-Type', 'application/json')
    init.body = JSON.stringify(body)
  }

  let response: Response
  try {
    response = await fetch(path, init)
  } catch {
    throw new ApiError({ status: 0, code: 'network_error', message: NETWORK_MESSAGE })
  }

  if (!response.ok) throw await toApiError(response)

  return (await response.json()) as T
}

/** 把任意异常翻成能直接显示给用户的一句话。 */
export function messageOf(err: unknown): string {
  if (err instanceof Error && err.message) return err.message
  return FALLBACK_MESSAGE
}

interface ErrorBody {
  error?: { code?: unknown; message?: unknown; field?: unknown }
}

async function toApiError(response: Response): Promise<ApiError> {
  const detail = await readErrorDetail(response)
  return new ApiError({
    status: response.status,
    code: typeof detail?.code === 'string' ? detail.code : `http_${response.status}`,
    message:
      typeof detail?.message === 'string' ? detail.message : `请求失败（HTTP ${response.status}）`,
    field: typeof detail?.field === 'string' ? detail.field : undefined,
  })
}

async function readErrorDetail(response: Response): Promise<ErrorBody['error']> {
  try {
    const body = (await response.json()) as ErrorBody
    return body.error
  } catch {
    // 非 JSON 响应（如前端资源未构建时返回的纯文本 503）：退回通用文案
    return undefined
  }
}
