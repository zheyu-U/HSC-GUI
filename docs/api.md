# 前后端接口（Wails v2 绑定）

绑定对象是 `app.go` 里的 `App`，前端生成文件在 `frontend/wailsjs/go/main/App.js`
与 `App.d.ts`，由 `wails build` / `wails dev` 自动生成，**不要手改**。
改绑定方法时同步更新本文件。

## 通用约定

- 每个方法都返回 Promise；出错走 Promise reject，错误文本是可以直接展示的中文提示
- state 目录与 hsc CLI 相同：开发模式（`wails dev`）下是项目根目录，打包后是 exe 所在目录；
  打包后的 exe 从任意工作目录启动都会切到 exe 目录，见 `internal/hsc/gui.go` 的
  `resolveStateRoot`；见上游 `go/config.go` 的 `stateLayout`，路径一律走 `statePath(name)`
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
| account | string | 账号（打码）：优先取已保存凭据；扫码登录从 cookie 的 `uname` 取 |
| accountTail | string | 账号最后 3 位，顶栏只显示这个尾号；账号未知时为空，顶栏显示“已登录” |
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

### ForgetCredentials() -> SessionInfo

删除本机保存的账号凭据（加密文件或明文 `cred.txt`），**不动当前登录态**。
返回刷新后的 `SessionInfo`，账号菜单里的“记住账号”会变成未保存。

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

### SelectCourseByURL(raw) -> CourseDetail

从课程页链接选课：解析 URL 里的 `courseid` / `clazzid` / `cpi` / `enc`，写
`state/course/target.json` 后立刻抓取课程结构。链接里没有 `courseid` 或 `clazzid`
时 reject，提示从课程页地址栏复制完整链接。上游解析见 `go/init_cmd.go` 的 `targetFromURL`。

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
| docScanned | boolean | 是否已有当前课程的 `state/course/scan.json`（文档任务扫描结果） |
| target | Target | 课程参数：`name` `courseId` `clazzId` `cpi` `enc` |
| summary | CourseSummary | `chapters` `videos` `videosPassed` `works` `docs` `docsPassed` `durationHours` `speedLimited` `fetchedAt` |
| chapters | CourseChapter[] | `index` `id` `title`（上游章节接口不返回标题，可能为空）`videos` `videosPassed` `works` |
| videos | CourseVideo[] | `chapterIndex` `chapterId` `objectId` `title` `duration`（秒）`isPassed` `speedLimited` `playTime` |
| works | CourseWork[] | `chapterIndex` `chapterId` `title` `workId` `workType` |
| docs | CourseDoc[] | 文档任务：`chapterIndex` `chapterId` `title` `module` `isPassed`；未扫描时为空数组 |
| message | string | 状态说明，可直接展示 |

`CourseDetail` 里的 `chapters` / `videos` / `works` / `docs` 始终是数组（空列表也返回 `[]`，
不会返回 `null`），前端可以直接取 `.length`；新增列表字段时不要在 Go 里返回 nil slice。

### ScanCourse() -> CourseDetail

逐章扫描附件（对应上游 `hsc scan`），结果写入 `state/course/scan.json`，返回刷新后的课程详情。
扫描需要已登录并已配置课程，可能要十几秒；GUI 用 busy 状态提示。

### CompleteDocs() -> CourseDetail

完成未完成的文档任务（对应上游 `hsc doc`）：调用上游 `cmdDoc` 逐个完成，再重新扫描
`scan.json` 刷新状态。未扫描时 reject，提示先点“扫描文档任务”。

### StartPlay(objectID, speed, limit) -> PlayStartResult

后台播放视频任务点（对应上游 `hsc play`）。立即返回，进度通过 `play:progress` 事件推送。

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| objectID | string | 为空时播放未完成任务点；给值只播这一个视频 |
| speed | number | 倍速，默认 1；限 1 倍速的任务点会自动按 1 倍播放 |
| limit | number | 大于 0 时本次最多处理这么多个任务点（objectID 模式忽略）；0 表示全部 |

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| running | boolean | 是否已开始播放 |
| total | number | 本次待播放的任务点数量 |
| message | string | 结果说明，可直接展示 |

- 播放循环在 `internal/hsc/gui.go` 里按 tick 编排，签名、上报参数与限速判断复用上游
  `report` / `videoMeta` / `refreshedVideo` / `courseRT` / `speedLimited`
- 每个任务点通过后会立刻把 `isPassed` 写回 `state/course/course.json`，CLI 读同一份缓存
- 同一时间只允许一个播放任务，重复调用 reject；未登录、未抓取课程结构时 reject

### CancelPlay() -> void

停止正在进行的播放：当前这一次上报完成后结束，事件会再推一条 `cancelled`。

### GetQuizOverview() -> QuizOverview

读取作业与测验列表和抓题情况，不发起网络请求。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| scanned | boolean | 是否已有 `state/course/scan.json`（课程附件扫描结果） |
| fetchedCount | number | 已抓取题目的份数 |
| questionCount | number | 已抓取题目总数 |
| items | QuizListItem[] | `workId` `title` `chapterIndex` `chapterId` `questionCount` `fetched` |
| message | string | 状态说明，可直接展示 |

- 列表来自 `scan.json` 里有 `workId` 的附件，按 `workId` 去重；题目缓存来自
  `state/quiz/quizzes.json`
- `fetched=false` 表示抓题时被上游跳过（通常是已提交或平台返回错误页）

### DumpQuizzes() -> QuizOverview

抓取未提交的作业/测验题目（对应上游 `hsc quiz dump`），写入 `state/quiz/quizzes.json`，
返回刷新后的列表。需要先扫描课程附件；已提交的会被上游跳过。

### GetQuizQuestions(workID) -> QuizDetailView

读取一份作业/测验的题目，只读，不返回答案与页面隐藏字段。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| workId | string | 作业/测验 ID |
| title | string | 标题 |
| questions | QuizQuestion[] | `qid` `typeLabel` `stem` `score` `options`（`value` `text`）`blankCount` |

- 没有抓过题时 reject，提示先抓取题目
- 只读浏览接口本身不接自动答题与交卷；不会读写 `state/quiz/answers.json`
- 自动答题与交卷在下面单独的方法里，写的是同一份 `state/quiz/answers.json`

### GetQuizSettings() -> QuizSettings

读取 `state/config.toml` 里的自动答题设置（与 CLI 共用），不发起网络请求。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| enabled | boolean | 是否启用自动答题 |
| baseUrl | string | 兼容 OpenAI 的接口地址 |
| apiKeyMasked | string | 已保存 Key 的脱敏值；未填写时为 `(未填写)` |
| apiKey | string | **读取时恒为空**，只用于保存；保存时留空表示保留原 Key |
| model | string | 模型名 |
| temperature | number | 采样温度 0 到 2 |
| maxTokens | number | 单次最大输出 |
| timeoutSeconds | number | 请求超时秒数 |
| autoSubmit | boolean | 答完是否自动交卷 |
| types | string[] | 参与作答的题型：`choice` `multi` `judge` `fill` `essay` |
| delayMs | number | 每题间隔毫秒 |
| retries | number | 单题重试次数 |
| message | string | 状态说明，可直接展示 |

文件不存在时按上游默认值生成。上游实现见 `go/config_toml.go` 的 `loadConfig`。

### SaveQuizSettings(settings) -> QuizSettings

写回 `state/config.toml`。字段不合法时 reject，错误文本可直接展示（温度 0 到 2、
max_tokens 1 到 128000、超时 1 到 600 秒、间隔 0 到 60000 毫秒、重试 0 到 10、
地址必须是 http(s)、题型必须在上面的白名单里）。`apiKey` 留空表示沿用原值。

### SolveQuizzes(only) -> QuizSolveResult

用配置的 LLM 接口自动作答（对应上游 `hsc quiz solve`），写入
`state/quiz/answers.json`（与旧答案合并）和 `state/quiz/qbank.json`。
进度通过 `quiz:progress` 事件推送。

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| only | number | `-1` 做全部；`>=0` 只做 `quizzes.json` 里该 Index 的一份 |

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| total | number | 本轮参与作答的题数 |
| ok | number | 成功题数（含题库命中） |
| cached | number | 题库命中题数 |
| failed | number | 失败题数 |
| message | string | 结果说明，可直接展示 |

- 配置了 `auto_submit=true` 且 `only<0` 时，作答完成后自动调用交卷（与上游一致）
- 同步阻塞调用，一次只能跑一轮；未启用、没有 Key、没抓题时 reject

### SubmitQuizzes(only) -> QuizSubmitResult

按 `state/quiz/answers.json` 交卷（对应上游 `hsc quiz submit`），只交未完成的。

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| only | number | `-1` 交全部；`>=0` 只交该 Index 的一份 |

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| ok | number | 提交成功的份数 |
| skipped | number | 没答案跳过的份数 |
| failed | number | 失败的份数 |
| message | string | 结果说明，可直接展示 |

交卷是状态变更操作，前端必须二次确认。协议与端点在上游 `go/quiz.go` 的 `cmdQuizSubmit`。

### GetQuizBank() / ClearQuizBank() -> QuizBankInfo

读取 / 清空题库缓存 `state/quiz/qbank.json`。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| count | number | 题库题数 |
| message | string | 状态说明，可直接展示 |

### PeekQuizzes(only) -> QuizPeekItem[]

只读读取每份测验页面标题，判断批阅状态（对应上游 `hsc quiz peek`）。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| index | number | `quizzes.json` 里的 Index（0 起） |
| title | string | 标题 |
| status | string | `已批阅` `已提交` `可作答` `未知` |

单份打不开时记 `未知`，不中断；`only` 语义同上。

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

### play:progress

视频播放进度。前端 `EventsOn('play:progress', handler)` 订阅，组件卸载时取消订阅。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| state | string | `starting` `video` `progress` `passed` `skipped` `failed` `finished` `cancelled` |
| message | string | 给用户看的一句话 |
| objectId | string | 当前视频的 objectId |
| title | string | 当前视频标题 |
| index | number | 当前第几个（从 1 开始；结束后为 0） |
| total | number | 本次总任务点数 |
| percent | number | 当前视频进度 0 到 100 |
| played | number | 已上报的播放秒数 |
| duration | number | 当前视频总秒数 |
| speedLimited | boolean | 当前任务点是否限 1 倍速 |
| ok / fail / skipped | number | 到目前为止通过 / 失败 / 跳过的数量 |

状态含义：

- `starting`：开始播放，message 里是总任务点数和总时长
- `video`：开始处理某个任务点
- `progress`：一次上报后的进度
- `passed`：该任务点通过，已写回 `course/course.json`
- `skipped`：服务端已标记通过，跳过
- `failed`：该任务点失败，message 里是原因
- `finished`：全部处理完，携带通过 / 失败 / 跳过数量
- `cancelled`：用户停止播放

### quiz:progress

自动答题进度。前端 `EventsOn('quiz:progress', handler)` 订阅，组件卸载时取消订阅。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| state | string | `starting` `solving` `answer` `done` `error` |
| message | string | 给用户看的一句话 |
| index | number | 当前第几题（1 起；done 时等于 total） |
| total | number | 本轮总题数 |
| question | string | 当前题 QID；没有 QID 时是截断题干 |
| answered | number | 已成功题数 |
| cached | number | 题库命中题数 |
| failed | number | 失败题数 |

状态含义：

- `starting`：开始作答，message 里是份数和题数
- `solving`：正在作答某一题
- `answer`：该题作答完成或失败跳过，继续下一题
- `done`：全部结束，message 是统计；开了自动交卷时包含交卷结果
- `error`：致命错误（未启用、没有 Key、没抓题等），本轮结束

## 上游锚点

- 扫码与账密流程：上游 `go/login_cmd.go`
- CAS 登录页、验证码、提交与换票：上游 `go/cas.go`
- 验证码识别：上游 `go/captcha.go`
- 凭据加密格式：上游 `go/creds.go`
- 状态路径映射：上游 `go/config.go` 的 `stateLayout`
- 播放上报、限速与签名：上游 `go/video.go`；开播前刷新参数：`go/video_refresh.go`
- 文档任务扫描与完成：上游 `go/scan_doc.go`
- 作业/测验抓题与解析：上游 `go/quiz.go`（`cmdQuizDump` / `parseQuiz`）
- 自动答题与 LLM 调用：上游 `go/quiz_solve.go` / `go/quiz_llm.go` / `go/quiz_bank.go`
- 交卷：上游 `go/quiz.go` 的 `cmdQuizSubmit`；批阅状态：`go/quiz_peek.go`
- LLM 配置：上游 `go/config_toml.go`（`state/config.toml`）
- GUI 侧的编排代码：`internal/hsc/gui.go`、`internal/hsc/quiz_auto.go`（本仓库新增，不属于上游同步范围）
