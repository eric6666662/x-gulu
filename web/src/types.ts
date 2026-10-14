/**
 * 接口契约类型 —— 与后端 DTO 一一对应（`internal/httpapi/handlers.go`）。
 * 后端字段是 camelCase，这里不加转换层，谁改字段两边一起改。
 */

/** 会话列表项，GET /api/conversations 的元素。 */
export interface Conversation {
  id: string
  title: string
  serviceId: string | null
  modelId: string | null
  roleId: string | null
  messageCount: number
  /** Unix 秒 */
  createdAt: number
  /** Unix 秒 */
  updatedAt: number
}

/** GET /api/health */
export interface Health {
  ok: boolean
  version: string
}
