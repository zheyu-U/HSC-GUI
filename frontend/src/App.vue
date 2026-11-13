<script setup>
import {computed, nextTick, onMounted, onUnmounted, ref, watch} from 'vue'
import {
  AppMode,
  CancelPlay,
  CancelQRLogin,
  CheckSession,
  ClearDebugLog,
  ClearLocalSession,
  CompleteDocs,
  DebugInfo,
  DebugLog,
  DumpQuizzes,
  ForgetCredentials,
  GetCourseDetail,
  GetQuizBank,
  GetQuizOverview,
  GetQuizQuestions,
  GetQuizSettings,
  ListCourses,
  LoginPassword,
  Logout,
  OpenDebugLog,
  PeekQuizzes,
  RefreshCourse,
  SaveQuizSettings,
  ScanCourse,
  SelectCourse,
  SelectCourseByURL,
  SessionStatus,
  SolveQuizzes,
  StartCaptcha,
  StartPlay,
  StartQRLogin,
  SubmitQuizzes,
} from '../wailsjs/go/main/App'
import {EventsOff, EventsOn, IsNotificationAvailable, RequestNotificationAuthorization, SendNotification, BrowserOpenURL} from '../wailsjs/runtime/runtime'

const TITLES = [
  '今天打算刷什么？',
  '不只优雅，不止高效',
  '快进的是课，慢下来的是你',
  '省下的时间，都归你',
  '任务点，点到为止',
  '今天也要好好「过」',
  '让视频自己看完自己',
  '把时间留给需要的灵魂',
  '让每一分进度，都算数',
  '刷课，本来就该这么省心',
  '清空，再出发',
  '今天，就把课清了',
  '何止快，简直快',
  '省下的时间，都是你的',
  '把重复的日子，过成自己的样子',
  '你忙你的，课有它看',
  '不用管，它自己来',
  '课，我来；觉，你来',
]

const QUIZ_TYPES = [
  {value: 'choice', label: '单选'},
  {value: 'multi', label: '多选'},
  {value: 'judge', label: '判断'},
  {value: 'fill', label: '填空'},
  {value: 'essay', label: '简答'},
]

const page = ref('home')
const title = ref(TITLES[0])
const mode = ref({mode: 'release', debug: false, version: ''})
const session = ref(null)
const notice = ref('')
const noticeKind = ref('info')
// 成功提示正在淡出（这段时间里文案还留着，只是透明度降下去）
const noticeLeaving = ref(false)
const menuOpen = ref(false)
const aboutOpen = ref(false)
const aboutCloseRef = ref(null)
const helpOpen = ref(false)
const helpCloseRef = ref(null)

const view = ref('login')
const justLoggedIn = ref(false)
const tab = ref('qr')
const qrImage = ref('')
const qrMessage = ref('')
const qrRemain = ref(0)
const qrBusy = ref(false)
let qrTimer = null

const username = ref('')
const password = ref('')
const captchaImage = ref('')
const captchaHint = ref('')
const captchaCode = ref('')
const captchaBusy = ref(false)
const loginBusy = ref(false)
const checkBusy = ref(false)
const checkMessage = ref('')

const courseList = ref([])
const courseDetail = ref(null)
const courseLoading = ref(false)
const courseBusy = ref(false)
const courseError = ref('')
// 云端取数的加载提示文案（空字符串表示没在取数）
const cloudLoading = ref('')

// 底部消息条：加载中优先显示加载文案（带转圈），否则显示普通提示（成功 / 错误）
const messageText = computed(() => (cloudLoading.value ? cloudLoading.value : notice.value))
const messageKind = computed(() => (cloudLoading.value ? 'loading' : noticeKind.value))

// 加载提示出现时立刻把正在淡出的成功提示收掉，避免互相压着
watch(cloudLoading, (value) => {
  if (value) {
    noticeLeaving.value = false
  }
})

// 视频 / 文档清单的筛选状态：all | todo | done
const videoFilter = ref('all')
const docFilter = ref('all')
// 课程页当前选中的块：video | doc | quiz（三张入口卡当标签页）
const courseTab = ref('video')
const playLimit = ref(0)
const courseURL = ref('')
const courseURLBusy = ref(false)

const quizOverview = ref(null)
const quizBusy = ref(false)
const quizError = ref('')
const quizSelected = ref('')
const quizQuestions = ref([])
// 这次进课程页有没有自动从云端抓过题（本地没缓存时自动抓一次，不反复抓）
const quizAutoFetched = ref(false)
// 本次课程页会话是否已经自动扫描过附件（失败后不再反复自动扫，交给工具条按钮）
const quizAutoScanned = ref(false)
// 作业块工具条的主按钮：没扫描 → 扫描附件；扫描了没题 → 抓取题目；抓过 → 刷新
const quizPrimaryAction = computed(() => {
  const overview = quizOverview.value
  if (!overview || !overview.scanned) {
    return 'scan'
  }
  if (!(overview.items || []).length) {
    return 'none'
  }
  return Number(overview.fetchedCount) > 0 ? 'refresh' : 'fetch'
})
const quizPrimaryLabel = computed(() => {
  if (quizPrimaryAction.value === 'scan') {
    return '扫描附件'
  }
  if (quizPrimaryAction.value === 'fetch') {
    return '抓取题目'
  }
  return '刷新'
})
const quizReady = computed(() => {
  const overview = quizOverview.value
  return !!(overview && overview.scanned && (overview.items || []).length)
})
const quizSettings = ref(null)
const quizSettingsBusy = ref(false)
const quizSettingsError = ref('')
const quizBank = ref(null)
const quizSolving = ref(false)
const quizSubmitting = ref(false)
const quizSolveState = ref({
  state: '',
  message: '',
  index: 0,
  total: 0,
  question: '',
  answered: 0,
  cached: 0,
  failed: 0,
})
const quizPeekItems = ref([])
const quizPeekBusy = ref(false)
const quizPeekOpen = ref(false)
const sessionRepairing = ref(false)

const docBusy = ref(false)
const playRunning = ref(false)
const playSpeed = ref(1)
// 上报间隔（真实秒数），和倍速一起存在本机 localStorage
const playTick = ref(10)

const PLAY_SETTINGS_KEY = 'hsc.playSettings'

function clampNumber(value, min, max, fallback) {
  const n = Number(value)
  if (!Number.isFinite(n)) {
    return fallback
  }
  return Math.min(max, Math.max(min, n))
}

function loadPlaySettings() {
  try {
    const raw = window.localStorage.getItem(PLAY_SETTINGS_KEY)
    if (!raw) {
      return
    }
    const saved = JSON.parse(raw)
    playSpeed.value = clampNumber(saved.speed, 1, 25, 1)
    playTick.value = clampNumber(saved.tick, 1, 30, 10)
  } catch (e) {
    // 本机设置坏了就用默认值
  }
}

function savePlaySettings() {
  try {
    window.localStorage.setItem(PLAY_SETTINGS_KEY, JSON.stringify({
      speed: Number(playSpeed.value) || 1,
      tick: Number(playTick.value) || 10,
    }))
  } catch (e) {
    // 存不了就算了，本次运行仍然生效
  }
}

function resetPlaySettings() {
  playSpeed.value = 1
  playTick.value = 10
  savePlaySettings()
  setNotice('播放设置已恢复默认', 'ok')
}

loadPlaySettings()
watch([playSpeed, playTick], savePlaySettings)
const playState = ref({
  state: '',
  message: '',
  objectId: '',
  title: '',
  index: 0,
  total: 0,
  percent: 0,
  played: 0,
  duration: 0,
  speedLimited: false,
  ok: 0,
  fail: 0,
  skipped: 0,
})

// 播放失败的任务点（按 objectId）；同一次播放里重试通过会清掉，切课程/重抓结构时清空
const failedVideos = ref(new Set())

function markVideoFailed(objectID) {
  if (!objectID) {
    return
  }
  const next = new Set(failedVideos.value)
  next.add(objectID)
  failedVideos.value = next
}

function clearVideoFailed(objectID) {
  if (!objectID || !failedVideos.value.has(objectID)) {
    return
  }
  const next = new Set(failedVideos.value)
  next.delete(objectID)
  failedVideos.value = next
}

function isVideoFailed(objectID) {
  return !!objectID && failedVideos.value.has(objectID)
}

const debugInfo = ref(null)
const debugLog = ref([])
const debugBusy = ref(false)

const loggedIn = computed(() => !!(session.value && session.value.loggedIn))
const account = computed(() => (session.value && session.value.account) || '')
const accountTail = computed(() => (session.value && session.value.accountTail) || '')
const accountLabel = computed(() => {
  if (accountTail.value) {
    return '尾号 ' + accountTail.value
  }
  return loggedIn.value ? '已登录' : '登录'
})
const hasCourse = computed(() => !!(courseDetail.value && courseDetail.value.hasCourse))
const prettySession = computed(() => (session.value ? JSON.stringify(session.value, null, 2) : ''))
const lastDebugMessage = computed(() => {
  if (!debugLog.value.length) {
    return '暂无调用记录'
  }
  return debugLog.value[debugLog.value.length - 1].message
})
const currentCourseID = computed(() => {
  if (!courseDetail.value || !courseDetail.value.target) {
    return ''
  }
  return courseDetail.value.target.courseId || ''
})
const videosByChapter = computed(() => {
  const map = {}
  const list = (courseDetail.value && courseDetail.value.videos) || []
  for (const item of list) {
    if (!map[item.chapterId]) {
      map[item.chapterId] = []
    }
    map[item.chapterId].push(item)
  }
  return map
})
const unpassedVideos = computed(() => {
  const list = (courseDetail.value && courseDetail.value.videos) || []
  return list.filter((v) => !v.isPassed)
})
const playCount = computed(() => {
  const total = unpassedVideos.value.length
  return playLimit.value > 0 ? Math.min(playLimit.value, total) : total
})
const playbackPercent = computed(() => Math.min(100, Math.max(0, Math.round(Number(playState.value.percent) || 0))))
const quizSolvePercent = computed(() => {
  const total = Number(quizSolveState.value.total) || 0
  if (!total) {
    return 0
  }
  return Math.min(100, Math.max(0, Math.round((Number(quizSolveState.value.index) || 0) / total * 100)))
})
const docs = computed(() => (courseDetail.value && courseDetail.value.docs) || [])
const docScanned = computed(() => !!(courseDetail.value && courseDetail.value.docScanned))
const docUnpassed = computed(() => docs.value.filter((d) => !d.isPassed).length)
const docPassed = computed(() => docs.value.filter((d) => d.isPassed).length)

const chapters = computed(() => (courseDetail.value && courseDetail.value.chapters) || [])

// 视频页：总览数字与章节进度；列表全部列出，不做分页
const videoStats = computed(() => {
  const list = (courseDetail.value && courseDetail.value.videos) || []
  const passed = list.filter((v) => v.isPassed).length
  return {
    total: list.length,
    passed,
    todo: list.length - passed,
    percent: list.length ? Math.round((passed / list.length) * 100) : 0,
  }
})

function chapterVideos(ch) {
  return videosByChapter.value[ch.id] || []
}

function chapterPassed(ch) {
  return chapterVideos(ch).filter((v) => v.isPassed).length
}

function chapterPercent(ch) {
  const list = chapterVideos(ch)
  return list.length ? Math.round((chapterPassed(ch) / list.length) * 100) : 0
}

// 章节内按当前筛选只渲染匹配的视频；全部时不过滤
function chapterTaskVideos(ch) {
  const list = chapterVideos(ch)
  if (videoFilter.value === 'todo') {
    return list.filter((v) => !v.isPassed)
  }
  if (videoFilter.value === 'done') {
    return list.filter((v) => v.isPassed)
  }
  return list
}

// 默认只展开第一个还有未完成任务的章节（列表不再分页，全展开会太长）；筛选时全部展开
const firstTodoChapterId = computed(() => {
  const hit = chapters.value.find((ch) => chapterVideos(ch).some((v) => !v.isPassed))
  return hit ? hit.id : ''
})

function isChapterOpen(ch) {
  if (videoFilter.value !== 'all') {
    return true
  }
  return ch.id === firstTodoChapterId.value
}

const videoFilteredChapters = computed(() => {
  if (videoFilter.value === 'all') {
    return chapters.value
  }
  const wantPassed = videoFilter.value === 'done'
  return chapters.value.filter((ch) => chapterVideos(ch).some((v) => !!v.isPassed === wantPassed))
})

// 文档页：完成百分比；列表全部列出，不做分页
const docPercent = computed(() => (docs.value.length ? Math.round((docPassed.value / docs.value.length) * 100) : 0))

// 标题里已经带了“第 N 章”就不再重复显示章节
function docChapterLabel(d) {
  if (!d.chapterIndex) {
    return ''
  }
  const chapter = '第 ' + d.chapterIndex + ' 章'
  return String(d.title || '').indexOf(chapter) === 0 ? '' : chapter
}

const docFilteredDocs = computed(() => {
  if (docFilter.value === 'all') {
    return docs.value
  }
  const wantPassed = docFilter.value === 'done'
  return docs.value.filter((d) => !!d.isPassed === wantPassed)
})

const quizItems = computed(() => (quizOverview.value && quizOverview.value.items) || [])
const selectedQuizItem = computed(() => quizItems.value.find((q) => q.workId === quizSelected.value) || null)

const courseName = computed(() => {
  if (!hasCourse.value || !courseDetail.value.target) {
    return ''
  }
  return courseDetail.value.target.name || ''
})
const courseTeacher = computed(() => {
  if (!hasCourse.value || !courseDetail.value.target) {
    return ''
  }
  return courseDetail.value.target.teacher || ''
})

const pageTitle = computed(() => {
  if (page.value === 'login') {
    if (view.value === 'success') {
      return justLoggedIn.value ? '登录成功' : '已登录'
    }
    return '登录'
  }
  if (page.value === 'course') {
    return '课程'
  }
  if (page.value === 'coursehome') {
    return courseName.value || '课程'
  }
  if (page.value === 'settings') {
    return '测验设置'
  }
  if (page.value === 'play-settings') {
    return '播放设置'
  }
  if (page.value === 'debug') {
    return '调试'
  }
  return ''
})
const pageLead = computed(() => {
  if (page.value === 'login') {
    if (view.value === 'success') {
      return account.value ? '账号 ' + account.value : '登录态已保存到本机'
    }
    return '用企业微信扫码，或账号密码登录。'
  }
  if (page.value === 'course') {
    return '选择一门课程，进入课程主页。'
  }
  if (page.value === 'coursehome') {
    // 课程元信息放在标题下：任课教师 · 章节数 · 总时长（原“完成概况”搬到这里）
    const parts = []
    if (courseTeacher.value) {
      parts.push('任课教师 ' + courseTeacher.value)
    }
    const summary = courseDetail.value && courseDetail.value.summary
    if (summary) {
      if (summary.chapters) {
        parts.push(summary.chapters + ' 章')
      }
      if (summary.durationHours) {
        parts.push('总时长 ' + summary.durationHours + ' 小时')
      }
    }
    return parts.join(' · ') || '视频、文档和作业与测验都在这一页'
  }
  if (page.value === 'settings') {
    return '自动答题的接口、题型与交卷选项。'
  }
  if (page.value === 'play-settings') {
    return '倍速与上报间隔。'
  }
  return ''
})

const courseTabTitle = computed(() => {
  if (courseTab.value === 'doc') {
    return '文档任务'
  }
  if (courseTab.value === 'quiz') {
    return '作业与测验'
  }
  return '视频任务'
})

function friendly(e) {
  return String(e).replace(/^Error:\s*/, '')
}

let noticeTimer = 0

function clearNoticeTimer() {
  if (noticeTimer) {
    clearTimeout(noticeTimer)
    noticeTimer = 0
  }
  noticeLeaving.value = false
}

// 成功提示 1.5 秒后开始淡出（1 秒透明度过渡，文案在淡出期间保留）；错误和普通提示留到下一次操作
function setNotice(message, kind) {
  clearNoticeTimer()
  notice.value = message || ''
  noticeKind.value = kind || 'info'
  if (notice.value && noticeKind.value === 'ok') {
    const text = notice.value
    noticeTimer = setTimeout(() => {
      if (notice.value !== text) {
        return
      }
      noticeLeaving.value = true
      noticeTimer = setTimeout(() => {
        noticeTimer = 0
        if (notice.value === text) {
          notice.value = ''
        }
        noticeLeaving.value = false
      }, 1000)
    }, 1500)
  }
}

// 播放中点这些按钮不执行，只在消息条说明原因（按钮本身保持可点，disabled 会吞掉点击）
function sayPlaying() {
  setNotice('正在播放中，先停止播放再操作', 'info')
}

const AUTH_HINTS = ['未登录', '登录态', '会话', 'cookie', '重新登录', '登录已失效']
const PARSE_HINTS = ['解析', '页面结构', '返回内容过短', '没解析到', '没有解析到', '缺少 state']

function isAuthError(message) {
  return AUTH_HINTS.some((hint) => String(message).includes(hint))
}

function looksLikeParseError(message) {
  return PARSE_HINTS.some((hint) => String(message).includes(hint))
}

// 登录态被平台踢掉时，解析错误往往看不出是登录问题；这里用一次站点校验确认，
// 确认失效就清掉本地 cookie、回登录页，课程和测验缓存保留。
async function repairInvalidSession() {
  if (sessionRepairing.value) {
    return false
  }
  sessionRepairing.value = true
  try {
    const res = await CheckSession()
    if (res && !res.ok && /会话无效|被踢到|未登录|登录态/.test(res.message || '')) {
      try {
        session.value = await ClearLocalSession()
      } catch (e) {
        await refreshSession()
      }
      view.value = 'login'
      justLoggedIn.value = false
      showPage('login')
      setNotice('登录态已失效，已清掉本地登录信息，请重新登录', 'error')
      return true
    }
  } catch (e) {
    // 网络错误不清理本地登录态，避免误判
  } finally {
    sessionRepairing.value = false
  }
  return false
}

// API 报登录态失效时统一提示并回登录页；解析类错误先确认会话是否真的失效。
async function handleActionError(e, prefix) {
  const message = friendly(e)
  if (isAuthError(message)) {
    if (await repairInvalidSession()) {
      return
    }
    await refreshSession()
    showPage('login')
    setNotice('登录态已失效，请重新登录', 'error')
    return
  }
  if (looksLikeParseError(message) && await repairInvalidSession()) {
    return
  }
  setNotice(prefix ? prefix + '：' + message : message, 'error')
}

// 页内错误区：登录态失效统一跳登录页，其他错误写回调用方自己的错误状态。
async function handleInlineError(e, apply) {
  const message = friendly(e)
  if (isAuthError(message)) {
    await handleActionError(e)
    return
  }
  if (looksLikeParseError(message) && await repairInvalidSession()) {
    return
  }
  apply(message)
}

async function checkLoginState() {
  closeMenu()
  try {
    const res = await CheckSession()
    if (res && !res.ok && /会话无效|被踢到|未登录|登录态/.test(res.message || '')) {
      await repairInvalidSession()
      return
    }
    setNotice(res && res.ok ? '登录态有效' : (res.message || '检查失败'), res && res.ok ? 'ok' : 'error')
  } catch (e) {
    setNotice('检查登录态失败: ' + friendly(e), 'error')
  }
}

function emptyPlayState() {
  return {
    state: '',
    message: '',
    objectId: '',
    title: '',
    index: 0,
    total: 0,
    percent: 0,
    played: 0,
    duration: 0,
    speedLimited: false,
    ok: 0,
    fail: 0,
    skipped: 0,
  }
}

function closeMenu() {
  menuOpen.value = false
}

function toggleMenu() {
  menuOpen.value = !menuOpen.value
}

async function openAbout() {
  closeMenu()
  aboutOpen.value = true
  await nextTick()
  if (aboutCloseRef.value) {
    aboutCloseRef.value.focus()
  }
}

function closeAbout() {
  aboutOpen.value = false
}

async function openHelp() {
  closeMenu()
  helpOpen.value = true
  await nextTick()
  if (helpCloseRef.value) {
    helpCloseRef.value.focus()
  }
}

function closeHelp() {
  helpOpen.value = false
}

function onDocumentClick() {
  closeMenu()
}

function onKeydown(e) {
  if (e.key === 'Escape') {
    closeMenu()
    if (aboutOpen.value) {
      closeAbout()
    }
    if (helpOpen.value) {
      closeHelp()
    }
  }
}

const PAGE_PARENT = {
  coursehome: 'course',
  settings: 'coursehome',
  'play-settings': 'coursehome',
}

function parentPage(name) {
  return PAGE_PARENT[name] || 'home'
}

function goBack() {
  showPage(parentPage(page.value))
}

function showPage(name) {
  if (name === 'debug' && !mode.value.debug) {
    return
  }
  if ((name === 'coursehome' || name === 'settings' || name === 'play-settings') && !hasCourse.value) {
    name = loggedIn.value ? 'course' : 'login'
  }
  if (page.value !== name) {
    window.scrollTo(0, 0)
  }
  if (page.value === 'coursehome' || name === 'coursehome') {
    resetCoursehomeFade()
  }
  if (name === 'coursehome' && !courseDetail.value && loggedIn.value) {
    loadCourseDetail()
  }
  page.value = name
  closeMenu()
  setNotice('')
  if (name === 'login' && !loggedIn.value && !qrImage.value) {
    generateQr()
  }
  if (name === 'course' && loggedIn.value) {
    if (!courseDetail.value) {
      loadCourseDetail()
    }
    if (!courseList.value.length && !courseLoading.value) {
      loadCourses(true)
    }
  }
  if (name === 'settings') {
    loadQuizSettings()
    loadQuizBank()
  }
  if ((name === 'home' || name === 'debug') && mode.value.debug) {
    refreshDebug()
  }
}

// 播放设置：播放中不让进，避免以为改了会对当前任务生效
function openPlaySettings() {
  if (playRunning.value) {
    sayPlaying()
    return
  }
  showPage('play-settings')
}

// 课程页的三张入口卡当标签页用：视频/文档都用课程缓存；作业块有本地缓存就直接读，
// 没有缓存就自动去云端抓一次。
function selectCourseTab(name) {
  if (courseTab.value === name) {
    return
  }
  courseTab.value = name
  window.scrollTo(0, 0)
  if (name === 'quiz') {
    ensureQuizData()
  }
}

async function ensureQuizData() {
  if (!quizBank.value) {
    loadQuizBank()
  }
  if (quizBusy.value) {
    return
  }
  if (!quizOverview.value) {
    await loadQuizOverview()
  }
  let overview = quizOverview.value
  if (!overview) {
    return
  }
  // 播放中不自动做云端动作（按钮此时也是不可用的），等停播后再进块
  if (playRunning.value) {
    return
  }
  // 本地还没有扫描结果：进块自动扫描课程附件，扫完再决定要不要抓题
  if (!overview.scanned && !quizAutoScanned.value) {
    quizAutoScanned.value = true
    await scanForQuiz(true)
    overview = quizOverview.value
    if (!overview) {
      return
    }
  }
  if (!overview.scanned || quizAutoFetched.value) {
    return
  }
  const items = overview.items || []
  if (!items.length || Number(overview.fetchedCount) > 0) {
    return
  }
  // 有扫描结果但一份题都没抓过：当作没有本地缓存，自动从云端取
  await refreshQuizzes()
}

function openCourse() {
  if (!loggedIn.value) {
    showPage('login')
    return
  }
  showPage('course')
}

async function openCourseRow(item) {
  if (playRunning.value) {
    sayPlaying()
    return
  }
  if (!item || courseBusy.value || courseLoading.value) {
    return
  }
  if (isCurrent(item) && hasCourse.value) {
    showPage('coursehome')
    return
  }
  await selectCourse(item.index)
  if (!courseError.value && hasCourse.value) {
    showPage('coursehome')
  }
}

async function openCourseByURL() {
  if (playRunning.value) {
    sayPlaying()
    return
  }
  const raw = courseURL.value.trim()
  if (!raw || courseURLBusy.value) {
    return
  }
  courseURLBusy.value = true
  courseError.value = ''
  setNotice('')
  cloudLoading.value = '正在从云端打开课程…'
  try {
    const detail = await SelectCourseByURL(raw)
    courseDetail.value = detail
    playState.value = emptyPlayState()
    courseTab.value = 'video'
    videoFilter.value = 'all'
    docFilter.value = 'all'
    quizOverview.value = null
    quizSelected.value = ''
    quizQuestions.value = []
    quizAutoFetched.value = false
    quizAutoScanned.value = false
    failedVideos.value = new Set()
    courseURL.value = ''
    await refreshSession()
    showPage('coursehome')
    setNotice('已按链接选择课程', 'ok')
  } catch (e) {
    await handleActionError(e, '按链接选课失败')
  } finally {
    courseURLBusy.value = false
    cloudLoading.value = ''
    refreshDebug()
  }
}

// 课程主页滚动动画：标题和入口卡在视口里定住，课程结构面板升上来盖住时按滚动进度淡出。
let coursehomeFadeFrame = 0

function resetCoursehomeFade() {
  const head = document.querySelector('.page-head-center')
  if (head) {
    head.style.opacity = ''
    head.style.transform = ''
  }
  const cards = document.querySelector('.entry-list')
  if (cards) {
    cards.style.opacity = ''
    cards.style.transform = ''
  }
}

function updateCoursehomeFade() {
  if (page.value !== 'coursehome') {
    return
  }
  const root = document.querySelector('.coursehome-page')
  if (!root) {
    return
  }
  const head = root.querySelector('.page-head-center')
  const cards = root.querySelector('.entry-list')
  const panel = root.querySelector('.panel')
  if (!head || !cards || !panel) {
    return
  }
  if (window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
    head.style.opacity = ''
    head.style.transform = ''
    cards.style.opacity = ''
    cards.style.transform = ''
    return
  }
  const viewport = window.innerHeight || 720
  const scrolled = window.scrollY || document.documentElement.scrollTop || 0
  const panelTop = panel.getBoundingClientRect().top + scrolled
  const distance = Math.max(180, Math.min(panelTop - viewport * 0.34, 560))
  const linear = Math.min(1, Math.max(0, scrolled / distance))
  const ease = (value) => value * value * (3 - 2 * value)
  const headProgress = ease(linear)
  const cardsProgress = ease(Math.min(1, Math.max(0, (linear - 0.08) / 0.92)))
  const shift = Math.min(scrolled, distance)
  head.style.opacity = String(1 - headProgress)
  head.style.transform = `translate3d(0, ${shift.toFixed(1)}px, 0)`
  cards.style.opacity = String(1 - cardsProgress)
  cards.style.transform = `translate3d(0, ${shift.toFixed(1)}px, 0)`
}

function scheduleCoursehomeFade() {
  if (coursehomeFadeFrame) {
    return
  }
  coursehomeFadeFrame = requestAnimationFrame(() => {
    coursehomeFadeFrame = 0
    updateCoursehomeFade()
  })
}

watch([page, courseDetail, courseTab], scheduleCoursehomeFade, {flush: 'post'})

function stopQrTimer() {
  if (qrTimer) {
    clearInterval(qrTimer)
    qrTimer = null
  }
}

function startQrTimer() {
  stopQrTimer()
  qrTimer = setInterval(() => {
    if (qrRemain.value > 0) {
      qrRemain.value -= 1
    }
    if (qrRemain.value <= 0) {
      stopQrTimer()
    }
  }, 1000)
}

async function refreshSession() {
  try {
    session.value = await SessionStatus()
  } catch (e) {
    setNotice('读取登录状态失败: ' + friendly(e), 'error')
  }
}

async function refreshDebug() {
  if (!mode.value.debug) {
    return
  }
  debugBusy.value = true
  try {
    debugInfo.value = await DebugInfo()
    debugLog.value = (await DebugLog()) || []
  } catch (e) {
    debugInfo.value = null
  } finally {
    debugBusy.value = false
  }
}

async function clearDebug() {
  try {
    await ClearDebugLog()
  } catch (e) {
    // 清空失败时保留原日志，刷新后会显示实际状态
  }
  debugLog.value = []
  refreshDebug()
}

async function openDebugLog() {
  try {
    await OpenDebugLog()
  } catch (e) {
    setNotice(friendly(e), 'error')
  }
}

async function generateQr() {
  if (qrBusy.value) {
    return
  }
  qrBusy.value = true
  setNotice('')
  stopQrTimer()
  qrMessage.value = ''
  try {
    const res = await StartQRLogin()
    qrImage.value = res.image
    qrRemain.value = Math.max(0, res.expiresAt - Math.floor(Date.now() / 1000))
    startQrTimer()
  } catch (e) {
    qrMessage.value = friendly(e)
    setNotice('二维码获取失败: ' + friendly(e), 'error')
  } finally {
    qrBusy.value = false
    refreshDebug()
  }
}

async function cancelQr() {
  try {
    await CancelQRLogin()
  } catch (e) {
    setNotice('取消扫码失败: ' + friendly(e), 'error')
  }
  stopQrTimer()
  qrRemain.value = 0
  qrImage.value = ''
  qrMessage.value = ''
}

function onQrEvent(ev) {
  qrMessage.value = ev.message
  if (ev.state === 'scanned') {
    qrRemain.value = 0
    stopQrTimer()
  }
  if (ev.state === 'success') {
    stopQrTimer()
    qrImage.value = ''
    handleLoginSuccess('登录成功，登录态已保存')
  }
  if (ev.state === 'expired') {
    stopQrTimer()
    setNotice('二维码已失效，点“重新生成二维码”再试一次', 'error')
  }
  if (ev.state === 'error') {
    stopQrTimer()
    setNotice('扫码登录失败: ' + ev.message, 'error')
  }
  if (ev.state === 'cancelled') {
    stopQrTimer()
  }
  refreshDebug()
}

async function handleLoginSuccess(message) {
  justLoggedIn.value = true
  view.value = 'success'
  setNotice(message, 'ok')
  await refreshSession()
  await loadCourseDetail()
  refreshDebug()
}

async function refreshCaptcha() {
  captchaBusy.value = true
  try {
    const res = await StartCaptcha()
    captchaImage.value = res.image
    captchaCode.value = res.suggestedCode || ''
    captchaHint.value = res.hint || (res.suggestedCode
      ? '自动识别建议 ' + res.suggestedCode + '，可直接登录或手动修改'
      : '')
  } catch (e) {
    captchaImage.value = ''
    captchaHint.value = ''
    setNotice('获取验证码失败: ' + friendly(e), 'error')
  } finally {
    captchaBusy.value = false
    refreshDebug()
  }
}

async function submitPassword() {
  loginBusy.value = true
  setNotice('')
  try {
    const res = await LoginPassword(username.value, password.value, captchaCode.value)
    if (res.ok) {
      password.value = ''
      captchaCode.value = ''
      captchaImage.value = ''
      await handleLoginSuccess(res.message)
    } else {
      setNotice(res.message, 'error')
      if (!res.need2fa) {
        await refreshCaptcha()
      }
    }
  } catch (e) {
    setNotice(friendly(e), 'error')
    await refreshCaptcha()
  } finally {
    loginBusy.value = false
  }
}

async function checkSession() {
  checkBusy.value = true
  checkMessage.value = ''
  try {
    const res = await CheckSession()
    checkMessage.value = res.message
  } catch (e) {
    checkMessage.value = friendly(e)
  } finally {
    checkBusy.value = false
    refreshDebug()
  }
}

async function doLogout(all) {
  closeMenu()
  if (all && !window.confirm('退出登录并删除本机保存的账号凭据？下次需要重新输入账号密码。')) {
    return
  }
  if (playRunning.value) {
    try {
      await CancelPlay()
    } catch (e) {
      // 停止播放失败时继续退出，后台会在下一次上报后自行结束
    }
  }
  playRunning.value = false
  playState.value = emptyPlayState()
  try {
    const res = await Logout(!!all)
    view.value = 'login'
    justLoggedIn.value = false
    page.value = 'home'
    qrImage.value = ''
    qrMessage.value = ''
    checkMessage.value = ''
    courseList.value = []
    courseDetail.value = null
    courseError.value = ''
    courseTab.value = 'video'
    videoFilter.value = 'all'
    docFilter.value = 'all'
    quizOverview.value = null
    quizSelected.value = ''
    quizQuestions.value = []
    quizSettings.value = null
    quizBank.value = null
    quizSolving.value = false
    quizSubmitting.value = false
    quizSolveState.value = emptyQuizSolveState()
    quizPeekItems.value = []
    quizPeekOpen.value = false
    setNotice(res.message, 'ok')
    await refreshSession()
  } catch (e) {
    setNotice('退出登录失败: ' + friendly(e), 'error')
  } finally {
    refreshDebug()
  }
}

async function forgetCredentials() {
  closeMenu()
  try {
    const info = await ForgetCredentials()
    session.value = info
    setNotice(info.message, 'ok')
  } catch (e) {
    setNotice('忘记账号失败: ' + friendly(e), 'error')
  } finally {
    refreshDebug()
  }
}

async function loadCourseDetail() {
  if (!loggedIn.value) {
    courseDetail.value = null
    return
  }
  try {
    courseDetail.value = await GetCourseDetail()
  } catch (e) {
    await handleInlineError(e, (message) => { courseError.value = message })
  }
}

async function loadCourses(refresh) {
  if (!loggedIn.value || courseLoading.value) {
    return
  }
  courseLoading.value = true
  courseError.value = ''
  cloudLoading.value = '正在从云端获取课程列表…'
  try {
    courseList.value = await ListCourses(refresh !== false)
  } catch (e) {
    await handleInlineError(e, (message) => { courseError.value = message })
  } finally {
    courseLoading.value = false
    cloudLoading.value = ''
    refreshDebug()
  }
}

// 课程页右上角的“刷新”：播放中不刷新，点了只提示；页面自动加载仍走 loadCourses
function refreshCourseList() {
  if (playRunning.value) {
    sayPlaying()
    return
  }
  loadCourses(true)
}

async function selectCourse(index) {
  if (courseBusy.value) {
    return
  }
  courseBusy.value = true
  courseError.value = ''
  cloudLoading.value = '正在从云端打开课程…'
  try {
    const detail = await SelectCourse(index)
    courseDetail.value = detail
    playState.value = emptyPlayState()
    courseTab.value = 'video'
    videoFilter.value = 'all'
    docFilter.value = 'all'
    quizOverview.value = null
    quizSelected.value = ''
    quizQuestions.value = []
    quizAutoFetched.value = false
    quizAutoScanned.value = false
    failedVideos.value = new Set()
    await refreshSession()
  } catch (e) {
    await handleInlineError(e, (message) => { courseError.value = message })
  } finally {
    courseBusy.value = false
    cloudLoading.value = ''
    refreshDebug()
  }
}

async function refreshCourse() {
  if (playRunning.value) {
    sayPlaying()
    return
  }
  if (courseBusy.value) {
    return
  }
  courseBusy.value = true
  courseError.value = ''
  cloudLoading.value = '正在从云端抓取课程结构…'
  try {
    courseDetail.value = await RefreshCourse()
    courseTab.value = 'video'
    videoFilter.value = 'all'
    docFilter.value = 'all'
    quizOverview.value = null
    quizAutoFetched.value = false
    quizAutoScanned.value = false
    failedVideos.value = new Set()
    setNotice('课程结构已更新', 'ok')
  } catch (e) {
    await handleInlineError(e, (message) => { courseError.value = message })
  } finally {
    courseBusy.value = false
    cloudLoading.value = ''
    refreshDebug()
  }
}

function isCurrent(course) {
  return currentCourseID.value && course.courseId === currentCourseID.value
}

async function startPlay(objectID) {
  if (playRunning.value) {
    sayPlaying()
    return
  }
  setNotice('')
  playState.value = emptyPlayState()
  const limit = objectID ? 0 : Number(playLimit.value) || 0
  const speed = clampNumber(playSpeed.value, 1, 25, 1)
  const tick = clampNumber(playTick.value, 1, 30, 10)
  const limitedCount = Number((courseDetail.value && courseDetail.value.summary && courseDetail.value.summary.speedLimited) || 0)
  try {
    // 限速课默认也按设置的倍速播（不再提供开关）
    const res = await StartPlay(objectID || '', speed, limit, tick, true)
    playRunning.value = true
    if (speed > 1 && limitedCount > 0) {
      setNotice('已按 ' + speed + ' 倍播放，含 ' + limitedCount + ' 个限速点；未通过就降速重试', 'info')
    }
    if (!playState.value.state) {
      playState.value = {
        state: 'starting',
        message: res.message || '开始播放',
        objectId: objectID || '',
        title: '',
        index: 0,
        total: res.total || 0,
        percent: 0,
        played: 0,
        duration: 0,
        speedLimited: false,
        ok: 0,
        fail: 0,
        skipped: 0,
      }
    }
  } catch (e) {
    playRunning.value = false
    await handleActionError(e)
  }
}

function openPlayback() {
  if (hasCourse.value) {
    showPage('coursehome')
    selectCourseTab('video')
    window.scrollTo(0, 0)
  }
}

async function notifyPlayback(message) {
  try {
    let available = await IsNotificationAvailable()
    if (!available) {
      await RequestNotificationAuthorization()
      available = await IsNotificationAvailable()
    }
    if (!available || !message) {
      return
    }
    await SendNotification({
      id: 'hsc-play-' + Date.now(),
      title: 'hsc 课程助手',
      body: message,
    })
  } catch (e) {
    // 系统通知不可用时静默，不影响播放结果
  }
}

async function stopPlay() {
  try {
    await CancelPlay()
  } catch (e) {
    setNotice(friendly(e), 'error')
  }
}

function onPlayEvent(ev) {
  if (!ev) {
    return
  }
  playState.value = ev
  if (ev.state === 'starting' || ev.state === 'video' || ev.state === 'progress') {
    playRunning.value = true
  }
  if (ev.state === 'passed' || ev.state === 'skipped' || ev.state === 'failed') {
    loadCourseDetail()
  }
  if (ev.state === 'failed') {
    markVideoFailed(ev.objectId)
    setNotice('未通过：' + (ev.title || '这个任务点') + '；标记为失败，可以稍微降低倍速再试', 'error')
  }
  if (ev.state === 'passed') {
    clearVideoFailed(ev.objectId)
  }
  if (ev.state === 'finished' || ev.state === 'cancelled') {
    playRunning.value = false
    loadCourseDetail()
    if (ev.state === 'finished') {
      const text = finishedMessage(ev)
      setNotice(text, ev.fail > 0 ? 'info' : 'ok')
      notifyPlayback(text)
    } else if (ev.message) {
      setNotice(ev.message, 'info')
    }
  }
  refreshDebug()
}

// 播放一轮结束后的提示：成功给一句鼓励，有失败就说明失败数量并提示重试
function finishedMessage(ev) {
  if (ev.fail > 0) {
    return '这一轮通过 ' + ev.ok + ' 个，' + ev.fail + ' 个没通过；已标成“失败”，可以重试一次'
  }
  if (ev.ok > 0) {
    return '全部通过：这一轮完成 ' + ev.ok + ' 个任务点，收工'
  }
  if (ev.skipped > 0) {
    return '这轮都是已经通过的缓存，没有要补的'
  }
  return ev.message || '播放结束'
}

async function scanDocs() {
  if (playRunning.value) {
    sayPlaying()
    return
  }
  if (docBusy.value) {
    return
  }
  docBusy.value = true
  setNotice('')
  cloudLoading.value = '正在从云端扫描文档任务…'
  try {
    courseDetail.value = await ScanCourse()
    setNotice('文档任务扫描完成', 'ok')
  } catch (e) {
    await handleActionError(e)
  } finally {
    docBusy.value = false
    cloudLoading.value = ''
    refreshDebug()
  }
}

// 完成未完成文档：批量提交给平台，回来后按完成前后的未完成数量给结果，别把“一个都没完成”说成成功
async function completeDocs() {
  if (playRunning.value) {
    sayPlaying()
    return
  }
  if (docBusy.value) {
    return
  }
  docBusy.value = true
  setNotice('')
  cloudLoading.value = '正在从云端完成未完成文档…'
  try {
    const res = await CompleteDocs()
    courseDetail.value = res.detail
    // 结果按服务器确认的数量给（平台有时不回写卡片状态，不能只看列表数量）；
    // 列表还没刷新时用 info，提示保留到下次操作，方便用户看到“稍后再扫”。
    let kind = 'ok'
    if (res.failed > 0 && res.done === 0) {
      kind = 'error'
    } else if (res.done > 0 && res.pending > 0) {
      kind = 'info'
    }
    setNotice(res.message || '文档任务已处理', kind)
  } catch (e) {
    await handleActionError(e)
  } finally {
    docBusy.value = false
    cloudLoading.value = ''
    refreshDebug()
  }
}

async function loadQuizOverview() {
  quizBusy.value = true
  quizError.value = ''
  try {
    quizOverview.value = await GetQuizOverview()
    if (quizSelected.value && !quizItems.value.some((q) => q.workId === quizSelected.value)) {
      quizSelected.value = ''
      quizQuestions.value = []
    }
  } catch (e) {
    await handleInlineError(e, (message) => { quizError.value = message })
  } finally {
    quizBusy.value = false
    refreshDebug()
  }
}

// auto 为 true 时是进块自动扫描：不重复显示成功提示，失败仍写 quizError
async function scanForQuiz(auto) {
  if (playRunning.value && !auto) {
    sayPlaying()
    return
  }
  if (quizBusy.value) {
    return
  }
  quizBusy.value = true
  quizError.value = ''
  cloudLoading.value = '正在从云端扫描课程附件…'
  try {
    courseDetail.value = await ScanCourse()
    quizOverview.value = await GetQuizOverview()
    if (auto) {
      setNotice('课程附件已自动扫描', 'ok')
    } else {
      setNotice('课程附件扫描完成', 'ok')
    }
  } catch (e) {
    await handleInlineError(e, (message) => { quizError.value = message })
  } finally {
    quizBusy.value = false
    cloudLoading.value = ''
    refreshDebug()
  }
}

// 刷新：从云端抓未提交的作业/测验题目，写缓存后重读列表；本地没缓存时也会走这里
async function refreshQuizzes() {
  if (playRunning.value) {
    sayPlaying()
    return
  }
  if (quizBusy.value) {
    return
  }
  quizAutoFetched.value = true
  quizBusy.value = true
  quizError.value = ''
  cloudLoading.value = '正在从云端获取题目…'
  try {
    quizOverview.value = await DumpQuizzes()
    setNotice('题目已从云端更新', 'ok')
  } catch (e) {
    await handleInlineError(e, (message) => { quizError.value = message })
  } finally {
    quizBusy.value = false
    cloudLoading.value = ''
    refreshDebug()
  }
}

function emptyQuizSolveState() {
  return {state: '', message: '', index: 0, total: 0, question: '', answered: 0, cached: 0, failed: 0}
}

async function loadQuizSettings() {
  if (quizSettingsBusy.value) {
    return
  }
  quizSettingsBusy.value = true
  quizSettingsError.value = ''
  try {
    const data = await GetQuizSettings()
    data.types = Array.isArray(data.types) ? data.types : []
    data.apiKey = ''
    quizSettings.value = data
  } catch (e) {
    await handleInlineError(e, (message) => { quizSettingsError.value = message })
  } finally {
    quizSettingsBusy.value = false
    refreshDebug()
  }
}

async function saveQuizSettings() {
  if (!quizSettings.value || quizSettingsBusy.value) {
    return
  }
  quizSettingsBusy.value = true
  quizSettingsError.value = ''
  try {
    const saved = await SaveQuizSettings({
      ...quizSettings.value,
      apiKey: (quizSettings.value.apiKey || '').trim(),
    })
    saved.types = Array.isArray(saved.types) ? saved.types : []
    saved.apiKey = ''
    quizSettings.value = saved
    setNotice(saved.message || '设置已保存', 'ok')
    await loadQuizBank()
  } catch (e) {
    await handleInlineError(e, (message) => { quizSettingsError.value = message })
  } finally {
    quizSettingsBusy.value = false
    refreshDebug()
  }
}

function toggleQuizType(value) {
  if (!quizSettings.value) {
    return
  }
  const current = Array.isArray(quizSettings.value.types) ? quizSettings.value.types : []
  quizSettings.value.types = current.includes(value)
    ? current.filter((item) => item !== value)
    : [...current, value]
}

async function loadQuizBank() {
  try {
    quizBank.value = await GetQuizBank()
  } catch (e) {
    quizBank.value = null
  }
}

async function clearQuizBank() {
  if (!window.confirm('清空本机题库缓存？自动答题会重新调用接口，已答题目不受影响。')) {
    return
  }
  try {
    quizBank.value = await ClearQuizBank()
    setNotice(quizBank.value.message, 'ok')
  } catch (e) {
    await handleActionError(e, '清空题库失败')
  }
}

// 设置页里给小白看的申请入口：用系统浏览器打开 DeepSeek 控制台
function openDeepSeekKeys() {
  BrowserOpenURL('https://platform.deepseek.com/api_keys')
}

// 自动答题前先看病设置：没启用或没存 Key 就直接去设置页，不弹确认
async function ensureQuizLLMReady() {
  if (!quizSettings.value) {
    await loadQuizSettings()
  }
  const settings = quizSettings.value
  const masked = settings && settings.apiKeyMasked ? settings.apiKeyMasked : ''
  if (!settings || !settings.enabled || !masked || masked === '(未填写)') {
    // showPage 会清空提示，所以先跳页再写消息条
    showPage('settings')
    setNotice('先在设置里启用自动答题，并填入 DeepSeek 的 API Key', 'info')
    return false
  }
  return true
}

async function solveQuizzes() {
  if (playRunning.value) {
    sayPlaying()
    return
  }
  if (quizSolving.value) {
    return
  }
  if (!(await ensureQuizLLMReady())) {
    return
  }
  const autoSubmit = quizSettings.value && quizSettings.value.autoSubmit
  const prompt = autoSubmit
    ? '开始自动答题？会把题目发给 AI 模型（DeepSeek）作答；已开启自动交卷，答完会直接提交、不能再改。答案只保存在本机。'
    : '开始自动答题？会把题目发给 AI 模型（DeepSeek）作答，答案只保存在本机；题库里已有的题不会重复调用。'
  if (!window.confirm(prompt)) {
    return
  }
  quizSolving.value = true
  quizSolveState.value = emptyQuizSolveState()
  quizError.value = ''
  setNotice('')
  try {
    const res = await SolveQuizzes(-1)
    setNotice(res.message, res.failed ? 'info' : 'ok')
    await loadQuizOverview()
    await loadQuizBank()
  } catch (e) {
    await handleActionError(e, '自动答题失败')
  } finally {
    quizSolving.value = false
  }
}

function onQuizEvent(ev) {
  if (!ev) {
    return
  }
  quizSolveState.value = {...quizSolveState.value, ...ev}
  if (ev.state === 'done' || ev.state === 'error') {
    quizSolving.value = false
    if (ev.message) {
      setNotice(ev.message, ev.state === 'error' ? 'error' : 'ok')
    }
    if (ev.state === 'done') {
      loadQuizOverview()
      loadQuizBank()
    }
  }
}

async function submitQuizzes() {
  if (playRunning.value) {
    sayPlaying()
    return
  }
  if (quizSubmitting.value) {
    return
  }
  if (!window.confirm('确认交卷？交卷后答案不能再改，平台可能直接批阅。')) {
    return
  }
  quizSubmitting.value = true
  quizError.value = ''
  try {
    const res = await SubmitQuizzes(-1)
    setNotice(res.message, res.failed ? 'error' : 'ok')
    await loadQuizOverview()
  } catch (e) {
    await handleActionError(e, '交卷失败')
  } finally {
    quizSubmitting.value = false
    refreshDebug()
  }
}

async function toggleQuizPeek() {
  if (quizPeekOpen.value) {
    quizPeekOpen.value = false
    return
  }
  quizPeekOpen.value = true
  quizPeekBusy.value = true
  quizError.value = ''
  cloudLoading.value = '正在从云端读取批阅结果…'
  try {
    quizPeekItems.value = await PeekQuizzes(-1)
  } catch (e) {
    quizPeekOpen.value = false
    await handleInlineError(e, (message) => { quizError.value = message })
  } finally {
    quizPeekBusy.value = false
    cloudLoading.value = ''
  }
}

async function openQuiz(item) {
  if (!item || !item.fetched) {
    return
  }
  quizSelected.value = item.workId
  quizQuestions.value = []
  quizError.value = ''
  quizBusy.value = true
  try {
    const detail = await GetQuizQuestions(item.workId)
    quizQuestions.value = detail.questions || []
  } catch (e) {
    await handleInlineError(e, (message) => { quizError.value = message })
  } finally {
    quizBusy.value = false
    refreshDebug()
  }
}

function backToQuizList() {
  quizSelected.value = ''
  quizQuestions.value = []
  quizError.value = ''
}

function formatDuration(seconds) {
  const value = Number(seconds) || 0
  if (value <= 0) {
    return '时长未知'
  }
  const hours = Math.floor(value / 3600)
  const minutes = Math.round((value % 3600) / 60)
  if (hours > 0) {
    return hours + ' 小时 ' + minutes + ' 分钟'
  }
  return minutes + ' 分钟'
}

function formatClock(seconds) {
  const total = Math.max(0, Math.round(Number(seconds) || 0))
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const secs = total % 60
  if (hours > 0) {
    return hours + ':' + String(minutes).padStart(2, '0') + ':' + String(secs).padStart(2, '0')
  }
  return minutes + ':' + String(secs).padStart(2, '0')
}

async function bootstrap() {
  title.value = TITLES[Math.floor(Math.random() * TITLES.length)]
  try {
    mode.value = await AppMode()
  } catch (e) {
    // AppMode 不可用时按 release 处理，保证界面能显示
  }
  await refreshSession()
  if (loggedIn.value) {
    loadCourseDetail()
    if (session.value && session.value.hasTarget) {
      // 启动时顺便校验一次登录态：被平台踢掉时自动清 cookie 并回登录页
      repairInvalidSession()
    }
  }
  if (mode.value.debug) {
    refreshDebug()
  }
}

let offQr = null
let offPlay = null
let offQuiz = null

onMounted(() => {
  bootstrap()
  offQr = EventsOn('login:qr', onQrEvent)
  offPlay = EventsOn('play:progress', onPlayEvent)
  offQuiz = EventsOn('quiz:progress', onQuizEvent)
  document.addEventListener('click', onDocumentClick)
  window.addEventListener('keydown', onKeydown)
  window.addEventListener('scroll', scheduleCoursehomeFade, {passive: true})
  window.addEventListener('resize', scheduleCoursehomeFade)
})

onUnmounted(() => {
  clearNoticeTimer()
  stopQrTimer()
  if (typeof offQr === 'function') {
    offQr()
  } else {
    EventsOff('login:qr')
  }
  if (typeof offPlay === 'function') {
    offPlay()
  } else {
    EventsOff('play:progress')
  }
  if (typeof offQuiz === 'function') {
    offQuiz()
  } else {
    EventsOff('quiz:progress')
  }
  document.removeEventListener('click', onDocumentClick)
  window.removeEventListener('keydown', onKeydown)
  window.removeEventListener('scroll', scheduleCoursehomeFade)
  window.removeEventListener('resize', scheduleCoursehomeFade)
  if (coursehomeFadeFrame) {
    cancelAnimationFrame(coursehomeFadeFrame)
    coursehomeFadeFrame = 0
  }
})
</script>

<template>
  <div class="app">
    <header class="topbar">
      <div class="bar-left">
        <button v-if="page !== 'home'" class="back-btn" @click="goBack()">返回</button>
        <button class="brand" @click="openAbout" aria-label="关于 hsc 课程助手">
          <span class="wordmark">hsc</span>
          <span class="dim">课程助手</span>
        </button>
      </div>

      <div class="bar-right">
        <button v-if="playRunning" class="playback-chip" @click="openPlayback">
          正在播放<span v-if="playbackPercent"> {{ playbackPercent }}%</span>
        </button>
        <template v-if="loggedIn">
          <div class="user-menu" @click.stop>
            <button class="account-btn" @click="toggleMenu">{{ accountLabel }}</button>
            <div v-if="menuOpen" class="menu" role="menu">
              <div class="menu-row"><span>账号</span><span class="v">{{ account || (loggedIn ? '已登录，未保存账号' : '未保存') }}</span></div>
              <div class="menu-row"><span>登录状态</span><span class="v">{{ loggedIn ? '有效' : '无' }}</span></div>
              <div class="menu-row"><span>记住账号</span><span class="v">{{ session && session.credSaved ? '已保存' : '未保存' }}</span></div>
              <div class="menu-row"><span>课程</span><span class="v">{{ session && session.hasTarget ? '已配置' : '未配置' }}</span></div>
              <button class="menu-action" @click="checkLoginState">检查登录态</button>
              <button class="menu-action" @click="openHelp">帮助</button>
              <button v-if="session && session.credSaved" class="menu-action" @click="forgetCredentials">忘记保存的账号</button>
              <button class="menu-danger" @click="doLogout(false)">退出登录</button>
              <button v-if="session && session.credSaved" class="menu-danger" @click="doLogout(true)">退出并清除凭据</button>
            </div>
          </div>
          <span v-if="mode.debug" class="debug-tag">debug</span>
        </template>
        <button v-else-if="page !== 'login'" class="login-btn" @click="showPage('login')">登录</button>
      </div>
    </header>

    <main v-if="page === 'home'" class="home">
      <h1 class="home-title">{{ title }}</h1>

      <div class="home-stage">
        <span class="stage-halo" aria-hidden="true"></span>

        <button class="course-hero" @click="openCourse">
          <span class="hero-title">开始刷课吧</span>
          <span class="hero-arrow" aria-hidden="true">
            <svg viewBox="0 0 24 24"><path d="M5 12h13M12 6l6 6-6 6"/></svg>
          </span>
        </button>

        <span class="hero-chip chip-video" aria-hidden="true">
          <svg viewBox="0 0 24 24"><path d="M4 6.8A2.8 2.8 0 0 1 6.8 4h10.4A2.8 2.8 0 0 1 20 6.8v10.4a2.8 2.8 0 0 1-2.8 2.8H6.8A2.8 2.8 0 0 1 4 17.2z"/><path d="m10 9 5 3-5 3z"/></svg>
          视频任务
        </span>
        <span class="hero-chip chip-doc" aria-hidden="true">
          <svg viewBox="0 0 24 24"><path d="M7 3h7l4 4v14H7z"/><path d="M14 3v4h4M10 12h5M10 16h5"/></svg>
          文档任务
        </span>
        <span class="hero-chip chip-quiz" aria-hidden="true">
          <svg viewBox="0 0 24 24"><path d="M9 5h10M9 12h10M9 19h10"/><path d="m4 5 1.2 1.2L7 4.4M4 12l1.2 1.2L7 11.4M4 19l1.2 1.2L7 18.4"/></svg>
          作业与测验
        </span>
      </div>

      <div v-if="mode.debug" class="home-debug-row">
        <button class="ghost-link" @click="showPage('debug')">调试信息</button>
      </div>

      <section v-if="mode.debug" class="debug-info" aria-label="调试信息">
        <div class="row">
          <span class="k">版本</span>
          <span class="v">{{ mode.version }} · {{ mode.mode }}</span>
        </div>
        <div class="row">
          <span class="k">state 目录</span>
          <span class="v mono">{{ (debugInfo && debugInfo.stateDir) || (session && session.stateDir) || '—' }}</span>
        </div>
        <div class="row">
          <span class="k">最近调用</span>
          <span class="v">{{ lastDebugMessage }}</span>
        </div>
      </section>
    </main>

    <main v-else class="subpage" :class="{'coursehome-page': page === 'coursehome'}">
      <header class="page-head" :class="{'page-head-center': page === 'coursehome'}">
        <div class="page-head-text">
          <h1 class="page-title">{{ pageTitle }}</h1>
          <p v-if="pageLead" class="page-sub">{{ pageLead }}</p>
          <button
            v-if="page === 'coursehome' && hasCourse"
            class="ghost-link head-refresh"
            :class="{'play-blocked': playRunning}"
            :aria-disabled="playRunning || null"
            :title="playRunning ? '正在播放中，先停止播放' : ''"
            @click="refreshCourse"
            :disabled="courseBusy"
          >{{ courseBusy ? '抓取中' : '重新抓取课程结构' }}</button>
        </div>
        <div v-if="page === 'course'" class="actions">
          <button
            :class="{'play-blocked': playRunning}"
            :aria-disabled="playRunning || null"
            :title="playRunning ? '正在播放中，先停止播放' : ''"
            @click="refreshCourseList"
            :disabled="courseLoading || courseBusy"
          >
            {{ courseLoading ? '刷新中' : '刷新' }}
          </button>
        </div>
        <div v-else-if="page === 'debug' && mode.debug" class="actions">
          <button @click="refreshDebug" :disabled="debugBusy">{{ debugBusy ? '刷新中' : '刷新' }}</button>
          <button @click="openDebugLog">打开日志</button>
          <button @click="clearDebug">清空日志</button>
        </div>
      </header>

      <div v-if="page === 'coursehome' && hasCourse" class="entry-list">
        <button class="entry-row" :class="{active: courseTab === 'video'}" @click="selectCourseTab('video')">
          <span class="entry-main">
            <span class="entry-name">视频任务</span>
            <span class="hint">已完成 {{ courseDetail.summary.videosPassed }}/{{ courseDetail.summary.videos }} 个</span>
          </span>
        </button>
        <button class="entry-row" :class="{active: courseTab === 'doc'}" @click="selectCourseTab('doc')">
          <span class="entry-main">
            <span class="entry-name">文档任务</span>
            <span class="hint">{{ docScanned ? '已完成 ' + docPassed + '/' + docs.length + ' 个' : '未扫描' }}</span>
          </span>
        </button>
        <button class="entry-row" :class="{active: courseTab === 'quiz'}" @click="selectCourseTab('quiz')">
          <span class="entry-main">
            <span class="entry-name">作业与测验</span>
            <span class="hint">{{ courseDetail.summary.works }} 个任务点</span>
          </span>
        </button>
      </div>

      <section class="panel">
        <template v-if="page === 'login'">
          <template v-if="view === 'success'">
            <dl class="facts">
              <div><dt>登录状态</dt><dd>{{ loggedIn ? '有效' : '无' }}</dd></div>
              <div><dt>记住账号</dt><dd>{{ session && session.credSaved ? '已保存' : '未保存' }}</dd></div>
              <div><dt>课程</dt><dd>{{ session && session.hasTarget ? '已配置' : '未配置' }}</dd></div>
              <template v-if="mode.debug">
                <div><dt>state 目录</dt><dd class="mono">{{ session ? session.stateDir : '' }}</dd></div>
                <div><dt>cookie 数量</dt><dd>{{ session ? session.cookieCount : 0 }}</dd></div>
              </template>
            </dl>
            <div class="actions">
              <button class="primary" @click="showPage('home')">回首页</button>
              <button @click="checkSession" :disabled="checkBusy">{{ checkBusy ? '检查中' : '检查会话' }}</button>
            </div>
            <p v-if="checkMessage" class="hint">{{ checkMessage }}</p>
          </template>

          <template v-else>
            <div class="tabs">
              <button :class="{active: tab === 'qr'}" @click="tab = 'qr'">扫码登录</button>
              <button :class="{active: tab === 'pw'}" @click="tab = 'pw'; refreshCaptcha()">账号密码登录</button>
            </div>

            <div v-if="tab === 'qr'" class="pane">
              <div v-if="qrImage" class="qr-wrap">
                <img class="qr" :src="qrImage" alt="登录二维码"/>
                <p class="hint">
                  {{ qrMessage || '等待扫码' }}<span v-if="qrRemain > 0">，剩余 {{ qrRemain }} 秒</span>
                </p>
              </div>
              <p v-else class="qr-empty">{{ qrBusy ? '正在获取二维码' : '准备二维码' }}</p>
              <div class="actions">
                <button class="primary" @click="generateQr" :disabled="qrBusy">
                  {{ qrImage ? '重新生成二维码' : '生成二维码' }}
                </button>
                <button v-if="qrImage" @click="cancelQr">取消</button>
              </div>
            </div>

            <div v-else class="pane">
              <div class="field">
                <label for="username">学号 / 账号</label>
                <input id="username" v-model="username" type="text" autocomplete="username" placeholder="输入学号或账号"/>
              </div>
              <div class="field">
                <label for="password">密码</label>
                <input id="password" v-model="password" type="password" autocomplete="current-password" placeholder="输入密码"/>
              </div>
              <div class="captcha-row">
                <div class="captcha-box">
                  <img v-if="captchaImage" class="captcha" :src="captchaImage" alt="验证码" title="点击换一张" @click="refreshCaptcha"/>
                  <span v-else class="hint">验证码未获取</span>
                </div>
                <div class="captcha-side">
                  <div class="captcha-actions">
                    <button @click="refreshCaptcha" :disabled="captchaBusy">{{ captchaBusy ? '获取中' : '换一张' }}</button>
                    <input v-model="captchaCode" class="captcha-input" type="text" maxlength="4" placeholder="4 位验证码"/>
                  </div>
                  <p class="hint">{{ captchaHint }}</p>
                </div>
              </div>
              <div class="actions">
                <button class="primary" @click="submitPassword" :disabled="loginBusy">{{ loginBusy ? '登录中' : '登录' }}</button>
              </div>
            </div>
          </template>
        </template>

        <template v-else-if="page === 'course'">
          <p v-if="courseError" class="inline-error">{{ courseError }}</p>
          <p v-if="!courseList.length && !courseLoading" class="hint">还没有课程列表，点右上角“刷新”。</p>

          <div v-if="courseList.length" class="course-list">
            <button
              v-for="item in courseList"
              :key="item.courseId"
              class="course-row"
              :class="{'play-blocked': playRunning}"
              :aria-disabled="playRunning || null"
              :title="playRunning ? '正在播放中，先停止播放' : ''"
              :disabled="courseBusy || courseLoading"
              @click="openCourseRow(item)"
            >
              <span class="course-index">{{ item.index }}</span>
              <span class="course-name">{{ item.name || '未命名课程' }}</span>
              <span class="course-teacher">{{ item.teacher }}</span>
              <svg class="course-chevron" viewBox="0 0 24 24" aria-hidden="true"><path d="m9 6 6 6-6 6"/></svg>
            </button>
          </div>

          <p class="hint">进入课程后可以看视频任务、文档任务和作业与测验；课程结构在课程主页里。</p>

          <details class="course-link">
            <summary>用课程链接进入</summary>
            <div class="course-link-row">
              <input v-model="courseURL" type="url" placeholder="粘贴课程页地址（含 courseid 与 clazzid）" @keyup.enter="openCourseByURL"/>
              <button
                :class="{'play-blocked': playRunning}"
                :aria-disabled="playRunning || null"
                :title="playRunning ? '正在播放中，先停止播放' : ''"
                @click="openCourseByURL"
                :disabled="courseURLBusy || !courseURL.trim()"
              >
                {{ courseURLBusy ? '抓取中' : '按链接进入' }}
              </button>
            </div>
            <p class="hint">链接从浏览器课程页地址栏复制；会覆盖当前课程配置并抓取结构。</p>
          </details>
        </template>

        <template v-else-if="page === 'coursehome'">
          <p v-if="courseError" class="inline-error">{{ courseError }}</p>

          <template v-if="hasCourse">
            <p v-if="courseDetail.stale" class="warn-text">缓存属于另一门课，点“重新抓取课程结构”刷新。</p>

            <div class="block-head">
              <div class="block-head-text">
                <h2 class="block-title">{{ courseTabTitle }}</h2>
                <span v-if="playRunning" class="play-hint">播放中，其他操作先停止播放</span>
              </div>
              <div v-if="courseTab === 'video'" class="actions">
                <label class="play-speed">
                  数量
                  <select v-model.number="playLimit" :disabled="playRunning" :title="playRunning ? '正在播放中，先停止播放' : ''">
                    <option :value="0">全部</option>
                    <option :value="1">1 个</option>
                    <option :value="3">3 个</option>
                    <option :value="5">5 个</option>
                    <option :value="10">10 个</option>
                  </select>
                </label>
                <span class="play-summary">{{ playSpeed }}x · {{ playTick }}s</span>
                <button
                  class="settings-btn"
                  :class="{'play-blocked': playRunning}"
                  :aria-disabled="playRunning || null"
                  :title="playRunning ? '正在播放中，先停止播放' : '播放设置：倍速与上报间隔'"
                  @click="openPlaySettings"
                >
                  <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
                  播放设置
                </button>
                <button v-if="playRunning" class="primary" @click="stopPlay">停止播放</button>
                <button v-else class="primary" @click="startPlay('')" :disabled="docBusy || !unpassedVideos.length">
                  {{ playCount ? '播放未完成 ' + playCount + ' 个' : '视频都已完成' }}
                </button>
              </div>
              <div v-else-if="courseTab === 'doc'" class="actions">
                <button
                  :class="{'play-blocked': playRunning}"
                  :aria-disabled="playRunning || null"
                  :title="playRunning ? '正在播放中，先停止播放' : ''"
                  @click="scanDocs"
                  :disabled="docBusy"
                >
                  {{ docBusy ? '处理中' : (docScanned ? '重新扫描' : '扫描文档任务') }}
                </button>
                <button
                  v-if="docScanned && docUnpassed"
                  class="primary"
                  :class="{'play-blocked': playRunning}"
                  :aria-disabled="playRunning || null"
                  :title="playRunning ? '正在播放中，先停止播放' : ''"
                  @click="completeDocs"
                  :disabled="docBusy"
                >
                  完成未完成文档 {{ docUnpassed }} 个
                </button>
              </div>
              <div v-else class="actions">
                <button
                  v-if="quizPrimaryAction !== 'none'"
                  :class="{'play-blocked': playRunning}"
                  :aria-disabled="playRunning || null"
                  :title="playRunning ? '正在播放中，先停止播放' : ''"
                  @click="quizPrimaryAction === 'scan' ? scanForQuiz(false) : refreshQuizzes()"
                  :disabled="quizBusy"
                >
                  {{ quizBusy ? '处理中' : quizPrimaryLabel }}
                </button>
                <button
                  class="primary"
                  :class="{'play-blocked': playRunning}"
                  :aria-disabled="playRunning || null"
                  :title="playRunning ? '正在播放中，先停止播放' : '把题目交给 AI 模型（DeepSeek）作答'"
                  @click="solveQuizzes"
                  :disabled="quizBusy || quizSolving || !quizReady"
                >
                  {{ quizSolving ? '答题中' : '自动答题' }}
                </button>
                <button
                  :class="{'play-blocked': playRunning}"
                  :aria-disabled="playRunning || null"
                  :title="playRunning ? '正在播放中，先停止播放' : ''"
                  @click="submitQuizzes"
                  :disabled="quizBusy || quizSubmitting || quizSolving || !quizReady"
                >
                  {{ quizSubmitting ? '交卷中' : '交卷' }}
                </button>
                <button @click="showPage('settings')">测验设置</button>
              </div>
            </div>
          </template>

          <template v-else>
            <p class="hint">还没有选择课程。</p>
            <div class="actions">
              <button @click="showPage('course')">去课程列表</button>
            </div>
          </template>
        </template>

        <template v-if="page === 'coursehome' && courseTab === 'video'">
          <template v-if="hasCourse">
            <section v-if="!chapters.length" class="task-overview">
              <div class="overview-empty">
                <p class="overview-caption">还没有课程结构</p>
                <p class="hint">点“重新抓取课程结构”，抓完这里会显示章节和视频进度。</p>
              </div>
            </section>

            <template v-else>
              <section class="task-overview" aria-label="视频完成概况">
                <div class="overview-top">
                  <div class="overview-figure">
                    <b>{{ videoStats.percent }}%</b>
                    <span class="overview-caption">{{ videoStats.passed }}/{{ videoStats.total }} 个视频已完成</span>
                  </div>
                  <div class="overview-chips">
                    <span class="stat-chip">未完成 <b>{{ videoStats.todo }}</b></span>
                    <span v-if="courseDetail.summary && courseDetail.summary.speedLimited" class="stat-chip">限 1 倍速 <b>{{ courseDetail.summary.speedLimited }}</b></span>
                  </div>
                </div>
                <div class="overview-track">
                  <div class="overview-fill" :style="{width: videoStats.percent + '%'}"></div>
                </div>

                <div v-if="playState.message" class="live-block">
                  <div class="live-head">
                    <span class="live-title">{{ playState.message }}</span>
                    <span v-if="playState.index" class="live-meta">第 {{ playState.index }}/{{ playState.total }} 个</span>
                    <span v-if="playState.speedLimited" class="live-meta">限 1 倍速</span>
                    <span v-if="playState.duration" class="live-meta">{{ formatClock(playState.played) }} / {{ formatClock(playState.duration) }}</span>
                  </div>
                  <div v-if="playRunning && playState.duration" class="play-track">
                    <div class="play-fill" :style="{width: playbackPercent + '%'}"></div>
                  </div>
                </div>
                <p v-else class="hint">点「播放未完成」一次播完，也可以单条播；限速点默认按 1 倍，规则见「播放设置」。</p>
              </section>

              <div class="list-head">
                <h3 class="list-title">章节任务</h3>
                <div class="filter-row" role="group" aria-label="按完成情况筛选">
                  <button :class="{active: videoFilter === 'all'}" @click="videoFilter = 'all'">全部 {{ videoStats.total }}</button>
                  <button :class="{active: videoFilter === 'todo'}" @click="videoFilter = 'todo'">未完成 {{ videoStats.todo }}</button>
                  <button :class="{active: videoFilter === 'done'}" @click="videoFilter = 'done'">已完成 {{ videoStats.passed }}</button>
                </div>
              </div>

              <div v-if="videoFilteredChapters.length" class="chapter-list">
                <details v-for="ch in videoFilteredChapters" :id="'chapter-' + ch.id" :key="ch.id" class="chapter" :open="isChapterOpen(ch)">
                  <summary>
                    <span class="chapter-name">{{ ch.title || ('章节 ' + ch.index) }}</span>
                    <span v-if="chapterVideos(ch).length" class="chapter-bar" aria-hidden="true"><i :style="{width: chapterPercent(ch) + '%'}"></i></span>
                    <span class="chapter-count">{{ chapterVideos(ch).length ? chapterPassed(ch) + '/' + chapterVideos(ch).length : '无视频' }}</span>
                    <svg class="chapter-chevron" viewBox="0 0 24 24" aria-hidden="true"><path d="m9 6 6 6-6 6"/></svg>
                  </summary>
                  <ul class="task-list">
                    <li v-for="(v, index) in chapterTaskVideos(ch)" :key="'v' + ch.id + index" :class="{playing: playRunning && playState.objectId === v.objectId}">
                      <span class="task-main">
                        <span class="task-title">{{ v.title || '未命名视频' }}</span>
                        <span class="task-meta">{{ formatDuration(v.duration) }}{{ v.speedLimited ? ' · 限 1 倍速' : '' }}</span>
                      </span>
                      <span v-if="!v.isPassed && isVideoFailed(v.objectId)" class="status-chip is-failed">失败</span>
                      <span v-else class="status-chip" :class="{'is-done': v.isPassed}">{{ v.isPassed ? '已完成' : '未完成' }}</span>
                      <button
                        v-if="!v.isPassed"
                        class="row-play"
                        :class="{'play-blocked': playRunning}"
                        :aria-disabled="playRunning || null"
                        :title="playRunning ? '正在播放中，先停止播放' : ''"
                        @click="startPlay(v.objectId)"
                        :disabled="docBusy"
                      >播放</button>
                    </li>
                    <li v-if="!chapterTaskVideos(ch).length" class="task-empty">当前筛选下本章没有任务。</li>
                  </ul>
                </details>
              </div>
              <p v-else class="hint">当前筛选下没有章节。</p>
            </template>
          </template>
        </template>

        <template v-if="page === 'coursehome' && courseTab === 'doc'">
          <template v-if="hasCourse">
            <section class="task-overview" aria-label="文档完成概况">
              <template v-if="docScanned && docs.length">
                <div class="overview-top">
                  <div class="overview-figure">
                    <b>{{ docPercent }}%</b>
                    <span class="overview-caption">{{ docPassed }}/{{ docs.length }} 个文档已完成</span>
                  </div>
                  <div class="overview-chips">
                    <span class="stat-chip">未完成 <b>{{ docUnpassed }}</b></span>
                  </div>
                </div>
                <div class="overview-track">
                  <div class="overview-fill" :style="{width: docPercent + '%'}"></div>
                </div>
              </template>
              <div v-else-if="docScanned" class="overview-empty">
                <p class="overview-caption">这门课没有文档任务点</p>
                <p class="hint">扫描结果里没有附件文档，不用处理。</p>
              </div>
              <div v-else class="overview-empty">
                <p class="overview-caption">还没有扫描文档任务</p>
                <p class="hint">点「扫描文档任务」逐章读取附件，完成后会再扫一次刷新状态。</p>
              </div>
            </section>

            <template v-if="docScanned && docs.length">
              <div class="list-head">
                <h3 class="list-title">文档清单</h3>
                <div class="filter-row" role="group" aria-label="按完成情况筛选">
                  <button :class="{active: docFilter === 'all'}" @click="docFilter = 'all'">全部 {{ docs.length }}</button>
                  <button :class="{active: docFilter === 'todo'}" @click="docFilter = 'todo'">未完成 {{ docUnpassed }}</button>
                  <button :class="{active: docFilter === 'done'}" @click="docFilter = 'done'">已完成 {{ docPassed }}</button>
                </div>
              </div>

              <ul v-if="docFilteredDocs.length" class="task-list doc-list">
                <li v-for="(d, index) in docFilteredDocs" :key="'d' + d.chapterId + index">
                  <span class="task-main">
                    <span class="task-title">{{ d.title || '未命名文档' }}</span>
                    <span v-if="docChapterLabel(d)" class="task-meta">{{ docChapterLabel(d) }}</span>
                  </span>
                  <span class="status-chip" :class="{'is-done': d.isPassed}">{{ d.isPassed ? '已完成' : '未完成' }}</span>
                </li>
              </ul>
              <p v-else class="hint">当前筛选下没有文档。</p>
            </template>
          </template>
        </template>

        <template v-if="page === 'coursehome' && courseTab === 'quiz' && hasCourse">
          <p v-if="quizError" class="inline-error">{{ quizError }}</p>
          <p class="hint">{{ quizOverview ? quizOverview.message : '正在读取作业与测验...' }}</p>
          <p class="hint ai-hint">「自动答题」是把题目交给 AI 模型（DeepSeek）作答，答案只保存在本机；交卷仍要你自己确认。</p>

          <div class="quiz-tools actions">
            <button @click="toggleQuizPeek" :disabled="quizPeekBusy">
              {{ quizPeekOpen ? '收起批阅结果' : (quizPeekBusy ? '读取中' : '批阅结果') }}
            </button>
            <span class="hint">题库 {{ quizBank ? quizBank.count : 0 }} 题</span>
          </div>

          <div v-if="quizPeekOpen" class="quiz-peek">
            <p v-if="quizPeekBusy" class="hint">正在读取批阅页，网络慢时可能要十几秒...</p>
            <ul v-else-if="quizPeekItems.length" class="quiz-peek-list">
              <li v-for="item in quizPeekItems" :key="item.index">
                <span class="course-index">{{ item.index + 1 }}</span>
                <span class="quiz-title">{{ item.title }}</span>
                <span class="hint">{{ item.status }}</span>
              </li>
            </ul>
            <p v-else class="hint">没有可查看的批阅记录。</p>
          </div>

          <div v-if="quizSolving || quizSolveState.message" class="quiz-progress">
            <div class="play-line">
              <span class="play-message">{{ quizSolveState.message || '准备答题' }}</span>
              <span v-if="quizSolveState.total" class="hint">第 {{ quizSolveState.index }}/{{ quizSolveState.total }} 题</span>
              <span class="hint">成功 {{ quizSolveState.answered }}，题库 {{ quizSolveState.cached }}，失败 {{ quizSolveState.failed }}</span>
            </div>
            <div v-if="quizSolving && quizSolveState.total" class="play-track">
              <div class="play-fill" :style="{width: quizSolvePercent + '%'}"></div>
            </div>
          </div>

          <template v-if="quizOverview && quizOverview.scanned">
            <template v-if="quizSelected">
              <div class="quiz-detail-head actions">
                <button @click="backToQuizList">返回列表</button>
                <span class="chapter-name">{{ (selectedQuizItem && selectedQuizItem.title) || '作业/测验' }}</span>
                <span v-if="quizQuestions.length" class="hint">共 {{ quizQuestions.length }} 题</span>
              </div>
              <p v-if="quizBusy && !quizQuestions.length" class="hint">正在读取题目...</p>
              <p v-else-if="!quizQuestions.length" class="hint">这份作业或测验没有可显示的题目。</p>
              <div class="quiz-questions">
                <div v-for="(q, i) in quizQuestions" :key="q.qid + '-' + i" class="quiz-question">
                  <div class="quiz-stem">
                    <span class="task-tag">{{ q.typeLabel || '题目' }}</span>
                    <span class="quiz-stem-text">{{ i + 1 }}. {{ q.stem }}</span>
                    <span v-if="q.score" class="hint">{{ q.score }} 分</span>
                  </div>
                  <ul v-if="q.options && q.options.length" class="quiz-options">
                    <li v-for="o in q.options" :key="o.value"><b>{{ o.value }}.</b> {{ o.text }}</li>
                  </ul>
                  <p v-else-if="q.blankCount" class="hint">填空题，共 {{ q.blankCount }} 空</p>
                </div>
              </div>
            </template>

            <template v-else>
              <p v-if="!quizItems.length" class="hint">这门课没有作业或测验任务点。</p>
              <div v-else class="quiz-list">
                <div v-for="item in quizItems" :key="item.workId" class="quiz-row">
                  <span class="course-index">{{ item.chapterIndex || '' }}</span>
                  <span class="quiz-title">{{ item.title }}</span>
                  <span v-if="item.fetched" class="hint">{{ item.questionCount }} 题</span>
                  <span v-else class="hint">未抓题</span>
                  <button @click="openQuiz(item)" :disabled="quizBusy || !item.fetched">
                    {{ item.fetched ? '查看题目' : '未抓取' }}
                  </button>
                </div>
              </div>
              <p class="hint">抓取题目只拉取未提交的作业与测验；已提交的会显示“未抓取”。</p>
            </template>
          </template>

          <p v-else-if="quizOverview" class="hint">还没有扫描课程附件，稍等自动扫描；也可以点工具条的“扫描附件”。</p>
        </template>

        <template v-if="page === 'settings'">
          <p v-if="quizSettingsError" class="inline-error">{{ quizSettingsError }}</p>
          <p v-if="!quizSettings" class="hint">正在读取设置...</p>

          <template v-else>
            <div class="settings-block">
              <label class="check-row">
                <input v-model="quizSettings.enabled" type="checkbox"/>
                <span>启用自动答题</span>
              </label>
              <p class="hint">「自动答题」会把题目发给 AI 模型（DeepSeek）作答，答案只保存在本机；关闭时不会调用 AI。</p>
            </div>

            <div class="settings-grid">
              <div class="field">
                <label>接口地址</label>
                <input v-model.trim="quizSettings.baseUrl" type="url" placeholder="https://api.deepseek.com/v1"/>
              </div>
              <div class="field">
                <label>API Key</label>
                <input
                  v-model="quizSettings.apiKey"
                  type="password"
                  autocomplete="off"
                  :placeholder="quizSettings.apiKeyMasked && quizSettings.apiKeyMasked !== '(未填写)' ? '留空沿用已保存的 Key（' + quizSettings.apiKeyMasked + '）' : '填写 API Key'"
                />
                <p class="field-hint">
                  没有 Key？
                  <a href="https://platform.deepseek.com/api_keys" @click.prevent="openDeepSeekKeys">到 DeepSeek 控制台申请</a>
                </p>
              </div>
              <div class="field">
                <label>模型</label>
                <input v-model.trim="quizSettings.model" type="text" placeholder="deepseek-flash"/>
                <p class="field-hint">通用用 deepseek-flash；更强的可以用 deepseek-v4-pro。旧的 deepseek-chat 已下线。</p>
              </div>
              <div class="field">
                <label>温度</label>
                <input v-model.number="quizSettings.temperature" type="number" step="0.1" min="0" max="2"/>
              </div>
              <div class="field">
                <label>max_tokens</label>
                <input v-model.number="quizSettings.maxTokens" type="number" min="1" max="128000"/>
              </div>
              <div class="field">
                <label>超时（秒）</label>
                <input v-model.number="quizSettings.timeoutSeconds" type="number" min="1" max="600"/>
              </div>
              <div class="field">
                <label>每题间隔（毫秒）</label>
                <input v-model.number="quizSettings.delayMs" type="number" min="0" max="60000"/>
              </div>
              <div class="field">
                <label>单题重试次数</label>
                <input v-model.number="quizSettings.retries" type="number" min="0" max="10"/>
              </div>
            </div>

            <div class="settings-block">
              <span class="settings-label">参与作答的题型</span>
              <div class="check-list">
                <label v-for="item in QUIZ_TYPES" :key="item.value" class="check-row compact">
                  <input type="checkbox" :checked="quizSettings.types.includes(item.value)" @change="toggleQuizType(item.value)"/>
                  <span>{{ item.label }}</span>
                </label>
              </div>
            </div>

            <div class="settings-block">
              <label class="check-row">
                <input v-model="quizSettings.autoSubmit" type="checkbox"/>
                <span>答完直接交卷</span>
              </label>
              <p v-if="quizSettings.autoSubmit" class="warn-text">开启后自动答题结束会直接提交，建议先关闭、检查答案后再手动交卷。</p>
            </div>

            <div class="actions">
              <button class="primary" @click="saveQuizSettings" :disabled="quizSettingsBusy">
                {{ quizSettingsBusy ? '保存中' : '保存设置' }}
              </button>
              <button @click="loadQuizSettings" :disabled="quizSettingsBusy">重新读取</button>
            </div>
            <p class="hint">{{ quizSettings.message }}</p>

            <h3>题库缓存</h3>
            <div class="actions">
              <span class="hint">{{ quizBank ? quizBank.message : '题库读取中' }}</span>
              <button @click="loadQuizBank" :disabled="quizSettingsBusy">刷新</button>
              <button @click="clearQuizBank" :disabled="quizSettingsBusy">清空题库</button>
            </div>
            <p class="hint">设置保存在 state/config.toml，与 hust-smartcourse 命令行共用。</p>
          </template>
        </template>

        <template v-if="page === 'play-settings'">
          <section class="play-setting">
            <div class="setting-head">
              <label for="play-speed-range">倍速</label>
              <strong>{{ playSpeed }}×</strong>
            </div>
            <input id="play-speed-range" v-model.number="playSpeed" type="range" min="1" max="25" step="0.5"/>
            <div class="preset-row">
              <button v-for="v in [1, 2, 3, 5, 10, 25]" :key="'sp' + v" :class="{active: Number(playSpeed) === v}" @click="playSpeed = v">{{ v }}×</button>
            </div>
            <p class="setting-note is-warn">限速课采用高倍数播放，平台可能不会判定通过，请手动补看最后部分，或者换低倍数重试。</p>
          </section>

          <section class="play-setting">
            <div class="setting-head">
              <label for="play-tick-range">上报间隔</label>
              <strong>{{ playTick }} 秒</strong>
            </div>
            <input id="play-tick-range" v-model.number="playTick" type="range" min="1" max="30" step="1"/>
            <p class="setting-note">只在播放时生效：间隔越短上报越频繁，总用时 = 视频时长 / 倍速。</p>
          </section>

          <footer class="play-setting-foot">
            <span class="hint">自动保存，下次播放生效。</span>
            <button class="ghost-link" @click="resetPlaySettings">恢复默认</button>
          </footer>
        </template>

        <template v-if="page === 'debug' && mode.debug">
          <div v-if="debugInfo" class="debug-grid">
            <div><span>模式</span><b>{{ debugInfo.mode }}</b></div>
            <div><span>state 目录</span><b class="mono">{{ debugInfo.stateDir }}</b></div>
            <div><span>工作目录</span><b class="mono">{{ debugInfo.cwd }}</b></div>
            <div><span>可执行文件</span><b class="mono">{{ debugInfo.exePath }}</b></div>
            <div><span>Go 版本</span><b>{{ debugInfo.goVersion }}</b></div>
            <div><span>系统</span><b>{{ debugInfo.os }}/{{ debugInfo.arch }}</b></div>
            <div><span>cookie 数量</span><b>{{ debugInfo.cookieCount }}</b></div>
            <div><span>登录态</span><b>{{ debugInfo.loggedIn ? '存在' : '无' }}</b></div>
            <div class="debug-wide"><span>日志文件</span><b class="mono">{{ debugInfo.logPath || '—' }}</b></div>
          </div>

          <h3>最近日志</h3>
          <ul class="debug-log">
            <li v-for="(entry, index) in debugLog" :key="index">
              <span class="mono">{{ entry.time }}</span>
              <em>{{ entry.kind }}</em>
              <span>{{ entry.message }}</span>
            </li>
          </ul>
          <p v-if="!debugLog.length" class="hint">暂无日志</p>

          <h3>原始会话状态</h3>
          <pre class="debug-json">{{ prettySession }}</pre>
        </template>
      </section>
    </main>

    <transition name="modal">
      <div v-if="aboutOpen" class="about-backdrop" @click.self="closeAbout">
        <div class="about-dialog" role="dialog" aria-modal="true" aria-labelledby="about-title">
          <button ref="aboutCloseRef" class="about-close" @click="closeAbout" aria-label="关闭介绍">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18"/></svg>
          </button>
          <h2 id="about-title" class="about-title">hsc 课程助手</h2>
          <p class="about-desc"><span class="nowrap">hust-smartcourse</span> 的桌面图形界面：课程、视频、文档与测验任务都能在这里完成，登录态和课程缓存与命令行共用。</p>
          <dl class="facts">
            <div><dt>版本</dt><dd>{{ mode.version }}（{{ mode.mode }}）</dd></div>
            <div><dt>登录态</dt><dd>与 <span class="nowrap">hust-smartcourse</span> 命令行共用，保存在 state 目录</dd></div>
            <div><dt>数据</dt><dd>登录信息只保存在本机，不上传账号</dd></div>
            <div v-if="mode.debug"><dt>state 目录</dt><dd class="mono">{{ session ? session.stateDir : '—' }}</dd></div>
          </dl>
          <div class="about-actions">
            <button class="primary" @click="closeAbout">知道了</button>
          </div>
        </div>
      </div>
    </transition>

    <transition name="modal">
      <div v-if="helpOpen" class="about-backdrop" @click.self="closeHelp">
        <div class="about-dialog help-dialog" role="dialog" aria-modal="true" aria-labelledby="help-title">
          <button ref="helpCloseRef" class="about-close" @click="closeHelp" aria-label="关闭帮助">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18"/></svg>
          </button>
          <h2 id="help-title" class="about-title">帮助</h2>
          <div class="help-body">
            <section>
              <h3>入门</h3>
              <p>顶栏登录后点「开始刷课吧」选课程，课程页用三张卡切换视频、文档和作业与测验。</p>
            </section>
            <section>
              <h3>视频播放</h3>
              <p>「播放未完成」一次播完剩余任务点；播放中主按钮变成「停止播放」，其他操作会先提示停播。</p>
              <p>齿轮进「播放设置」：倍速 1–25，上报间隔 1–30 秒（默认 10）。间隔只影响上报频率，总用时 = 视频时长 / 倍速。</p>
              <p>限速课也会按设置的倍速播放；平台可能不判定通过，必要时手动补看最后部分或降速重试。</p>
            </section>
            <section>
              <h3>文档任务</h3>
              <p>先扫描附件，再「完成未完成文档」。结果按完成前后的数量给出，没完成的可以重新扫描再试。</p>
            </section>
            <section>
              <h3>作业与测验</h3>
              <p>进块会自动扫描附件、抓取未提交的题目；「刷新」从云端重抓。答题和交卷会真实提交，交卷前有确认。</p>
              <p>「自动答题」是把题目交给 AI 模型（默认 DeepSeek）作答，答案只存在本机；交卷仍要你自己确认。</p>
            </section>
            <section>
              <h3>常见问题</h3>
              <p>登录态失效时回登录页重新登录，课程与题目缓存会保留。</p>
              <p>数据都在 state 目录，和 hust-smartcourse 命令行共用；debug 构建的调试页能看到具体路径。</p>
              <p>系统通知不可用时，播放完成不会弹通知，结果仍会显示在页面和消息条里。</p>
            </section>
          </div>
          <div class="about-actions">
            <button class="primary" @click="closeHelp">知道了</button>
          </div>
        </div>
      </div>
    </transition>

    <div class="message-bar" :class="[messageKind, {show: !!messageText, leaving: noticeLeaving}]" role="status" aria-live="polite">
      <span v-if="cloudLoading" class="cloud-spinner" aria-hidden="true"></span>
      <span class="message-bar-text">{{ messageText }}</span>
    </div>
  </div>
</template>

<style scoped>
.app {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

/* 顶栏 */
.topbar {
  position: sticky;
  top: 0;
  z-index: 20;
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 52px;
  padding: 0 24px;
  background: rgba(245, 245, 247, 0.25);
  -webkit-backdrop-filter: saturate(180%) blur(30px);
  backdrop-filter: saturate(180%) blur(30px);
  border-bottom: 1px solid var(--line-soft);
}

.bar-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.back-btn {
  border: none;
  background: transparent;
  color: var(--link);
  font-size: 13px;
  padding: 4px 6px;
  border-radius: var(--radius-sm);
  cursor: pointer;
}

.back-btn:hover {
  background: rgba(0, 0, 0, 0.05);
}

.brand {
  display: inline-flex;
  align-items: baseline;
  gap: 6px;
  border: none;
  background: transparent;
  padding: 4px 2px;
  cursor: pointer;
  font-size: 13px;
  font-weight: 600;
  color: var(--text);
}

.wordmark {
  color: var(--link);
  font-weight: 700;
}

.dim {
  color: var(--muted);
  font-weight: 400;
}

.bar-right {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13px;
  color: var(--muted);
}

.user-menu {
  position: relative;
}

.account-btn {
  border: none;
  background: transparent;
  height: 28px;
  padding: 0 12px;
  border-radius: var(--radius-pill);
  color: var(--text);
  font-size: 13px;
  cursor: pointer;
}

.account-btn:hover,
.account-btn:focus-visible {
  background: rgba(0, 0, 0, 0.05);
  outline: none;
}

.menu {
  position: absolute;
  right: 0;
  top: calc(100% + 8px);
  z-index: 30;
  width: 232px;
  padding: 6px;
  background: var(--surface);
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.14);
}

.menu-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 6px 8px;
  font-size: 12px;
  color: var(--muted);
}

.menu-row .v {
  color: var(--text);
  text-align: right;
  word-break: break-all;
}

.menu-danger {
  width: 100%;
  margin-top: 4px;
  padding: 8px;
  border: none;
  border-top: 1px solid var(--line-soft);
  border-radius: 0 0 var(--radius-sm) var(--radius-sm);
  background: transparent;
  color: var(--danger);
  font-size: 12px;
  text-align: left;
  cursor: pointer;
}

.menu-danger:hover {
  background: #fff2f2;
}

.menu-action {
  width: 100%;
  margin-top: 4px;
  padding: 7px 8px;
  border: none;
  border-top: 1px solid var(--line-soft);
  border-radius: 0;
  background: transparent;
  color: var(--text);
  font-size: 12px;
  text-align: left;
  cursor: pointer;
}

.menu-action:hover {
  background: var(--surface-2);
}

.playback-chip {
  height: 28px;
  padding: 0 12px;
  border: 1px solid rgba(0, 113, 227, 0.25);
  border-radius: var(--radius-pill);
  background: #eaf1ff;
  color: var(--primary);
  font-size: 12px;
  cursor: pointer;
}

.playback-chip:hover {
  background: #dfeaFF;
}

.debug-tag {
  padding: 0 8px;
  border: 1px solid var(--line);
  border-radius: var(--radius-pill);
  color: var(--muted);
  font-size: 12px;
  line-height: 19px;
}

.login-btn {
  height: 28px;
  padding: 0 14px;
  border: none;
  border-radius: var(--radius-pill);
  background: var(--primary);
  color: #ffffff;
  font-size: 13px;
  cursor: pointer;
}

.login-btn:hover {
  background: var(--primary-hover);
}

/* 首页 */
.home {
  flex: 1 0 auto;
  width: 100%;
  max-width: 760px;
  margin: 0 auto;
  padding: 40px 24px 48px;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

.home-title {
  margin: 0 0 56px;
  font-size: 40px;
  font-weight: 700;
  letter-spacing: -0.02em;
  line-height: 1.15;
  text-align: center;
  text-wrap: balance;
  /* 标语始终单行：窄屏靠缩字号放下，不换行 */
  white-space: nowrap;
}

/* 首页主按钮：整屏只有一个焦点，周围只留几个轻量标签 */
.home-stage {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  max-width: 500px;
  min-height: 210px;
}

.stage-halo {
  position: absolute;
  width: min(400px, 92%);
  height: 200px;
  border-radius: 50%;
  background: radial-gradient(closest-side, rgba(0, 113, 227, 0.13), rgba(0, 113, 227, 0));
  pointer-events: none;
}

.course-hero {
  position: relative;
  z-index: 2;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 14px;
  width: min(280px, 100%);
  min-height: 112px;
  padding: 18px 20px;
  border: none;
  border-radius: var(--radius-xl);
  overflow: hidden;
  background:
    radial-gradient(130% 100% at 18% 0%, rgba(255, 255, 255, 0.26) 0%, rgba(255, 255, 255, 0) 52%),
    linear-gradient(148deg, #2f9dff 0%, #0071e3 46%, #0057ad 100%);
  color: #ffffff;
  cursor: pointer;
  box-shadow:
    0 22px 40px -20px rgba(0, 84, 176, 0.55),
    0 2px 6px rgba(0, 0, 0, 0.06);
  transition: transform 1s cubic-bezier(0.16, 1, 0.3, 1), box-shadow 1s cubic-bezier(0.16, 1, 0.3, 1);
}

.course-hero::before {
  content: "";
  position: absolute;
  top: -50%;
  bottom: -50%;
  left: -40%;
  width: 34%;
  background: linear-gradient(100deg, transparent 0%, rgba(255, 255, 255, 0.26) 46%, rgba(255, 255, 255, 0.05) 62%, transparent 100%);
  transform: translateX(-130%) rotate(8deg);
  transition: transform 0.55s cubic-bezier(0.2, 0.8, 0.2, 1);
  pointer-events: none;
}

.course-hero:hover {
  transform: translateY(-3px);
  box-shadow:
    0 30px 52px -22px rgba(0, 84, 176, 0.62),
    0 4px 10px rgba(0, 0, 0, 0.08);
}

.course-hero:hover::before {
  transform: translateX(430%) rotate(8deg);
}

.course-hero:active {
  transform: translateY(-1px) scale(0.997);
}

.course-hero:focus-visible {
  outline: 3px solid rgba(0, 113, 227, 0.35);
  outline-offset: 4px;
}

.hero-title {
  font-size: 22px;
  font-weight: 700;
  letter-spacing: -0.01em;
  line-height: 1.2;
}

.hero-arrow {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.18);
  transition: transform 0.2s ease, background 0.2s ease;
}

.course-hero:hover .hero-arrow {
  background: rgba(255, 255, 255, 0.26);
  transform: translateY(2px);
}

.hero-arrow svg {
  width: 16px;
  height: 16px;
  fill: none;
  stroke: currentColor;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.hero-chip {
  position: absolute;
  z-index: 1;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 30px;
  padding: 0 12px;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-pill);
  background: rgba(255, 255, 255, 0.94);
  box-shadow: 0 8px 18px -14px rgba(0, 0, 0, 0.4);
  color: var(--muted);
  font-size: 12px;
  white-space: nowrap;
  pointer-events: none;
  transform: rotate(var(--chip-tilt, 0deg));
}

.hero-chip svg {
  width: 14px;
  height: 14px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.chip-video {
  top: 30px;
  left: 38px;
  --chip-tilt: 4deg;
}

.chip-doc {
  bottom: 26px;
  left: 30px;
  --chip-tilt: -6deg;
}

.chip-quiz {
  top: 82px;
  right: 18px;
  --chip-tilt: -3deg;
}

.home-debug-row {
  margin-top: 4px;
}

.ghost-link {
  border: none;
  background: transparent;
  padding: 4px 8px;
  border-radius: var(--radius-sm);
  color: var(--muted);
  font-size: 12px;
  cursor: pointer;
}

.ghost-link:hover:not(:disabled) {
  color: var(--text);
  background: rgba(0, 0, 0, 0.04);
}

.ghost-link:disabled {
  cursor: default;
  opacity: 0.6;
}

/* 播放中不可用的按钮：压暗但保持可点，点一下用底部消息条说明原因 */
.play-blocked {
  opacity: 0.5;
  cursor: not-allowed;
}

/* 关于弹层：点顶栏品牌弹出，替代原来的关于页 */
.about-backdrop {
  position: fixed;
  inset: 0;
  z-index: 60;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: rgba(0, 0, 0, 0.18);
}

.about-dialog {
  position: relative;
  width: min(430px, 100%);
  padding: 28px 28px 22px;
  border: 1px solid var(--line-soft);
  border-radius: 22px;
  background: var(--surface);
  box-shadow: 0 32px 80px -28px rgba(0, 0, 0, 0.45);
  box-sizing: border-box;
}

.about-close {
  position: absolute;
  top: 12px;
  right: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: none;
  border-radius: 50%;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
}

.about-close:hover {
  background: rgba(0, 0, 0, 0.05);
  color: var(--text);
}

.about-close svg {
  width: 16px;
  height: 16px;
  fill: none;
  stroke: currentColor;
  stroke-width: 2;
  stroke-linecap: round;
}

.about-title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.about-desc {
  margin: 8px 0 18px;
  color: var(--muted);
  line-height: 1.6;
}

.about-dialog .facts {
  margin: 0;
}

/* 帮助弹层：内容比关于多一些，可滚动 */
.help-dialog {
  width: min(560px, 100%);
}

.help-body {
  max-height: min(60vh, 520px);
  margin-top: 14px;
  padding-right: 4px;
  overflow: auto;
}

.help-body h3 {
  margin: 14px 0 6px;
  font-size: 13px;
  font-weight: 600;
}

.help-body section:first-child h3 {
  margin-top: 0;
}

.help-body p {
  margin: 0 0 6px;
  color: var(--muted);
  font-size: 13px;
  line-height: 1.6;
}

.about-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 20px;
}

.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.16s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}

@media (max-width: 700px) {
  .home-title {
    margin-bottom: 36px;
  }

  .home-stage {
    flex-wrap: wrap;
    gap: 10px;
    min-height: 0;
    padding-top: 8px;
  }

  .stage-halo {
    display: none;
  }

  .course-hero {
    min-height: 108px;
  }

  .hero-chip {
    position: static;
    transform: none;
  }
}

.debug-info {
  width: 100%;
  max-width: 560px;
  margin: 16px auto 0;
  padding: 16px;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-lg);
  background: var(--surface);
  font-size: 12px;
  color: var(--muted);
  box-sizing: border-box;
}

.debug-info .row {
  display: flex;
  gap: 12px;
  padding: 2px 0;
}

.debug-info .k {
  flex: none;
  width: 76px;
  color: var(--muted-2);
}

.debug-info .v {
  min-width: 0;
  word-break: break-all;
}

/* 子页面：标题在面板外，面板只放内容 */
.subpage {
  flex: 1 0 auto;
  width: 100%;
  max-width: 720px;
  margin: 0 auto;
  padding: 32px 24px 40px;
  box-sizing: border-box;
}

.page-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  margin: 0 0 20px;
}

.page-head-center {
  justify-content: center;
  text-align: center;
  --coursehome-drop: clamp(48px, 10vh, 100px);
  margin-top: var(--coursehome-drop);
  margin-bottom: calc(144px - var(--coursehome-drop));
}

.page-head-center .page-title {
  font-size: 40px;
}

/* 滚动淡出：标题和入口卡在视口里定住，课程结构面板从下面升上来盖住并让它们淡出 */
.coursehome-page .page-head-center,
.coursehome-page .entry-list {
  position: relative;
  z-index: 1;
  will-change: opacity, transform;
}

.coursehome-page .panel {
  position: relative;
  z-index: 2;
}

.page-title {
  margin: 0;
  font-size: 32px;
  font-weight: 700;
  line-height: 1.2;
  letter-spacing: -0.02em;
  overflow-wrap: anywhere;
}

.page-sub {
  margin: 8px 0 0;
  color: var(--muted);
}

.panel {
  background: var(--surface);
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-lg);
  padding: 24px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04);
}

.tabs {
  display: flex;
  gap: 4px;
  margin: 0 0 16px;
}

.tabs button {
  border: none;
  background: transparent;
  padding: 6px 12px;
  border-radius: var(--radius-sm);
  color: var(--muted);
  font-size: 14px;
  cursor: pointer;
}

.tabs button.active {
  background: #eaf1ff;
  color: var(--primary);
  font-weight: 500;
}

.actions {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

button.primary {
  height: 32px;
  padding: 0 16px;
  border: none;
  border-radius: var(--radius-pill);
  background: var(--primary);
  color: #ffffff;
  font-size: 13px;
  cursor: pointer;
}

button.primary:hover:not(:disabled) {
  background: var(--primary-hover);
}

button.primary:disabled {
  background: #b9d4ff;
  cursor: default;
}

.actions button:not(.primary),
.tabs button,
.captcha-actions button {
  height: 32px;
  padding: 0 14px;
  border: 1px solid var(--line);
  border-radius: var(--radius-pill);
  background: var(--surface);
  color: var(--text);
  font-size: 13px;
  cursor: pointer;
}

.actions button:not(.primary):hover:not(:disabled),
.captcha-actions button:hover:not(:disabled) {
  background: var(--surface-2);
}

.qr-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 8px 0 16px;
}

.qr {
  width: 200px;
  height: 200px;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
}

.qr-empty {
  margin: 24px 0;
  text-align: center;
  color: var(--muted);
}

.field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 12px;
}

.field label {
  color: var(--muted);
  font-size: 13px;
}

.field input {
  height: 32px;
  padding: 0 10px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--text);
  font-size: 14px;
  outline: none;
}

.field input:focus {
  border-color: var(--primary);
}

/* 字段下方的小提示：申请 Key 的入口、模型怎么填 */
.field-hint {
  margin: 2px 0 0;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.55;
}

.field-hint a {
  color: var(--link);
  text-decoration: none;
}

.field-hint a:hover,
.field-hint a:focus-visible {
  text-decoration: underline;
  outline: none;
}

.captcha-row {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  margin: 4px 0 16px;
}

.captcha-box {
  width: 110px;
  height: 42px;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  flex: none;
}

.captcha {
  width: 100%;
  height: 100%;
  object-fit: contain;
  cursor: pointer;
}

.captcha-side {
  flex: 1;
  min-width: 0;
}

.captcha-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}

.captcha-input {
  width: 110px;
  height: 32px;
  padding: 0 10px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  font-size: 14px;
  outline: none;
}

.captcha-input:focus {
  border-color: var(--primary);
}

.hint {
  margin: 6px 0 0;
  color: var(--muted);
  font-size: 12px;
}

/* 自动答题 = 调 AI 的说明，和列表状态提示贴在一起 */
.ai-hint {
  margin-top: 2px;
}

.facts {
  margin: 16px 0 20px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.facts > div {
  display: flex;
  gap: 12px;
}

.facts dt {
  flex: none;
  width: 84px;
  color: var(--muted);
  font-size: 13px;
}

.facts dd {
  margin: 0;
  min-width: 0;
  /* 路径要能断，普通单词不断：只用 anywhere 兜底，不要 break-all */
  word-break: normal;
  overflow-wrap: anywhere;
}

.nowrap {
  white-space: nowrap;
}

.course-list {
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
  margin: 8px 0;
  overflow: hidden;
}

.course-row {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  padding: 14px 16px;
  border: none;
  border-bottom: 1px solid #f5f6f7;
  background: transparent;
  font-family: inherit;
  font-size: 13px;
  color: var(--text);
  text-align: left;
  cursor: pointer;
  transition: background 0.15s ease;
}

.course-row:last-child {
  border-bottom: none;
}

.course-row:hover:not(:disabled) {
  background: var(--surface-2);
}

.course-row:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: -2px;
}

.course-row:disabled {
  cursor: default;
  opacity: 0.6;
}

.course-index {
  flex: none;
  width: 24px;
  color: var(--muted-2);
  font-variant-numeric: tabular-nums;
}

.course-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
  font-weight: 500;
}

.course-teacher {
  flex: none;
  width: 96px;
  color: var(--muted);
  font-size: 12px;
  text-align: right;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.course-chevron {
  flex: none;
  width: 16px;
  height: 16px;
  fill: none;
  stroke: var(--muted-2);
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}

/* 课程链接入口 */
.course-link {
  margin-top: 14px;
  padding: 8px 12px;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
}

.course-link summary {
  cursor: pointer;
  color: var(--muted);
  font-size: 13px;
}

.course-link-row {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-top: 10px;
}

.course-link-row input {
  flex: 1;
  min-width: 0;
  height: 32px;
  padding: 0 10px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--text);
  font-size: 13px;
  outline: none;
}

.course-link-row input:focus {
  border-color: var(--primary);
}

.course-link-row button {
  flex: none;
  height: 32px;
  padding: 0 14px;
  border: 1px solid var(--line);
  border-radius: var(--radius-pill);
  background: var(--surface);
  color: var(--text);
  font-size: 13px;
  cursor: pointer;
}

.course-link-row button:hover:not(:disabled) {
  background: var(--surface-2);
}

.course-link-row button:disabled {
  color: var(--muted);
  cursor: default;
}

/* 课程页入口卡：三张卡当标签页，选中态用蓝色描边 + 浅蓝底 */
.entry-list {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  width: 100%;
  max-width: 560px;
  margin: 0 auto 120px;
}

.entry-row {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 104px;
  width: 100%;
  padding: 16px;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-lg);
  background: var(--surface);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04);
  color: var(--text);
  font: inherit;
  text-align: center;
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease, box-shadow 0.15s ease;
}

.entry-row:hover {
  background: var(--surface-2);
  border-color: rgba(0, 0, 0, 0.1);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
}

.entry-row:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

.entry-row.active {
  border-color: rgba(0, 113, 227, 0.45);
  background: #f2f7ff;
  box-shadow: 0 1px 2px rgba(0, 113, 227, 0.12);
}

.entry-row.active .entry-name {
  color: var(--link);
}

.entry-main {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.entry-name {
  font-size: 15px;
  font-weight: 600;
}

.entry-main .hint {
  margin: 0;
  line-height: 1.45;
}

/* 课程页：标题下的重新抓取 + 面板里的块标题 */
.head-refresh {
  margin-top: 8px;
}

.block-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin: 0 0 16px;
}

.block-head-text {
  display: flex;
  align-items: baseline;
  gap: 10px;
  flex-wrap: wrap;
  min-width: 0;
}

.block-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  letter-spacing: -0.01em;
}

.play-hint {
  color: var(--muted);
  font-size: 12px;
}

/* 视频 / 文档任务：总览、筛选、清单 */
.task-overview {
  margin: 0 0 20px;
  padding: 16px 18px;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
  background: var(--surface-2);
}

.overview-top {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.overview-figure {
  display: flex;
  align-items: baseline;
  gap: 10px;
  min-width: 0;
}

.overview-figure b {
  font-size: 28px;
  font-weight: 600;
  line-height: 1;
  letter-spacing: -0.02em;
  font-variant-numeric: tabular-nums;
}

.overview-caption {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
}

.overview-chips {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.stat-chip {
  padding: 3px 10px;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-pill);
  background: var(--surface);
  color: var(--muted);
  font-size: 12px;
}

.stat-chip b {
  color: var(--text);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.overview-track {
  height: 6px;
  margin: 12px 0 0;
  border-radius: 3px;
  background: #e5e5ea;
  overflow: hidden;
}

.overview-fill {
  height: 100%;
  border-radius: 3px;
  background: var(--primary);
  transition: width 0.4s ease;
}

.overview-empty .overview-caption {
  color: var(--text);
  font-size: 14px;
  font-weight: 500;
}

.overview-empty .hint {
  margin: 4px 0 0;
}

.live-block {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--line-soft);
}

.live-head {
  display: flex;
  align-items: baseline;
  gap: 10px;
  flex-wrap: wrap;
}

.live-title {
  min-width: 0;
  font-size: 13px;
  word-break: break-all;
}

.live-meta {
  color: var(--muted);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

.list-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin: 0 0 10px;
}

.list-title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
}

.filter-row {
  display: flex;
  gap: 2px;
  padding: 3px;
  border-radius: var(--radius-pill);
  background: #f0f0f3;
}

.filter-row button {
  height: 26px;
  padding: 0 12px;
  border: none;
  border-radius: var(--radius-pill);
  background: transparent;
  color: var(--muted);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
  cursor: pointer;
}

.filter-row button:hover {
  color: var(--text);
}

.filter-row button.active {
  background: var(--surface);
  color: var(--text);
  font-weight: 500;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.06);
}

.filter-row button:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

.chapter-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 0;
}

.chapter {
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
  padding: 0;
  background: var(--surface);
  scroll-margin-top: 64px;
}

.chapter summary {
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  list-style: none;
}

.chapter summary::-webkit-details-marker {
  display: none;
}

.chapter summary:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: -2px;
  border-radius: var(--radius-md);
}

.chapter-name {
  font-weight: 500;
}

.chapter summary .chapter-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
}

.chapter-bar {
  flex: none;
  width: 48px;
  height: 4px;
  border-radius: 2px;
  background: #e5e5ea;
  overflow: hidden;
}

.chapter-bar i {
  display: block;
  height: 100%;
  border-radius: 2px;
  background: var(--primary);
}

.chapter-count {
  flex: none;
  color: var(--muted);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

.chapter-chevron {
  flex: none;
  width: 16px;
  height: 16px;
  fill: none;
  stroke: var(--muted);
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
  transition: transform 0.2s cubic-bezier(0.2, 0.8, 0.2, 1);
}

.chapter[open] .chapter-chevron {
  transform: rotate(90deg);
}

.task-list {
  margin: 0;
  padding: 0;
  list-style: none;
  border-top: 1px solid var(--line-soft);
}

/* 收起的章节不要留布局盒子：Chromium 的 details 用 content-visibility 藏内容，
   自绘 summary 时会被量成“文字叠在一起”，这里直接 display: none。 */
.chapter:not([open]) > .task-list {
  display: none;
}

.task-list li {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 14px;
  font-size: 13px;
}

.task-list li + li {
  border-top: 1px solid var(--line-soft);
}

.task-main {
  display: flex;
  flex: 1;
  min-width: 0;
  flex-direction: column;
  gap: 2px;
}

.task-title {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 窄窗口里标题允许折行，把“未完成”标签让给播放按钮（已完成仍显示） */
@media (max-width: 480px) {
  .status-chip:not(.is-done) {
    display: none;
  }

  .task-title {
    overflow: visible;
    white-space: normal;
    overflow-wrap: break-word;
  }
}

.task-meta {
  color: var(--muted);
  font-size: 12px;
}

.status-chip {
  flex: none;
  padding: 2px 8px;
  border-radius: var(--radius-pill);
  background: #f0f0f3;
  color: var(--muted);
  font-size: 12px;
}

.status-chip.is-done {
  background: #e8f5ec;
  color: #1a6b2f;
}

.status-chip.is-failed {
  background: #fdeeee;
  color: #a02f2a;
}

.task-empty {
  color: var(--muted);
}

.task-tag {
  flex: none;
  font-size: 11px;
  color: var(--muted);
  border: 1px solid var(--line-soft);
  border-radius: 3px;
  padding: 0 4px;
}

.ok-text {
  color: #1a6b2f;
}

.warn-text {
  color: #a35c00;
}

/* 播放 */
.play-speed {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--muted);
  font-size: 13px;
}

.play-speed select {
  height: 30px;
  padding: 0 8px;
  border: 1px solid var(--line);
  border-radius: var(--radius-sm);
  background: var(--surface);
  color: var(--text);
  font-size: 13px;
}

/* 工具条里的当前倍速 / 间隔摘要 + 播放设置按钮（浅蓝描边，比图标更显眼） */
.play-summary {
  color: var(--muted);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

.actions button.settings-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 30px;
  padding: 0 12px;
  border: 1px solid rgba(0, 113, 227, 0.28);
  border-radius: var(--radius-pill);
  background: #f2f7ff;
  color: var(--link);
  font-size: 13px;
  cursor: pointer;
}

.actions button.settings-btn:hover,
.actions button.settings-btn:focus-visible {
  border-color: rgba(0, 113, 227, 0.5);
  background: #e8f1ff;
  outline: none;
}

.settings-btn svg {
  width: 15px;
  height: 15px;
  flex: none;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.8;
  stroke-linecap: round;
}

/* 播放设置页：一段一个设置，留白为主 */
.play-setting {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 2px 0 24px;
  margin-bottom: 24px;
  border-bottom: 1px solid var(--line-soft);
}

.play-setting:last-of-type {
  border-bottom: none;
  padding-bottom: 8px;
  margin-bottom: 8px;
}

.setting-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
}

.setting-head label {
  color: var(--muted);
  font-size: 13px;
}

.setting-head strong {
  font-size: 22px;
  font-weight: 600;
  letter-spacing: -0.01em;
  font-variant-numeric: tabular-nums;
}

.play-setting input[type='range'] {
  width: 100%;
  /* 滑条本身只有 16px 高，上下补 padding 把可点范围撑到 32px */
  padding: 8px 0;
  accent-color: var(--primary);
}

.setting-note {
  margin: 0;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.65;
}

/* 限速课的风险说明用黄色字，比普通说明更醒目 */
.setting-note.is-warn {
  color: #a35c00;
}

.play-setting-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.preset-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.preset-row button {
  height: 28px;
  padding: 0 12px;
  border: 1px solid var(--line);
  border-radius: var(--radius-pill);
  background: var(--surface);
  color: var(--text);
  font-size: 12px;
  cursor: pointer;
}

.preset-row button.active {
  border-color: rgba(0, 113, 227, 0.45);
  background: #f2f7ff;
  color: var(--link);
}

.play-line {
  display: flex;
  align-items: baseline;
  gap: 10px;
  flex-wrap: wrap;
}

.play-message {
  font-size: 13px;
  min-width: 0;
  word-break: break-all;
}

.play-line .hint {
  margin: 0;
}

.play-track {
  height: 4px;
  margin-top: 8px;
  border-radius: 2px;
  background: #e5e5ea;
  overflow: hidden;
}

.play-fill {
  height: 100%;
  border-radius: 2px;
  background: var(--primary);
  transition: width 0.4s ease;
}

.task-list li.playing {
  background: #eaf1ff;
  border-radius: 4px;
}

.row-play {
  flex: none;
  height: 24px;
  padding: 0 10px;
  border: 1px solid var(--line);
  border-radius: var(--radius-pill);
  background: var(--surface);
  color: var(--link);
  font-size: 12px;
  cursor: pointer;
}

.row-play:hover:not(:disabled) {
  background: var(--surface-2);
}

.row-play:disabled {
  color: var(--muted);
  cursor: default;
}

/* 作业与测验列表 */
.quiz-list {
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
  margin: 10px 0;
}

.quiz-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  border-bottom: 1px solid #f5f6f7;
  font-size: 13px;
}

.quiz-row:last-child {
  border-bottom: none;
}

.quiz-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.quiz-row button {
  flex: none;
  height: 28px;
  padding: 0 12px;
  border: 1px solid var(--line);
  border-radius: var(--radius-pill);
  background: var(--surface);
  color: var(--text);
  font-size: 12px;
  cursor: pointer;
}

.quiz-row button:hover:not(:disabled) {
  background: var(--surface-2);
}

.quiz-row button:disabled {
  color: var(--muted);
  cursor: default;
}

/* 题目 */
/* 自动答题工具区 */
.quiz-tools {
  margin: 12px 0 8px;
}

.quiz-tools .hint {
  margin: 0;
}

.quiz-progress {
  margin: 10px 0;
  padding: 10px 12px;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
  background: var(--surface-2);
}

.quiz-progress .play-line .hint {
  margin: 0;
}

.quiz-peek {
  margin: 10px 0;
  padding: 4px 12px;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
}

.quiz-peek-list {
  margin: 0;
  padding: 0;
  list-style: none;
}

.quiz-peek-list li {
  display: flex;
  align-items: baseline;
  gap: 10px;
  padding: 6px 0;
  border-bottom: 1px solid #f5f6f7;
  font-size: 13px;
}

.quiz-peek-list li:last-child {
  border-bottom: none;
}

.quiz-peek-list .hint {
  margin: 0;
  flex: none;
}

/* 测验设置 */
.settings-block {
  margin: 4px 0 16px;
}

.settings-label {
  display: block;
  margin-bottom: 6px;
  color: var(--muted);
  font-size: 13px;
}

.settings-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 4px 16px;
  margin: 4px 0 16px;
}

.settings-grid .field {
  margin-bottom: 8px;
}

.check-row {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text);
  font-size: 13px;
  cursor: pointer;
}

.check-row input {
  /* 真实输入框做成 24px 透明覆盖层：点击区够大，视觉交给 span::before 画 16px 方框 */
  position: absolute;
  left: 0;
  top: 50%;
  width: 24px;
  height: 24px;
  margin: 0;
  transform: translateY(-50%);
  opacity: 0;
  cursor: pointer;
}

.check-row {
  position: relative;
  padding-left: 32px;
}

.check-row span::before {
  content: '';
  position: absolute;
  left: 4px;
  top: 50%;
  width: 16px;
  height: 16px;
  transform: translateY(-50%);
  border: 1px solid var(--line);
  border-radius: 4px;
  background: var(--surface);
  box-sizing: border-box;
}

.check-row input:checked + span::before {
  border-color: var(--primary);
  background: var(--primary) url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 16 16'%3E%3Cpath d='m3.5 8.5 3 3 6-7' fill='none' stroke='%23fff' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'/%3E%3C/svg%3E") center/12px no-repeat;
}

.check-row input:focus-visible + span::before {
  outline: 2px solid var(--link);
  outline-offset: 2px;
}

.check-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 18px;
}

.quiz-detail-head {
  margin-bottom: 12px;
}

.quiz-detail-head .hint {
  margin: 0;
}

.quiz-questions {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.quiz-question {
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
  padding: 10px 12px;
}

.quiz-stem {
  display: flex;
  align-items: baseline;
  gap: 8px;
  font-size: 13px;
}

.quiz-stem-text {
  flex: 1;
  min-width: 0;
  word-break: break-word;
}

.quiz-stem .hint {
  margin: 0;
  flex: none;
}

.quiz-options {
  margin: 8px 0 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 13px;
  color: var(--text);
}

.quiz-options li {
  word-break: break-word;
}

.quiz-options b {
  font-weight: 500;
  color: var(--muted);
}

.inline-error {
  margin: 8px 0;
  padding: 6px 10px;
  border-radius: var(--radius-sm);
  background: #fde2e2;
  color: var(--danger);
  font-size: 13px;
}

.debug-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 6px 16px;
  margin: 12px 0;
}

.debug-grid > div {
  display: flex;
  gap: 8px;
  min-width: 0;
}

/* 日志路径太长，单独占满一行，减少断行 */
.debug-grid > div.debug-wide {
  grid-column: 1 / -1;
}

.debug-grid span {
  flex: none;
  width: 76px;
  color: var(--muted);
  font-size: 13px;
}

.debug-grid b {
  font-weight: 400;
  min-width: 0;
  word-break: break-all;
}

.panel h3 {
  margin: 16px 0 8px;
  font-size: 14px;
  font-weight: 600;
}

.debug-log {
  margin: 0;
  padding: 0;
  list-style: none;
  max-height: 200px;
  overflow: auto;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
}

.debug-log li {
  display: flex;
  gap: 8px;
  padding: 4px 8px;
  border-bottom: 1px solid #f5f6f7;
  font-size: 12px;
}

.debug-log li:last-child {
  border-bottom: none;
}

.debug-log em {
  flex: none;
  width: 40px;
  font-style: normal;
  color: var(--muted);
}

.mono {
  font-family: Consolas, "Cascadia Mono", monospace;
}

.debug-json {
  margin: 0;
  padding: 8px;
  max-height: 220px;
  overflow: auto;
  background: var(--surface-2);
  border-radius: var(--radius-sm);
  font-size: 12px;
  white-space: pre-wrap;
}

/* 底部消息条：加载中显示转圈 + 文案，平时显示提示；节点常驻，只切显隐不改结构 */
.message-bar {
  position: fixed;
  left: 50%;
  bottom: 20px;
  z-index: 50;
  display: flex;
  align-items: center;
  gap: 10px;
  max-width: 80%;
  padding: 8px 14px;
  border-radius: var(--radius-pill);
  background: rgba(29, 29, 31, 0.9);
  color: #ffffff;
  font-size: 13px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.18);
  opacity: 0;
  visibility: hidden;
  pointer-events: none;
  transform: translate(-50%, 8px);
  transition: opacity 0.18s ease, transform 0.18s cubic-bezier(0.2, 0.8, 0.2, 1);
}

.message-bar.show {
  opacity: 1;
  visibility: visible;
  transform: translate(-50%, 0);
}

/* 成功提示到点后单独用 1 秒的透明度过渡淡出，文案在淡出期间保留 */
.message-bar.leaving {
  opacity: 0;
  transition: opacity 1s ease;
}

.message-bar.error {
  background: rgba(215, 0, 21, 0.92);
}

.message-bar.ok {
  background: rgba(26, 107, 47, 0.92);
}

.message-bar-text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cloud-spinner {
  flex: none;
  width: 14px;
  height: 14px;
  border: 2px solid rgba(255, 255, 255, 0.28);
  border-top-color: #ffffff;
  border-radius: 50%;
  animation: cloud-spin 0.8s linear infinite;
}

@keyframes cloud-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .message-bar,
  .message-bar.leaving {
    transition: none;
  }

  .cloud-spinner {
    animation: none;
  }
}

@media (max-width: 680px) {
  .home-title {
    margin-bottom: 80px;
    /* 6vw 保证最长 14 字的标语在 375 宽也放得下，且始终单行 */
    font-size: clamp(22px, 6vw, 32px);
  }

  .topbar {
    padding: 0 16px;
  }

  .subpage {
    padding: 20px 16px 32px;
  }

  .page-title {
    font-size: 28px;
  }

  .page-head-center .page-title {
    font-size: 32px;
  }

  .settings-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 560px) {
  .entry-list {
    grid-template-columns: 1fr;
  }

  .entry-row {
    min-height: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .course-hero,
  .hero-arrow,
  .entry-row {
    transition: none;
  }

  .course-hero::before {
    display: none;
  }

  .course-hero:hover,
  .course-hero:active,
  .course-hero:hover .hero-arrow {
    transform: none;
  }
}
</style>
