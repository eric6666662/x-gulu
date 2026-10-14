# ADR-0010: 依赖装配用手写显式构造（弃用 wire，不引任何 DI 工具）

> 状态：已接受 · 产出：flow-init 补记（2026-10-14）· 取代：ADR-0009（已废弃）· 关联：docs/deliverables/x-gulu/02-后端设计.md §3

## 背景

后端需把约十个长生命周期对象装配起来（`config.Config` → `*store.Store` → `provider.Registry` → `context.Builder` → `*chat.Service` → `*httpapi.Server` → `*app.App`），并管理其释放顺序。

ADR-0009 曾决定用 `google/wire` 的编译期代码生成来做这件事。该决策当日被推翻，原因见 ADR-0009「废弃说明」：**wire 已于 2025-08-25 归档不再维护，且其分析器不支持泛型 provider**（泛型 provider 被静默忽略或报 `cannot infer type parameter`，需逐类型手写非泛型包装）——归档意味着后者永远不会被修复。

于是问题回到原点：这十来个对象，怎么接起来。

## 决策

**不引任何 DI 工具或框架，装配就是 `internal/app` 里一段按依赖顺序排列的普通 Go 代码。**

```go
// internal/app/app.go（示意）
func New(cfg config.Config) (*App, error) {
    logger := logging.New(cfg)
    st, err := store.Open(cfg)                    // 打开 + WAL/pragma
    if err != nil { return nil, fmt.Errorf("打开数据库: %w", err) }
    reg := provider.NewRegistry(cfg)
    chatSvc := chat.New(st, reg, context.NewBuilder(reg), logger)
    srv := &http.Server{
        Addr: cfg.Addr, Handler: httpapi.New(chatSvc, st, cfg, logger).Handler(),
        ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 120 * time.Second,
    }
    return &App{cfg: cfg, logger: logger, store: st, server: srv}, nil
}

func (a *App) Run(ctx context.Context) error   // ListenAndServe → 等 ctx.Done → Shutdown(10s)
func (a *App) Close() error                    // 逆序释放：先 server 后 store
```

配套约定（评审检查项）：

1. **装配集中在 `internal/app` 一处**；`cmd/xgulu/main.go` 只剩"解析旗标 → 跑迁移 → 启动自愈 → `app.New` → `Run` / `Close`"。业务包**不提供** `NewDefault()`、包级单例之类的"自己找依赖"入口——依赖一律由参数传入。
2. **只注入长生命周期对象**；请求级对象（SSE emitter、`*http.Request`、请求级 logger）**绝不进构造函数**，走函数参数。
3. **接口定义在使用方（依赖倒置）**：`chat` 包自己声明 `type Store interface{...}` / `type Adapter interface{...}`，由 `store` / `provider` 提供实现。这既是"测试不用 DI 工具"的替代方案，也是依赖方向单向的保证。
4. **构造函数只接收真正需要的东西**，能失败才返回 error（开 DB、读配置）；纯内存组装不返回 error。
5. **释放顺序与构造相反**：`App.Close()` 逆序关闭（先 server 后 store），或由 `main` 的 `defer` 栈自然满足。

## 权衡

- **`google/wire`（ADR-0009 原方案）**：编译期校验依赖图、装配错误在编译期暴露，放弃——理由见上；且泛型 provider 需逐类型手写包装，对本项目是长期摩擦。
- **`uber/dig` / `uber/fx`**：运行期反射装配，放弃——装配错误推迟到启动期才暴露；fx 还引入整个生命周期框架与"什么代码该放哪里"的新规矩，与"简单直白、能被读完的代码"冲突。
- **社区 wire fork（如 `avianlabs/wire`、`goforj/wire`）**：仍在维护，放弃——无背书，为解决问题 A（装配）而引入问题 B（供应链风险），不划算。
- **保留 wire 但在泛型处绕过**（手写非泛型包装函数）：放弃——为迁就一个已归档工具而扭曲业务代码的类型设计，是本末倒置。

**为什么手写在这里够用**（本 ADR 的核心判断）：

- **规模小**：依赖图约十个节点、一次构造、无生命周期作用域（无 `request`/`session` 级作用域，无按需构造），这是 DI 工具真正发光的地方——本项目一个都没有。
- **编译期保障并未丢失多少**：Go 的构造函数签名本身就是类型检查，漏传参数**编译不过**，不是运行期 panic。wire 额外提供的只是"顺序与接线的自动推导"，而在十行代码里顺序是肉眼可查的。
- **零工具链**：没有 codegen 步骤、没有"改了 ProviderSet 忘记重新生成"的漂移风险、没有新人需要先学会的构建前步骤。

## 后果

- **正面**：零依赖、零 codegen；`main.go` 与 `internal/app` 都是任何 Go 开发者不用学新概念就能读懂的代码；装配顺序与副作用顺序写在同一个函数里，调试"为什么某个依赖不是预期实例"时只需看一个文件。
- **负面 / 代价**：新增一个依赖时要改 `internal/app` 十几行（接受）；依赖图变大后没有自动校验帮忙，只能靠评审与 `depguard` 保证方向正确——**这是本决策的主要成本，且它随模块数量增长而增长**。
- **可逆性**：若阶段 3 助手门户使依赖图显著膨胀（出现多套 adapter 组合、按场景切换的实现、生命周期作用域），届时重新评估 DI 方案的成本仍然很低——装配逻辑已隔离在 `internal/app` 一个包里，替换它不触碰任何业务包。
- **后续跟进**：CI 用 golangci-lint 的 `depguard` 守住依赖方向（`store`/`provider` 不得 import `chat`/`httpapi`），弥补"没有工具帮你看图"的缺口；评审时把 `internal/app` 的 diff 视为高风险区（它一错就是全站性的）。
