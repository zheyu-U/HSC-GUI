<script setup>
import {computed, onMounted, onUnmounted, ref} from 'vue'
import {
  AppMode,
  CancelPlay,
  CancelQRLogin,
  CheckSession,
  ClearDebugLog,
  CompleteDocs,
  DebugInfo,
  DebugLog,
  GetCourseDetail,
  ListCourses,
  LoginPassword,
  Logout,
  RefreshCourse,
  ScanCourse,
  SelectCourse,
  SessionStatus,
  StartCaptcha,
  StartPlay,
  StartQRLogin,
} from '../wailsjs/go/main/App'
import {EventsOff, EventsOn} from '../wailsjs/runtime/runtime'

const TITLES = [
  '今天打算刷什么？',
  '今天从哪门课开始？',
  '先把任务点清一清',
  '该刷课了',
]

const page = ref('home')
const title = ref(TITLES[0])
const mode = ref({mode: 'release', debug: false, version: ''})
const session = ref(null)
const notice = ref('')
const noticeKind = ref('info')
const menuOpen = ref(false)

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

const docBusy = ref(false)
const playRunning = ref(false)
const playSpeed = ref(1)
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

const debugInfo = ref(null)
const debugLog = ref([])
const debugBusy = ref(false)

const loggedIn = computed(() => !!(session.value && session.value.loggedIn))
const account = computed(() => (session.value && session.value.account) || '')
const accountTail = computed(() => (session.value && session.value.accountTail) || '')
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
const worksByChapter = computed(() => {
  const map = {}
  const list = (courseDetail.value && courseDetail.value.works) || []
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
const docScanned = computed(() => !!(courseDetail.value && courseDetail.value.docScanned))
const docUnpassed = computed(() => {
  const list = (courseDetail.value && courseDetail.value.docs) || []
  return list.filter((d) => !d.isPassed).length
})
const docPassed = computed(() => {
  const list = (courseDetail.value && courseDetail.value.docs) || []
  return list.filter((d) => d.isPassed).length
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
  if (page.value === 'about') {
    return '关于'
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
  return ''
})

function friendly(e) {
  return String(e).replace(/^Error:\s*/, '')
}

function setNotice(message, kind) {
  notice.value = message || ''
  noticeKind.value = kind || 'info'
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

function onDocumentClick() {
  closeMenu()
}

function onKeydown(e) {
  if (e.key === 'Escape') {
    closeMenu()
  }
}

function showPage(name) {
  if (name === 'debug' && !mode.value.debug) {
    return
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
  if ((name === 'home' || name === 'debug') && mode.value.debug) {
    refreshDebug()
  }
}

function openCourse() {
  if (!loggedIn.value) {
    showPage('login')
    return
  }
  showPage('course')
}

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

async function doLogout() {
  closeMenu()
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
    const res = await Logout(false)
    view.value = 'login'
    justLoggedIn.value = false
    page.value = 'home'
    qrImage.value = ''
    qrMessage.value = ''
    checkMessage.value = ''
    courseList.value = []
    courseDetail.value = null
    courseError.value = ''
    setNotice(res.message, 'ok')
    await refreshSession()
  } catch (e) {
    setNotice('退出登录失败: ' + friendly(e), 'error')
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
    courseError.value = friendly(e)
  }
}

async function loadCourses(refresh) {
  if (!loggedIn.value) {
    return
  }
  courseLoading.value = true
  courseError.value = ''
  try {
    courseList.value = await ListCourses(refresh !== false)
  } catch (e) {
    courseError.value = friendly(e)
  } finally {
    courseLoading.value = false
    refreshDebug()
  }
}

async function selectCourse(index) {
  courseBusy.value = true
  courseError.value = ''
  try {
    const detail = await SelectCourse(index)
    courseDetail.value = detail
    playState.value = emptyPlayState()
    setNotice('已切换到课程 ' + (detail.target.name || detail.target.courseId), 'ok')
    await refreshSession()
  } catch (e) {
    courseError.value = friendly(e)
  } finally {
    courseBusy.value = false
    refreshDebug()
  }
}

async function refreshCourse() {
  courseBusy.value = true
  courseError.value = ''
  try {
    courseDetail.value = await RefreshCourse()
    setNotice('课程结构已更新', 'ok')
  } catch (e) {
    courseError.value = friendly(e)
  } finally {
    courseBusy.value = false
    refreshDebug()
  }
}

function isCurrent(course) {
  return currentCourseID.value && course.courseId === currentCourseID.value
}

async function startPlay(objectID) {
  if (playRunning.value) {
    return
  }
  setNotice('')
  playState.value = emptyPlayState()
  try {
    const res = await StartPlay(objectID || '', Number(playSpeed.value) || 1)
    playRunning.value = true
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
    setNotice(friendly(e), 'error')
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
  if (ev.state === 'finished' || ev.state === 'cancelled') {
    playRunning.value = false
    loadCourseDetail()
    if (ev.message) {
      setNotice(ev.message, ev.state === 'finished' ? 'ok' : 'info')
    }
  }
  refreshDebug()
}

async function scanDocs() {
  docBusy.value = true
  setNotice('')
  try {
    courseDetail.value = await ScanCourse()
    setNotice('文档任务扫描完成', 'ok')
  } catch (e) {
    setNotice(friendly(e), 'error')
  } finally {
    docBusy.value = false
    refreshDebug()
  }
}

async function completeDocs() {
  docBusy.value = true
  setNotice('')
  try {
    courseDetail.value = await CompleteDocs()
    setNotice('文档任务处理完成，状态已刷新', 'ok')
  } catch (e) {
    setNotice(friendly(e), 'error')
  } finally {
    docBusy.value = false
    refreshDebug()
  }
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
  }
  if (mode.value.debug) {
    refreshDebug()
  }
}

let offQr = null
let offPlay = null

onMounted(() => {
  bootstrap()
  offQr = EventsOn('login:qr', onQrEvent)
  offPlay = EventsOn('play:progress', onPlayEvent)
  document.addEventListener('click', onDocumentClick)
  window.addEventListener('keydown', onKeydown)
})

onUnmounted(() => {
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
  document.removeEventListener('click', onDocumentClick)
  window.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <div class="app">
    <header class="topbar">
      <div class="bar-left">
        <button v-if="page !== 'home'" class="back-btn" @click="showPage('home')">返回</button>
        <button class="brand" @click="showPage('home')" aria-label="回到首页">
          <span class="wordmark">hsc</span>
          <span class="dim">课程助手</span>
        </button>
      </div>

      <div class="bar-right">
        <template v-if="loggedIn">
          <div class="user-menu" @click.stop>
            <button class="account-btn" @click="toggleMenu">尾号 {{ accountTail || '—' }}</button>
            <div v-if="menuOpen" class="menu" role="menu">
              <div class="menu-row"><span>账号</span><span class="v">{{ account || '未保存' }}</span></div>
              <div class="menu-row"><span>登录状态</span><span class="v">{{ loggedIn ? '有效' : '无' }}</span></div>
              <div class="menu-row"><span>记住账号</span><span class="v">{{ session && session.credSaved ? '已保存' : '未保存' }}</span></div>
              <div class="menu-row"><span>课程</span><span class="v">{{ session && session.hasTarget ? '已配置' : '未配置' }}</span></div>
              <button class="menu-danger" @click="doLogout">退出登录</button>
            </div>
          </div>
          <span v-if="mode.debug" class="debug-tag">debug</span>
        </template>
        <button v-else-if="page !== 'login'" class="login-btn" @click="showPage('login')">登录</button>
      </div>
    </header>

    <main v-if="page === 'home'" class="home">
      <h1 class="home-title">{{ title }}</h1>
      <nav class="cards" aria-label="页面导航">
        <button class="card" :class="{disabled: !loggedIn}" @click="openCourse">
          <h2>课程</h2>
          <p>{{ loggedIn ? '课程列表与任务点' : '课程列表与任务点，登录后可用' }}</p>
        </button>
        <button class="card" @click="showPage('about')">
          <h2>关于</h2>
          <p>版本与说明</p>
        </button>
        <button v-if="mode.debug" class="card" @click="showPage('debug')">
          <h2>调试</h2>
          <p>环境与调用日志</p>
        </button>
      </nav>

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

    <main v-else class="subpage">
      <header class="page-head">
        <div class="page-head-text">
          <h1 class="page-title">{{ pageTitle }}</h1>
          <p v-if="pageLead" class="page-sub">{{ pageLead }}</p>
        </div>
        <div v-if="page === 'course'" class="actions">
          <button @click="loadCourses(true)" :disabled="courseLoading || courseBusy || playRunning">
            {{ courseLoading ? '获取中' : '获取课程列表' }}
          </button>
          <button v-if="hasCourse" @click="refreshCourse" :disabled="courseBusy || playRunning">
            {{ courseBusy ? '抓取中' : '抓取课程结构' }}
          </button>
        </div>
        <div v-else-if="page === 'debug' && mode.debug" class="actions">
          <button @click="refreshDebug" :disabled="debugBusy">{{ debugBusy ? '刷新中' : '刷新' }}</button>
          <button @click="clearDebug">清空日志</button>
        </div>
      </header>

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
          <p v-if="courseDetail" class="hint">{{ courseDetail.message }}</p>
          <p v-if="courseError" class="inline-error">{{ courseError }}</p>

          <div v-if="courseList.length" class="course-list">
            <div v-for="item in courseList" :key="item.courseId" class="course-row">
              <span class="course-index">{{ item.index }}</span>
              <span class="course-name">{{ item.name || '未命名课程' }}</span>
              <span class="course-teacher">{{ item.teacher }}</span>
              <span v-if="isCurrent(item)" class="course-current">当前</span>
              <button @click="selectCourse(item.index)" :disabled="courseBusy || courseLoading || playRunning">选择</button>
            </div>
          </div>

          <template v-if="hasCourse">
            <div class="course-summary">
              <span>章节 <b>{{ courseDetail.summary.chapters }}</b></span>
              <span>视频 <b>{{ courseDetail.summary.videosPassed }}/{{ courseDetail.summary.videos }}</b></span>
              <span>作业 <b>{{ courseDetail.summary.works }}</b></span>
              <span v-if="courseDetail.summary.docs">文档 <b>{{ courseDetail.summary.docsPassed }}/{{ courseDetail.summary.docs }}</b></span>
              <span>总时长 <b>{{ courseDetail.summary.durationHours }}</b> 小时</span>
              <span v-if="courseDetail.summary.speedLimited">限 1 倍速 <b>{{ courseDetail.summary.speedLimited }}</b> 个</span>
              <span v-if="courseDetail.stale" class="warn-text">缓存属于另一门课</span>
            </div>

            <div class="play-bar">
              <div class="play-controls actions">
                <label class="play-speed">
                  倍速
                  <select v-model.number="playSpeed" :disabled="playRunning">
                    <option :value="1">1x</option>
                    <option :value="1.5">1.5x</option>
                    <option :value="2">2x</option>
                    <option :value="3">3x</option>
                  </select>
                </label>
                <button class="primary" @click="startPlay('')" :disabled="playRunning || docBusy || !unpassedVideos.length">
                  {{ playRunning ? '播放中' : (unpassedVideos.length ? '播放未完成 ' + unpassedVideos.length + ' 个' : '视频都已完成') }}
                </button>
                <button v-if="playRunning" @click="stopPlay">停止播放</button>
                <span class="hint">限 1 倍速的任务点会按 1 倍播放</span>
              </div>
              <div v-if="playState.message" class="play-status">
                <div class="play-line">
                  <span class="play-message">{{ playState.message }}</span>
                  <span v-if="playState.index" class="hint">第 {{ playState.index }}/{{ playState.total }} 个</span>
                  <span v-if="playState.speedLimited" class="hint">限 1 倍速</span>
                  <span v-if="playState.duration" class="hint">{{ formatClock(playState.played) }} / {{ formatClock(playState.duration) }}</span>
                </div>
                <div v-if="playRunning && playState.duration" class="play-track">
                  <div class="play-fill" :style="{width: Math.min(100, playState.percent || 0) + '%'}"></div>
                </div>
              </div>
            </div>

            <div class="chapter-list">
              <details v-for="ch in courseDetail.chapters" :key="ch.id" class="chapter" :open="ch.index <= 2">
                <summary>
                  <span class="chapter-name">{{ ch.title || ('章节 ' + ch.index) }}</span>
                  <span class="hint">视频 {{ ch.videosPassed }}/{{ ch.videos }}，作业 {{ ch.works }}</span>
                </summary>
                <ul class="task-list">
                  <li v-for="(v, index) in videosByChapter[ch.id] || []" :key="'v' + ch.id + index" :class="{playing: playRunning && playState.objectId === v.objectId}">
                    <span class="task-tag">视频</span>
                    <span class="task-title">{{ v.title || '未命名视频' }}</span>
                    <span class="hint">{{ formatDuration(v.duration) }}</span>
                    <span v-if="v.isPassed" class="ok-text">已完成</span>
                    <span v-else class="hint">未完成</span>
                    <span v-if="v.speedLimited" class="hint">限 1 倍速</span>
                    <button v-if="!v.isPassed" class="row-play" @click="startPlay(v.objectId)" :disabled="playRunning || docBusy">播放</button>
                  </li>
                  <li v-for="(w, index) in worksByChapter[ch.id] || []" :key="'w' + ch.id + index">
                    <span class="task-tag">作业</span>
                    <span class="task-title">{{ w.title || '未命名作业' }}</span>
                  </li>
                  <li v-if="!((videosByChapter[ch.id] || []).length || (worksByChapter[ch.id] || []).length)" class="hint">
                    本章没有可展示的任务点
                  </li>
                </ul>
              </details>
            </div>

            <details class="doc-section" open>
              <summary>
                <span class="chapter-name">文档任务</span>
                <span class="hint" v-if="docScanned">已完成 {{ docPassed }}/{{ courseDetail.docs.length }}</span>
                <span class="hint" v-else>未扫描</span>
              </summary>
              <div class="doc-actions actions">
                <button @click="scanDocs" :disabled="docBusy || playRunning">
                  {{ docBusy ? '处理中' : (docScanned ? '重新扫描' : '扫描文档任务') }}
                </button>
                <button v-if="docScanned && docUnpassed" class="primary" @click="completeDocs" :disabled="docBusy || playRunning">
                  完成未完成的文档 {{ docUnpassed }} 个
                </button>
                <span class="hint" v-if="!docScanned">扫描会逐章读取附件，可能需要十几秒</span>
              </div>
              <p v-if="docScanned && !courseDetail.docs.length" class="hint">这门课没有文档任务点。</p>
              <ul v-if="courseDetail.docs.length" class="task-list doc-list">
                <li v-for="(d, index) in courseDetail.docs" :key="'d' + d.chapterId + index">
                  <span class="task-tag">文档</span>
                  <span class="task-title">{{ d.title || '未命名文档' }}</span>
                  <span v-if="d.chapterIndex" class="hint">第 {{ d.chapterIndex }} 章</span>
                  <span v-if="d.isPassed" class="ok-text">已完成</span>
                  <span v-else class="hint">未完成</span>
                </li>
              </ul>
            </details>
          </template>
        </template>

        <template v-else-if="page === 'about'">
          <dl class="facts">
            <div><dt>版本</dt><dd>{{ mode.version }}（{{ mode.mode }}）</dd></div>
            <div><dt>登录态</dt><dd>与 hsc 命令行共用，保存在 state 目录</dd></div>
            <div><dt>数据</dt><dd>登录信息只保存在本机，不上传账号</dd></div>
            <template v-if="mode.debug">
              <div><dt>state 目录</dt><dd class="mono">{{ session ? session.stateDir : '—' }}</dd></div>
            </template>
          </dl>
          <div class="actions">
            <button class="primary" @click="showPage('home')">回首页</button>
          </div>
        </template>

        <template v-else-if="page === 'debug' && mode.debug">
          <div v-if="debugInfo" class="debug-grid">
            <div><span>模式</span><b>{{ debugInfo.mode }}</b></div>
            <div><span>state 目录</span><b class="mono">{{ debugInfo.stateDir }}</b></div>
            <div><span>工作目录</span><b class="mono">{{ debugInfo.cwd }}</b></div>
            <div><span>可执行文件</span><b class="mono">{{ debugInfo.exePath }}</b></div>
            <div><span>Go 版本</span><b>{{ debugInfo.goVersion }}</b></div>
            <div><span>系统</span><b>{{ debugInfo.os }}/{{ debugInfo.arch }}</b></div>
            <div><span>cookie 数量</span><b>{{ debugInfo.cookieCount }}</b></div>
            <div><span>登录态</span><b>{{ debugInfo.loggedIn ? '存在' : '无' }}</b></div>
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

    <p v-if="notice" class="notice" :class="noticeKind">{{ notice }}</p>
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
  background: rgba(245, 245, 247, 0.72);
  -webkit-backdrop-filter: saturate(180%) blur(20px);
  backdrop-filter: saturate(180%) blur(20px);
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

.debug-tag {
  padding: 0 7px;
  border: 1px solid var(--line);
  border-radius: var(--radius-pill);
  color: var(--muted);
  font-size: 11px;
  line-height: 18px;
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
  max-width: 720px;
  margin: 0 auto;
  padding: 32px 24px 40px;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.home-title {
  margin: 0 0 120px;
  font-size: 40px;
  font-weight: 700;
  letter-spacing: -0.02em;
  line-height: 1.15;
  text-align: center;
  text-wrap: balance;
}

.cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 12px;
  width: 100%;
  max-width: 560px;
  margin: 0 auto;
}

.card {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  min-height: 104px;
  padding: 16px;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-lg);
  background: var(--surface);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04);
  text-align: left;
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease, box-shadow 0.15s ease;
}

.card:hover {
  background: var(--surface-2);
  border-color: rgba(0, 0, 0, 0.1);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
}

.card:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

.card h2 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--text);
}

.card p {
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--muted);
  line-height: 1.45;
}

.card.disabled {
  background: #fafafc;
  box-shadow: none;
}

.card.disabled h2,
.card.disabled p {
  color: #b0b0b5;
}

.card.disabled:hover {
  background: #fafafc;
  border-color: var(--line-soft);
  box-shadow: none;
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

.page-title {
  margin: 0;
  font-size: 32px;
  font-weight: 700;
  line-height: 1.2;
  letter-spacing: -0.02em;
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
.captcha-actions button,
.course-row button {
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
.captcha-actions button:hover:not(:disabled),
.course-row button:hover:not(:disabled) {
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
  word-break: break-all;
}

.course-list {
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
  margin: 8px 0;
}

.course-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  border-bottom: 1px solid #f5f6f7;
  font-size: 13px;
}

.course-row:last-child {
  border-bottom: none;
}

.course-index {
  flex: none;
  width: 22px;
  color: var(--muted);
}

.course-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.course-teacher {
  flex: none;
  width: 80px;
  color: var(--muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.course-current {
  flex: none;
  font-size: 12px;
  color: var(--primary);
}

.course-summary {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  margin: 10px 0;
  color: var(--muted);
  font-size: 13px;
}

.course-summary b {
  color: var(--text);
  font-weight: 600;
}

.chapter-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 8px;
}

.chapter {
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
  padding: 8px 12px;
}

.chapter summary {
  cursor: pointer;
  display: flex;
  gap: 12px;
  align-items: baseline;
}

.chapter-name {
  font-weight: 500;
}

.task-list {
  margin: 8px 0 0;
  padding: 0;
  list-style: none;
  max-height: 280px;
  overflow: auto;
}

.task-list li {
  display: flex;
  gap: 8px;
  align-items: baseline;
  padding: 3px 0;
  font-size: 13px;
}

.task-tag {
  flex: none;
  font-size: 11px;
  color: var(--muted);
  border: 1px solid var(--line-soft);
  border-radius: 3px;
  padding: 0 4px;
}

.task-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ok-text {
  color: #1a6b2f;
}

.warn-text {
  color: #a35c00;
}

/* 播放 */
.play-bar {
  margin: 12px 0 10px;
  padding: 12px;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
  background: var(--surface-2);
}

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

.play-controls .hint {
  margin: 0;
}

.play-status {
  margin-top: 10px;
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

/* 文档任务 */
.doc-section {
  margin-top: 10px;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
  padding: 8px 12px;
}

.doc-section summary {
  cursor: pointer;
  display: flex;
  gap: 12px;
  align-items: baseline;
}

.doc-actions {
  margin: 10px 0 4px;
}

.doc-actions .hint {
  margin: 0;
}

.doc-list {
  max-height: 240px;
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

.notice {
  position: fixed;
  left: 50%;
  bottom: 20px;
  transform: translateX(-50%);
  z-index: 50;
  max-width: 80%;
  padding: 8px 14px;
  border-radius: var(--radius-pill);
  background: rgba(29, 29, 31, 0.9);
  color: #ffffff;
  font-size: 13px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.18);
}

.notice.error {
  background: rgba(215, 0, 21, 0.92);
}

.notice.ok {
  background: rgba(26, 107, 47, 0.92);
}

@media (max-width: 680px) {
  .home-title {
    margin-bottom: 80px;
    font-size: 32px;
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
}

@media (prefers-reduced-motion: reduce) {
  .card {
    transition: none;
  }
}
</style>
