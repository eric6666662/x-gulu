# ADR-0005: 模型服务商接入用自研 Adapter 抽象（不引 LLM 框架）

> 状态：已接受 · 产出：flow-design-backend（2026-10-12）· 关联：docs/deliverables/chat-mvp/02-后端设计.md §3

## 背景

需求要求接入 OpenAI 兼容（DeepSeek / ChatGPT / 豆包 / Qwen / GLM / 自定义）、Anthropic 原生、Ollama 本地三类协议，且要支持流式与"探测可用模型列表"。这些协议只有两处真实差异：请求/响应字段名、流式帧格式。Motivation B 权重已下调，但覆盖面不能缩。

## 决策

**自研最小 Adapter 抽象**，接口只有两个方法：

```go
type Adapter interface {
    Stream(ctx context.Context, req Request, emit func(Delta) error) (Usage, error)
    ListModels(ctx context.Context) ([]Model, error)
}
```

实现三个：`openai`（一家吃下 DeepSeek/ChatGPT/豆包/Qwen/GLM/自定义）、`ollama`（原生 `/api/chat` NDJSON 流）、`anthropic`（原生 `event:` 类型流）。预置服务商表（base url / requiresKey / adapter）由服务端持有。

## 权衡

- **LangChainGo / 各家官方 SDK**：放弃——官方 SDK 只覆盖自家，仍要一层统一抽象；LangChainGo 抽象层厚、升级频繁，对"两个方法"的需求属过度工程，且学习/开源作品的动机（C）更希望依赖面小、代码可读。
- **只做 OpenAI 兼容一家**：实现最省，放弃——Ollama 是"数据自主"用户的常见选择，Anthropic 是主流服务，砍掉与需求的模型可及性目标冲突。
- **每家协议一个 HTTP 手写分支**：可以，放弃——流式解析与错误映射会重复三份，抽成 Adapter 后测试也只需一套假上游模式。
- **在 Request 里预留 tools/multimodal 字段并实现**：放弃——阶段 1 非目标（多模态、工具执行），只留空字段位，避免为假想需求增加测试面。

## 后果

- **正面**：新增一家服务商 = 一行预置配置（若走 OpenAI 兼容）；测试用 `httptest` 假上游即可覆盖全部协议分支。
- **负面 / 代价**：各家高级特性（如 Anthropic 的 thinking、Prompt caching）不暴露，需要时要在 Adapter 内部按协议特判。
- **后续跟进**：阶段 2 的联网搜索若需要工具调用，先在 `openai` adapter 内实现 `tools`，再评估是否提升为跨 adapter 的公共能力（届时可能推翻本 ADR 的"只留空字段"决定）。
