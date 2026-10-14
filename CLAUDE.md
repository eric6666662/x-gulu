# x-gulu

<!-- BEGIN:FLOW-INIT （由 /flow-init 维护，勿手改；人写内容请放在标记区块外） -->

# 项目概况

> 本节由 `/flow-init` 生成 / 刷新；代码演进后重跑 `/flow-init` 即可更新。

## 项目定位

自托管的类豆包开源个人 AI 聊天工具（阶段 1 为极简 Chat 克隆），目标用户是**通用用户（含不写代码者）**；远期方向为辅助办公，四阶段路线图：聊天 → AI 搜索 → 助手门户 → Markdown 笔记。动机优先级：C 学习/开源作品 > A 数据自主 > B 模型可及性。

## 项目状态

**半成品（设计完成、编码未开工）** · 初始化于 2026-10-14，最近刷新 2026-10-14

- 文档层齐全：`00-构想` / `01-需求` / `02-前端·后端·ui 设计` + **10 个 ADR**（9 接受 + 1 废弃，ADR-0009 由 ADR-0010 取代）+ `docs/设计稿.md` + `GLOSSARY.md`
- UI 层另有可交互原型 `02-ui原型.html`（单文件，10 个场景，2725 行）
- 代码层为零：Go 源码 0 个、无 `package.json`、无构建/测试/lint 配置
- **git 仓库已初始化**（2026-10-14，分支 `main`，尚无提交；`.gitignore` 已备好）

## 技术栈

> 以下来自 `docs/adr/` 的已接受决策；依赖尚未写入 `go.mod` / `package.json`（均未引入）。

- 语言：Go 1.26.1（`go.mod` module `github.com/xgulu`）+ TypeScript
- 框架：
  - 后端：Go 标准库 HTTP + SSE（ADR-0003：SSE 而非 fetch 轮询）；路由用 Go 1.22+ 的 `http.ServeMux` 方法+通配符模式，**不引 chi/gorilla**
  - 前端：Vue 3（`<script setup>`）+ Vite + 自研组件 + 无样式底座 Reka UI（ADR-0001）
- 包管理 / 构建：Go modules（本机 go1.26.1）；前端 **pnpm**（2026-10-14 定，本机 12.8.1 / node v26.10.0），前端工程放 `web/`，构建产物 `web/dist` 经 `embed.FS` 打进二进制（lockfile 尚未生成）
- 测试框架：Vitest + @testing-library/vue（ADR-0001，未安装）；Go 侧用标准 `testing` + `httptest`，时间相关用例用 `testing/synctest`（Go 1.25+ 正式可用，见 02-后端设计 §7）
- 关键依赖（计划中，**当前 `go.mod` 无 `require`**）：
  - `modernc.org/sqlite` — 纯 Go SQLite 驱动，WAL + `MaxOpenConns(1)`（ADR-0004）
  - `markdown-it`（`html:false`）+ `highlight.js` — 回答渲染（ADR-0001）
  - Provider 适配层：OpenAI 兼容 + Anthropic 原生 + Ollama（ADR-0005）
  - API 契约不带版本号（ADR-0007）
  - **日志**：标准库 `log/slog`（JSON→stderr + 文件双写），**不引 zap/zerolog/logrus**；轮转交给 `gopkg.in/natefinch/lumberjack.v2`（v2.2.1，低维护但职责极窄、易替换）——按大小/时间切割，落盘 `<data-dir>/logs/`（ADR-0008）
  - **依赖装配**：**手写显式构造**，不引 DI 工具/框架（ADR-0010）。原计划的 `github.com/google/wire` 已弃用（ADR-0009，已废弃）——该库 2025-08-25 归档不再维护，且不支持泛型 provider

## 运行 · 测试 · 构建

| 用途 | 命令 |
|------|------|
| 启动 | 未探测到（无 Makefile / npm scripts / main 包） |
| 测试 | 未探测到（无测试框架配置） |
| 构建 | 未探测到 |
| 类型检查 / lint | 未探测到（无 eslint / tsconfig） |

## 目录结构

```
x-gulu/
├── CLAUDE.md              项目概况（本文件，FLOW-INIT 标记区块自动维护）
├── GLOSSARY.md            项目术语表（flow-grill 维护）
├── .gitignore             已备好（/bin、*.db*、/logs）；仓库尚未 init
├── go.mod                 仅 module + go 版本，无 require
├── docs/
│   ├── 需求图.md           需求登记表（项目级）
│   ├── 设计稿.md           视觉设计系统（flow-design-ui 维护）
│   ├── adr/               9 篇架构决策记录（ADR-0001~0009，均已接受）
│   └── deliverables/chat-mvp/  00-构想、01-需求、02-前端/后端/ui 设计、02-ui原型.html
├── .claude/skills/        flow-* 流水线技能
└── .idea/                 IDE 配置
```

> 预期但尚未创建（计划见 `02-后端设计.md` §3）：`cmd/xgulu/main.go`、`internal/{app,config,store,provider,context,chat,httpapi,export,migrate,apperr,logging,secret}`、`migrations/`、前端工程目录（`package.json` 等）、`README.md`。

## 完成度

| 模块 | 状态 | 说明 |
|------|------|------|
| 构想 / 需求 | ✅ 已完成 | `00-构想.md`、`01-需求.md`（阶段 1，MVP 14 项 + 完整版 6 项） |
| 前端 / 后端 / UI 设计 | ✅ 已完成 | `02-前端设计.md`、`02-后端设计.md`、`02-ui设计.md` + 设计稿 |
| 架构决策 | ✅ 已完成 | ADR-0001~0010（0001~0008、0010 已接受；0009 已废弃，由 0010 取代）——含日志 slog、手写依赖装配 |
| UI 原型 | ✅ 已完成 | `02-ui原型.html`，10 个场景可交互；⚠️ 一次性原型，非生产代码 |
| 术语表 | ✅ 已完成 | GLOSSARY.md，13 条业务术语 |
| 后端实现 | ⬜ 未开工 | 0 个 `.go` 文件 |
| 前端实现 | ⬜ 未开工 | 无 `package.json` |
| 数据库迁移 | ⬜ 未开工 | 无 `migrations/` |
| 测试 | ⬜ 未探测到 | 无任何测试配置或用例 |
| CI / lint | ⬜ 未探测到 | 尚无 workflow；计划加 golangci-lint（含 depguard 依赖方向检查，见 02-后端设计 §3） |
| git 版本控制 | ✅ 已初始化 | 分支 `main`，**尚无首个提交**；工作区 7 项未跟踪（`.claude/`、`.gitignore`、`.idea/`、`CLAUDE.md`、`GLOSSARY.md`、`docs/`、`go.mod`） |
| README / 开源文档 | ⬜ 未探测到 | 需求要求「达到可开源展示水准」 |

<!-- END:FLOW-INIT -->

## 人写补充（此区块外由维护者自行编辑）
