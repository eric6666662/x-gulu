# ADR-0003: 流式通道采用 POST + SSE over fetch（提议，待后端确认）

> 状态：**已接受**（2026-10-12 由 flow-design-backend 确认，协议细节见 02-后端设计.md §1.3）· 产出：flow-design-frontend（2026-10-12）· 关联：docs/deliverables/x-gulu/02-前端设计.md §5

## 背景

需求功能 #2 要求逐 token 流式渲染且首 token 200ms 内到界面；功能 #3 要求可中断；请求体含会话 id、消息、模型引用等结构化字段。需要在浏览器可用的三种实时通道里选一个，且前后端必须一致——不决策的代价是前端按 A 写、后端按 B 实现。

## 决策

**HTTP POST 请求 + `text/event-stream`（SSE）响应**，前端用 `fetch` 读 `ReadableStream`（原生 `EventSource` 不支持 POST 故不用），`AbortController` 承担"停止生成"。事件协议见 02-前端设计 §5（`delta` / `done` / `error` / `interrupted`）。

## 权衡

- **WebSocket**：双向能力用不上（停止用 abort 即可），放弃——需要独立升级握手、心跳与重连协议，Go/浏览器两侧复杂度都更高；代理兼容性更差。
- **普通 POST + 整包响应**：实现最简，放弃——无法满足逐 token 流式（需求 #2）。
- **纯 `EventSource` SSE**：浏览器原生自动重连，放弃——不能带 POST body（无法传会话上下文与模型引用），补丁式改造不值得。
- **短轮询**：放弃——延迟与服务端开销都不满足 200ms 指标。

## 后果

- **正面**：单请求单响应，生命周期简单；中断 = abort，与需求边界 #3 天然对齐；Go 侧 `net/http` 直接写 `Flusher` 即可。
- **负面 / 代价**：SSE 只适用于单向流；代理/中间层若缓冲需确认（本项目默认 127.0.0.1 直连，风险低）；前端要自己处理跨 chunk 增量解码（已封装进 `api/client`）。
- **后续跟进**：`flow-design-backend` 确认协议与事件表后，将本 ADR 状态改为"已接受"；若后端选择其他通道，作废本 ADR 并更新 02-前端设计 §5。
