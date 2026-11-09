# 前后端接口（Wails v2 绑定）

绑定对象是 `app.go` 里的 `App`，前端生成文件在 `frontend/wailsjs/go/main/App.js`
与 `App.d.ts`，由 `wails build` / `wails dev` 自动生成，**不要手改**。
改绑定方法时同步更新本文件。

## 通用约定

- 每个方法都返回 Promise；出错走 Promise reject，错误文本是可以直接展示的中文提示
- state 目录与 hsc CLI 相同：开发模式下是项目根目录，打包后是 exe 所在目录；
  见上游 `go/config.go` 的 `stateLayout`，路径一律走 `statePath(name)`
- 文案朴素：中文、无 emoji、无符号；报错给出下一步动作
- 构建模式：默认 release；`-tags debug` 构建 debug。前端先调 `AppMode`，debug 构建才显示调试面板与调试信息

## 方法

### StartQRLogin() -> QRLoginResult

生成企业微信扫码登录二维码，并在后台轮询登录态。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| image | string | PNG 的 data URL，直接给 `<img>` 用 |
| expiresAt | number | 过期时间，Unix 秒；前端用它做倒计时 |

- 二维码同时写到 `state/auth/login-qr.png`（与 CLI 相同路径）
- 轮询进度通过 `login:qr` 事件推送；60 次 × 3 秒，约 3 分钟
- 已有扫码在进行时 reject：先取消再生成

### CancelQRLogin() -> void

停止后台轮询。事件会再推一条 `cancelled`。

### StartCaptcha() -> CaptchaView

打开登录页并取一张验证码，自动识别结果作为建议值。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| image | string | 验证码图片的 data URL（GIF 或 PNG） |
| suggestedCode | string | 自动识别结果，4 位数字；识别失败为空 |
| confidence | number | 自动识别置信度 0 到 1 |
| count | number | 识别出的字符数 |
| hint | string | 给用户看的提示（识别把握不大时提示核对） |

- 验证码同时落盘 `state/auth/login-captcha.gif` 或 `.png`（与 CLI 一致）
- 每次调用都会新建一次登录页会话，旧验证码随之作废

### LoginPassword(username, password, code) -> LoginResult

提交账号、密码与验证码。`code` 为空时使用上一步的自动识别建议。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| ok | boolean | 是否登录成功 |
| message | string | 结果说明，可直接展示 |
| credSaved | boolean | 是否已把凭据加密写入 `state/auth/cred.enc.json` |
| need2fa | boolean | 账号需要企业微信二次验证（本版本不支持自动处理） |

- 成功后保存凭据，格式与 CLI `hsc cred save` 完全一致
- 验证码错误或账号密码错误时 `ok=false`，验证码会话作废，前端应重新取验证码

### SessionStatus() -> SessionInfo

读取本地登录态与凭据状态，不发起网络请求。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| loggedIn | boolean | `state/auth/cookies.json` 里是否有 cookie |
| cookieCount | number | cookie 条数 |
| account | string | 已保存凭据里的账号（打码） |
| credSaved | boolean | 是否有可用凭据 |
| credEncrypted | boolean | 凭据是否为加密格式 |
| hasTarget | boolean | 是否已有 `state/course/target.json` 课程配置 |
| stateDir | string | 当前 state 目录绝对路径 |
| message | string | 状态说明，可直接展示 |

### CheckSession() -> CheckResult

用当前 cookie 做一次站点校验；没有课程配置时只确认本地登录态。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| ok | boolean | 登录态是否可用 |
| checked | boolean | 是否真的做了站点校验 |
| hasTarget | boolean | 是否有课程配置 |
| chapters | number | 章节页解析出的章节数（做了校验时有效） |
| message | string | 结果说明，可直接展示 |

- 无课程配置时返回 `checked=false`，提示未做站点校验（对应 CLI `hsc check` 需要 target）
- 上游实现见 `go/cas.go` 的 `verifySite`

### Logout(all) -> LogoutResult

清除登录态。`all=true` 时连凭据一起清（对应 CLI `hsc logout --all`）。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| removed | string[] | 实际删除的文件名，如 `cookies.json`、`cred.enc.json` |
| message | string | 结果说明，可直接展示 |

### ListCourses(refresh) -> CourseItem[]

获取当前账号的课程列表，需要已登录。

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| refresh | boolean | true 时重新拉取；false 优先用 `state/course/courses.json` 缓存 |

返回 `CourseItem` 数组，字段：`index`、`name`、`teacher`、`courseId`、`clazzId`、`cpi`。
未登录、会话失效、列表为空时 reject，错误文本可直接展示。

### SelectCourse(index) -> CourseDetail

按列表序号选课：写 `state/course/target.json`，必要时重新抓取课程结构。
序号对应刚拉取的列表（与 CLI `hsc init <序号>` 一致）；抓取可能需要几秒。
上游实现见 `go/courses.go` 的 `switchCourseOpt`、`go/course.go` 的 `crawlCourse`。

### RefreshCourse() -> CourseDetail

对当前课程重新抓取章节、视频与作业，写 `state/course/course.json`。
没有课程配置或未登录时 reject。

### GetCourseDetail() -> CourseDetail

读取本地课程结构缓存，不发起网络请求。

`CourseDetail` 字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| hasCourse | boolean | 是否已有课程结构缓存 |
| stale | boolean | 缓存属于另一门课（需要重新抓取） |
| target | Target | 课程参数：`name` `courseId` `clazzId` `cpi` `enc` |
| summary | CourseSummary | `chapters` `videos` `videosPassed` `works` `durationHours` `speedLimited` `fetchedAt` |
| chapters | CourseChapter[] | `index` `id` `title`（上游章节接口不返回标题，可能为空）`videos` `videosPassed` `works` |
| videos | CourseVideo[] | `chapterIndex` `chapterId` `title` `duration`（秒）`isPassed` `speedLimited` `playTime` |
| works | CourseWork[] | `chapterIndex` `chapterId` `title` `workId` `workType` |
| message | string | 状态说明，可直接展示 |

### AppMode() -> AppModeInfo

返回构建模式与版本。前端在启动时调用一次，决定显示 release 还是 debug 界面。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| mode | string | `release` 或 `debug` |
| debug | boolean | 是否 debug 构建 |
| version | string | 应用版本号 |

### DebugInfo() -> DebugInfo

运行环境信息，仅 debug 构建可用；release 构建调用会 reject。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| mode | string | 固定 `debug` |
| stateDir | string | 当前 state 目录绝对路径 |
| cwd | string | 当前工作目录 |
| exePath | string | 可执行文件路径 |
| goVersion | string | Go 运行时版本 |
| os / arch | string | 操作系统与架构 |
| cookieCount | number | 当前 cookie 条数 |
| loggedIn | boolean | 本地是否存在登录态 |

### DebugLog() -> DebugEntry[]

最近的绑定调用与事件日志，仅 debug 构建可用；release 构建返回空数组。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| time | string | `HH:MM:SS` |
| kind | string | `启动` `调用` `结果` `错误` `事件` |
| message | string | 日志内容 |

最多保留最近 300 条，不写入磁盘。

### ClearDebugLog() -> void

清空调试日志，仅 debug 构建有效。

## 事件

### login:qr

扫码登录进度。前端 `EventsOn('login:qr', handler)` 订阅，组件卸载时取消订阅。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| state | string | `waiting` `scanned` `success` `expired` `cancelled` `error` |
| message | string | 给用户看的一句话 |

状态含义：

- `waiting`：已生成二维码，等待扫码
- `scanned`：已扫码，正在换取登录态
- `success`：登录成功，登录态已写入 `state/auth/cookies.json`
- `expired`：3 分钟内没有扫码结果
- `cancelled`：用户取消
- `error`：换取登录态或保存失败，message 为原因

## 上游锚点

- 扫码与账密流程：上游 `go/login_cmd.go`
- CAS 登录页、验证码、提交与换票：上游 `go/cas.go`
- 验证码识别：上游 `go/captcha.go`
- 凭据加密格式：上游 `go/creds.go`
- 状态路径映射：上游 `go/config.go` 的 `stateLayout`
- GUI 侧的编排代码：`internal/hsc/gui.go`（本仓库新增，不属于上游同步范围）
