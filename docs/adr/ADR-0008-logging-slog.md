# ADR-0008: 日志用标准库 `log/slog` + `lumberjack` 轮转（请求域 ctx 传递 + 密钥类型化打码）

> 状态：已接受 · 产出：flow-init 补记（2026-10-14 首版定 slog；同日按"需要日志切割"补充轮转方案）· 关联：docs/deliverables/x-gulu/02-后端设计.md §0 §3 §4

## 背景

后端需要一个统一的日志通道，约束来自四处：

1. **依赖克制**：项目对"不引入外部库"有明确偏好（个人自托管工具、单二进制交付、代码要能作为开源作品被人读完）。日志是**每个包都要 import** 的横切关注点，它引一个外部库等于全项目引。
2. **隐私红线**：日志里**绝不能出现** API key 明文、上游响应体、用户消息内容（需求约束「API key 不进日志」）。个人工具会长期在本机运行，日志一旦留痕就难以回收。
3. **日志切割**：需要按大小/时间轮转并清理旧文件，避免长驻运行把磁盘写满；轮转文件还得能被用户找到（报障时要能拿出来）。
4. **无外部收集系统**：没有 ELK / Loki / 云日志。消费者只有两种——运行时的用户（终端直接可见）和事后排查的开发者（需要结构化字段 + 历史留存）。

不决策的代价：各包自行 `fmt.Println`，输出格式不统一、无法按级别过滤、排障时得不到 `req_id` 关联；磁盘上的日志无人清理；且**没有任何机制拦住密钥被顺手打进日志**。

## 决策

**日志库用标准库 `log/slog`；轮转交给 `lumberjack`（`io.Writer` 层面的独立组件）。**

- 库：**标准库 `log/slog`**（Go 1.21+），不引 zap / zerolog / logrus 作为日志库。
- **输出端（sink）**：`io.MultiWriter(os.Stderr, &lumberjack.Logger{...})` —— 同时满足"终端启动时立刻看见"与"历史留存可回溯"。`--log-dir=` 置空即完全不落盘（只留 stderr）。
- **轮转参数**：文件 `<log-dir>/xgulu.log`；`MaxSize=10`(MB) · `MaxAge=30`(天) · `MaxBackups=10` · `Compress=true`（旧文件 gzip）· `LocalTime=true`；`--log-dir` 默认 `<data-dir>/logs`，随 `--data-dir` 一并迁移。
- **切割语义 = 按大小/时间轮转**（已确认）；**不做按级别分流到不同文件**——单文件 + 结构化字段 + `grep`/`jq` 已足够，分流会把"一次请求的完整日志"拆散。
- 权限：日志目录 `0700`、日志文件 `0600`（与数据库同级的本地隐私边界）。
- 格式：**JSON**（生产默认）；`--log-format text` 切 `slog.NewTextHandler` 供人读；`--log-level` 经 `slog.LevelVar` 支持运行期调整；`AddSource` 仅 debug 态开启。
- 传递：**以 `context.Context` 为主通道**——业务代码一律 `slog.InfoContext(ctx, ...)`，中间件把请求域 logger 注入 ctx（`logging.FromContext(ctx)`，取不到回退 `slog.Default()`）；仅当需要固定字段时才显式注入 `logger.With("component", "store")`。
- 请求域字段固定为 `req_id / method / path / status / duration_ms / bytes / err_code`；`req_id` 用 `crypto/rand` 生成并回写 `X-Request-Id` 响应头（零依赖）。
- 级别约定：Info = 生命周期 + 每请求一行访问日志；Warn = 上游 401/429、超时、降级路径（摘要失败、自动加载模型失败）；Error = 5xx、存储失败、panic。
- **密钥用类型强制打码**：`internal/secret.Secret` 实现 `slog.LogValuer` 与 `json.Marshaler`，输出恒为 `sk-****3f2a`。API key 全程以此类型流转，使"顺手写 `slog.Any("key", svc.APIKey)` 导致泄漏"在结构上不可能。**日志落盘后这一层的价值显著上升**——泄漏面从"终端滚屏"变成"磁盘上的文件"。

## 权衡

**日志库选型**（slog 胜出）：

- **`go.uber.org/zap`**：性能与生态最强，放弃——本项目日志量级是每秒个位数，性能完全不是瓶颈，而它带来的外部依赖、`zap.Field` API 学习成本与"全项目都要 import 它"的扩散效应是实打实的。
- **`rs/zerolog`**：零分配、JSON 快，放弃——同上，且其链式 API 与标准库 `context` 的结合不如 slog 自然。
- **`sirupsen/logrus`**：2026-10-14 因"需要切割日志"**重新评估过一次**，仍放弃，两条理由都有决定性：(1) **logrus 本身不提供日志切割**——切割是 `io.Writer` 层的能力，用它照样得引 lumberjack，选它并不能解决切割问题；(2) logrus 自 2020 年起处于 **maintenance mode**（其 README 明言不再加新功能，并推荐 zap/zerolog/slog），与刚被弃用的 wire 是同一种状况。（唯一真实优势是按级别分流的 hooks 生态——本项目已确认不做分流。）
- **自研极简 logger**：能省依赖，放弃——重新发明 `slog` 已有的 `Handler` / `LevelVar` / `LogValuer`，收益为负；且标准库与第三方（`http.Server.ErrorLog` 等）都在向 slog 靠拢，自研会被孤立。
- **显式把 `*slog.Logger` 注入每个组件**：更利于单测断言，放弃作为主通道——会让构造函数多一个参数、漏传时运行期才发现。改为"ctx 主通道 + 需要固定字段时注入"，测试断言用替换默认 logger 的方式。

**轮转组件选型**（lumberjack 胜出）：

- **自研轮转 writer**：放弃——约 100 行且要自己处理重命名竞态、压缩、按时间清理；这正是"能读懂的依赖"优于"自己写的代码"的场合。
- **不落盘、只输出 stderr**（本 ADR 首版的结论）：已推翻——用户明确要求切割能力，且长驻运行下"没有历史日志"对报障是硬伤。
- **`gopkg.in/natefinch/lumberjack.v2`**：采用。**知情风险**：最新版 **v2.2.1**，发布稀疏，社区普遍视其为低维护/休眠状态（未归档，约 5.4k star，被大量项目 vendored）。仍然采用的三条理由：(1) **职责极窄**——它就是一个实现了 `io.WriteCloser` 的 writer，几乎不存在"因 Go 语言演进而失效"的面（这与 wire 的 codegen 依赖工具链、logrus 的框架级 API 都不同）；(2) **无标准库替代品**——Go 至今不提供日志轮转；(3) **替换成本约两行**——`io.MultiWriter` 里换一个 writer 而已，不存在锁定。
- **社区 fork（如 `xtutu/lumberjack`）**：放弃——无背书，且本组件窄到没有 fork 的必要。

**并发说明**：`slog` 的 handler 在写每条记录时内部加锁，因此 `MultiWriter` 双写（stderr + 文件）不会出现记录交错——这是选择"一个 handler + MultiWriter"而非"两个 handler 并行"的原因。

## 后果

- **正面**：零日志库依赖（唯一的额外依赖是写入端的 lumberjack）；日志自动轮转与压缩，不会撑爆磁盘；stderr 与文件双写使开发者与用户各自都能拿到日志；`LogValuer` 让密钥脱敏成为类型系统保证，而不是代码评审时的人肉检查——**在日志落盘之后，这条从"最好有"变成"必须有"**；结构化字段使"按 `req_id` 串起一次流式请求的全部日志"成为可能。
- **负面 / 代价**：多一个外部依赖且其处于低维护状态（判断见上：职责窄、无替代、易替换，风险可接受）；日志落盘使敏感信息面扩大（已由 Secret 类型化、`0600/0700` 权限、不记消息内容三重约束兜住）；lumberjack **假定单进程写同一文件**——本项目单进程 + SQLite 单写者，一致；`slog.SetDefault` 是进程级全局，测试替换需 `t.Cleanup` 恢复。
- **后续跟进**：
  1. 写一条测试：跑完整请求后断言日志缓冲中**不出现明文 API key**（与 §7「`apiKey` 不出现在任何响应/日志」同源，此测试给机制兜底）。
  2. README 写明日志位置（`<data-dir>/logs/xgulu.log`）、如何关闭落盘（`--log-dir=`）、以及旧文件如何清理；报障模板要求附日志文件。
  3. 若将来接入 OpenTelemetry，用 `otelslog` 的 handler 包一层即可，业务代码零改动——这正是选标准库接口而非具体实现的价值。
  4. 若 lumberjack 长期无更新且出现明确的维护问题，退路是换任一实现了 `io.Writer` 的轮转组件（或自研），改动局限在 `internal/logging` 一个文件。
