# ADR-0009: 依赖注入用编译期代码生成 `google/wire`（生成物入库，接受工具已归档）

> 状态：**已废弃（由 ADR-0010 取代）** · 产出：flow-init（2026-10-14 提出并接受，**同日推翻**）· 关联：docs/deliverables/x-gulu/02-后端设计.md §3

## 废弃说明（2026-10-14）

本决策**未落地任何代码**（项目当时无 `.go` 文件、无 `require`），当日即被 **ADR-0010** 取代。推翻的两条理由：

1. **`google/wire` 已归档（2025-08-25）**——本 ADR 当时知情并选择"接受风险继续用"。用户知情后判断"不必用一个已停止维护的工具"，这是对同一事实的**另一种合理取舍**：本 ADR 论证的是"风险可接受"，而非"收益足够大到值得承担风险"。
2. **wire 对泛型支持不佳**（本 ADR 当时未评估）——wire 的分析器不做类型参数推导，泛型 provider（`func NewRepo[T any]() *Repo[T]`）会被**静默忽略**或报 `cannot infer type parameter`，需要为每个具体类型手写非泛型包装函数。对 Golang 项目这是实打实的长期摩擦，而归档意味着**它永远不会被修**。

下文原样保留，作为"为什么当时认为可以用"的推理记录——其中「后果 · 可逆性与退路」一节描述的退路（退回手写装配，本规模约 40 行），正是 ADR-0010 现在采用的方案。

## 背景

后端需要把约十个长生命周期对象装配起来：`config.Config` → `*store.Store` → `provider.Registry` → `context.Builder` → `*chat.Service` → `*httpapi.Server` → `*app.App`，并携带 `*slog.Logger` 与若干需要 `func()` 清理的资源（数据库连接）。

约束：

1. **依赖方向必须单向**（`store`/`provider` 不得 import `chat`/`httpapi`），装配是唯一"知道全图"的地方——它要么集中在一处，要么就会渗透到各包的构造函数里。
2. **测试要能反向自由构造**（`chat.New(fakeStore, fakeAdapter, logger)`），因此业务包不能自带 `NewDefault()` 之类"自己找依赖"的捷径。
3. **依赖克制**：项目取向是不引入外部库，但**构建期工具**与**运行时依赖**的风险量级不同——前者不进入二进制、不进入用户的依赖树。

不决策的代价：装配顺序散落在 `main.go` 里手写，新增一个依赖要手工重排顺序（漏传一处就运行期 panic 或静默用错实例），且"谁依赖谁"只存在于开发者脑中。

## 决策

用 **`github.com/google/wire` v0.7.0**（编译期代码生成 DI）：

- 每个包就近导出 `var ProviderSet = wire.NewSet(...)`；`internal/app/wire.go` 只保留一个 `//go:build wireinject` 的 `InitializeApp(cfg config.Config) (*App, func(), error)`，图始终小到一眼能看完。
- 工具用 **Go 1.24+ 的 `tool` 指令**管理（`go get -tool github.com/google/wire/cmd/wire@v0.7.0`，`go tool wire ./internal/app`），不污染 `require` 列表。
- **生成的 `wire_gen.go` 提交进仓库**；CI 重新生成后要求 `git diff --exit-code`（防止改了 ProviderSet 忘记重新生成）。
- **测试不使用 wire**，直接手工构造依赖。

配套五条铁律（写入 `02-后端设计.md` §3，作为评审检查项）：

1. **只注入长生命周期对象**；请求级对象（SSE emitter、`*http.Request`、请求级 logger）**绝不进图**，走函数参数。
2. **原始类型不进图**：`--addr` 与 `--data-dir` 都是 `string`，wire 无法消歧；统一以 `config.Config` 结构体作为唯一原始值来源，确需细粒度时用命名类型（`type DataDir string`）。
3. **清理函数走 provider 返回值**：`func NewStore(cfg config.Config) (*Store, func(), error)`，由 wire 汇总到 `InitializeApp` 的第二个返回值。
4. **生成物入库 + CI 校验漂移**（见上）。
5. **测试路径独立于装配图**。

**知情风险**：`google/wire` 已于 **2025-08-25 归档**（最后版本 v0.7.0，不再维护）。本项目仍然采用，理由见下。

## 权衡

- **手写装配**（`main.go` 里按顺序 new 出来，约 40 行）：本需求规模下**完全可行**，放弃的代价也最小；未选它的原因是收益侧——新增依赖时改 `ProviderSet` 一行 vs 手工重排 `main.go` 的顺序，以及"依赖关系被显式声明并可生成依赖图"对**开源作品**这一交付目标（动机 C：代码结构要拿得出手）有价值。
- **运行期 DI 框架（`uber/dig` / `uber/fx`）**：能自动接线，放弃——基于反射，装配错误推迟到启动期才暴露，且 fx 引入整个生命周期框架，与"简单直白、能被读完的代码"直接冲突。
- **社区 fork（如 `avianlabs/wire`、`goforj/wire`）**：仍在维护，放弃——无 Google 背书，供应链风险高于它解决的问题；本项目对 wire 的需求是"codegen 能跑"，不是"持续演进"。
- **因归档而弃用 wire**：不采用。**wire 是编译期代码生成器，不是运行时依赖**——`wire_gen.go` 一旦生成就是本仓库的普通 Go 代码，工具本身可以随时删除而工程不受影响。归档带来的真实风险只有一个：**未来 Go 版本下 codegen 跑不起来**。而这一风险有明确退路（见「后果」）。

## 后果

- **正面**：装配错误在编译期暴露（类型不匹配直接编不过，而不是运行期 panic）；`main.go` 极薄且只表达启动顺序（旗标 → logger → store → 迁移 → 自愈 → 装配 → Listen）；依赖图可导出审阅；新增依赖的改动面收敛到 `ProviderSet`。
- **负面 / 代价**：多一个 codegen 步骤（新人需知道改完 ProviderSet 要 `go tool wire ./internal/app`，CI 检查用于兜底）；团队成员需理解"什么该进图、什么不该"（铁律已明写）；wire 不再适配新 Go 版本。
- **可逆性与退路**：若某天 wire 在新 Go 版本下无法运行，`wire_gen.go` 已是可读可维护的普通代码，可**冻结生成物并手工维护**，或退回手写装配（本规模约 40 行）——成本可控，不存在"锁死"。
- **后续跟进**：CI 加"重新生成后 `git diff` 为空"检查；每次升级 Go 版本时验证 `go tool wire` 仍能运行；若项目依赖图显著增长（阶段 3 助手门户）再评估是否值得继续维护 codegen 步骤。
