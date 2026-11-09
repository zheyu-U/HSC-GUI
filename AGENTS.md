# AGENTS.md

给 AI 编码代理看的工程说明。改代码前先读这一份。

## 这是什么

[hust-smartcourse-cli](https://github.com/xcdoge/hust-smartcourse-cli) 的 GUI 外壳。
技术栈 **Wails v2**（Go 后端 + Vue 3 前端），产物单文件 exe。

本仓库只做编排和展示，**不 fork 业务逻辑**。协议细节（CAS 登录、播放签名、
交卷端点、限速判断）全部留在上游代码里。

## 计划与步骤的维护规范（硬要求）

本仓库有两层进度跟踪，都是**项目文件夹里的纯文件**，不依赖任何特定
AI 工具或会话状态。**代理每次动手前先读这两个文件，状态变了就同步更新**。

### 文件位置

| 层 | 位置 | 入库 |
| --- | --- | --- |
| 宏观进度 | `plan.md`（项目根目录，本文件旁边） | 否，`.gitignore` 排除 |
| 节点进度 | `todos/` 目录，一个节点一个 `.md` 文件，文件名即节点 id | 否，`.gitignore` 排除 |

两者都不入库，克隆仓库拿不到——新环境从 `plan.md` 的节点清单表重建
`todos/`（一个节点一个文件，状态全标 pending，已完成的照阶段条标 done）。

### 宏观进度：`plan.md`

- 顶部是阶段条：`[1] 决策 [2] 环境 [3] 骨架 [4] 库化 [5] 登录 [6] 验证`，
  标出当前在哪个阶段
- 下面是：已拍板的决策表、环境现状、上游关键事实、节点清单、风险
- 阶段推进、决策变更、发现新风险时更新它
- 写法自由，但阶段条、决策表、节点清单表三节必须有

### 节点进度：`todos/`

一个节点一个文件，文件名即节点 id（kebab-case）加 `.md`，格式见
`todos/README.md`：标题、状态（pending/in_progress/done/blocked）、
依赖、一句话内容。开始节点标 in_progress，做完标 done，
卡住标 blocked 并写原因。

### 写入顺序

- 新步骤：先在 `todos/` 建文件（写清依赖），再在 `plan.md` 节点清单表登记一行
- 步骤完成：先把 `todos/<id>.md` 标 done；阶段内节点全完成时更新 `plan.md` 阶段条
- 两边不一致时以 `todos/` 为准，并立刻修正 `plan.md`

## 上游集成：vendor 改库

上游 `go/` 是 `package main`，函数未导出，不能直接 import。做法：

- `scripts/sync-upstream.ps1` 从 `codeload.github.com` 下载上游 main 分支 tarball
  （本机 github.com:443 直连不可用，**不要用 `git clone`**），按**功能白名单**复制到
  `internal/hsc/`，改 `package hsc`
- 首版登录白名单 13 个文件：`config.go httpc.go jar.go jar_uid.go util.go session.go
  creds.go cred_cmd.go cas.go rsa512.go captcha.go login_cmd.go logout.go`
- 不复制 CLI 入口（`main.go` `help.go` `pause.go`）和仓库里的 `hsc-pr.exe`；
  `args.go` 等辅助文件只在白名单功能需要时加入
- **不改上游文件逻辑**：GUI 适配集中在新文件 `internal/hsc/gui.go`，包括启动时切到
  exe 所在目录、登录分步编排、进度回调、导出包装。上游更新后重跑脚本，适配文件不动
- `data/captcha-model.json` 的 go:embed 相对路径保持不动
  （`internal/hsc/data/captcha-model.json`）
- 新增 GUI 功能时先扩白名单、再重跑脚本，并把新增文件和补丁点写进脚本注释

## 状态兼容（硬要求）

GUI 直接读写与上游 CLI **完全相同**的 `state/` 布局，加密凭据格式不变
（AES-256-GCM + scrypt，密钥绑机器）。CLI 登录过 GUI 即用，反之亦然。
路径一律走上游 `statePath(name)`，不手搓 `state/xxx`。

## 文案约定（硬要求）

- UI 文案和日志**朴素**：中文、无 emoji、无 `✔ ✘ ▶ ⚠` 符号
- 报错要能直接指导下一步动作，不抛裸路径或堆栈

## Git 规范（硬要求）

- 提交信息中文，`类型: 改了什么`，类型选：`新增` `修复` `重构` `文档` `构建` `同步上游`
- 一个提交只做一件事；提交前必过 `go build ./... && go vet ./... && go test ./...`，
  前端改动加 `cd frontend && npm.cmd run build`
- 永不入库：`state/` `build/bin/` `node_modules/` `frontend/dist/` `*.exe` `*.log`
  `plan.md`、凭据/cookie。`build/` 下的图标、清单、安装脚本要入库（Wails 构建需要），
  只排除产物目录 `build/bin/`。新增产物路径先补 `.gitignore` 再提交
- 不 rewrite 已推送历史；`main` 保持可构建，功能走 `feat/xxx` + squash 合并

## 文档规范（硬要求）

- 代码改行为，文档同步改，同一次提交内完成
- 本文件是工程说明单一事实源：目录、集成方式、构建命令、约定有变动先改这里
- 前后端绑定方法与事件清单维护在 `docs/api.md`，加/改绑定时同步更新
- 引用上游行为给锚点（如「见上游 go/cas.go」），不复制协议细节——复制即过期

## 构建

```bash
wails build                 # 单文件 exe（Wails v2.17.1，release 模式）
wails build -tags debug -o hsc-gui-debug.exe   # debug 模式：额外调试面板与调试日志
wails dev                   # 开发模式（release 界面；加 -tags debug 为 debug 界面）
go build ./... && go vet ./... && go test ./...   # 后端检查
go build -tags debug ./... && go vet -tags debug ./...   # debug 构建检查
cd frontend && npm.cmd install && npm.cmd run build   # 前端构建

# 同步上游（本机执行策略禁止直接跑 .ps1）
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts\sync-upstream.ps1

# 与上游 CLI 的凭据互通测试（可选，需要先编译上游 CLI）
$env:HSC_CLI_BIN = "<path>\hsc.exe"
go test ./internal/hsc -run TestCredentialInteropWithCLI -v
```

模式差异：release 界面只显示关键信息（账号、登录状态、是否记住账号、课程配置），
不显示 state 路径、cookie 数、原始 JSON；debug 额外显示调试面板、绑定调用日志和原始数据。
构建模式由构建标签 `debug` 决定（`app_mode_debug.go` / `app_mode_release.go`），
前端启动时通过 `AppMode` 绑定判断。

注意：`scripts/*.ps1` 必须保存为 **UTF-8 带 BOM**，否则 Windows PowerShell 5.1
会按 ANSI 解析中文注释而报语法错误；本机没有 PowerShell 7（`pwsh`）。

## 边界（沿用上游）

不加：绕过平台风控的新手段、批量账号操作、把答案分享出去的功能。
