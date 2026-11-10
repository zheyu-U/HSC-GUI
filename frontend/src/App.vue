<script setup>
import {computed, onMounted, onUnmounted, ref, watch} from 'vue'
import {
  AppMode,
  CancelQRLogin,
  CheckSession,
  ClearDebugLog,
  DebugInfo,
  DebugLog,
  GetCourseDetail,
  ListCourses,
  LoginPassword,
  Logout,
  RefreshCourse,
  SelectCourse,
  SessionStatus,
  StartCaptcha,
  StartQRLogin,
} from '../wailsjs/go/main/App'
import {EventsOff, EventsOn} from '../wailsjs/runtime/runtime'

const page = ref('login')
const mode = ref({mode: 'release', debug: false, version: ''})
const tab = ref('qr')
const view = ref('login')
const justLoggedIn = ref(false)
const session = ref(null)
const notice = ref('')
const noticeKind = ref('info')

const qrImage = ref('')
const qrState = ref('idle')
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
const logoutBusy = ref(false)
const checkMessage = ref('')
const logoutAll = ref(false)

const courseList = ref([])
const courseDetail = ref(null)
const courseLoading = ref(false)
const courseBusy = ref(false)
const courseError = ref('')

const debugInfo = ref(null)
const debugLog = ref([])
const debugBusy = ref(false)

const loggedIn = computed(() => !!(session.value && session.value.loggedIn))
const account = computed(() => (session.value && session.value.account) || '')
const hasCourse = computed(() => !!(courseDetail.value && courseDetail.value.hasCourse))
const prettySession = computed(() => (session.value ? JSON.stringify(session.value, null, 2) : ''))
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

function friendly(e) {
  return String(e).replace(/^Error:\s*/, '')
}

function setNotice(message, kind) {
  notice.value = message || ''
  noticeKind.value = kind || 'info'
}

function showPage(name) {
  if (name === 'debug' && !mode.value.debug) {
    return
  }
  page.value = name
  setNotice('')
  if (name === 'course' && loggedIn.value) {
    if (!courseDetail.value) {
      loadCourseDetail()
    }
    if (!courseList.value.length && !courseLoading.value) {
      loadCourses(true)
    }
  }
  if (name === 'debug') {
    refreshDebug()
  }
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
    const s = await SessionStatus()
    session.value = s
    if (s.loggedIn && view.value !== 'success') {
      view.value = 'success'
      justLoggedIn.value = false
    }
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
    // 忽略清空失败，下面刷新会显示实际情况
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
    qrState.value = 'waiting'
    qrRemain.value = Math.max(0, res.expiresAt - Math.floor(Date.now() / 1000))
    startQrTimer()
  } catch (e) {
    qrState.value = 'error'
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
  qrState.value = 'idle'
  qrRemain.value = 0
  qrImage.value = ''
  qrMessage.value = ''
}

function onQrEvent(ev) {
  qrState.value = ev.state
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
  page.value = 'login'
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
  logoutBusy.value = true
  try {
    const res = await Logout(logoutAll.value)
    justLoggedIn.value = false
    view.value = 'login'
    page.value = 'login'
    qrImage.value = ''
    qrState.value = 'idle'
    qrMessage.value = ''
    checkMessage.value = ''
    courseList.value = []
    courseDetail.value = null
    courseError.value = ''
    setNotice(res.message, 'ok')
    await refreshSession()
    if (!loggedIn.value) {
      generateQr()
    }
  } catch (e) {
    setNotice('退出登录失败: ' + friendly(e), 'error')
  } finally {
    logoutBusy.value = false
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

async function bootstrap() {
  try {
    mode.value = await AppMode()
  } catch (e) {
    // AppMode 不可用时按 release 处理，保证界面能显示
  }
  await refreshSession()
  if (loggedIn.value) {
    loadCourseDetail()
  } else {
    generateQr()
  }
  refreshDebug()
}

let offQr = null

watch(tab, (val) => {
  if (val === 'pw' && !captchaImage.value) {
    refreshCaptcha()
  }
})

onMounted(() => {
  bootstrap()
  offQr = EventsOn('login:qr', onQrEvent)
})

onUnmounted(() => {
  stopQrTimer()
  if (typeof offQr === 'function') {
    offQr()
  } else {
    EventsOff('login:qr')
  }
})
</script>

<template>
  <div class="app">
    <header class="topbar">
      <div class="brand">
        <span class="logo">hsc</span>
        <span class="brand-name">课程助手</span>
        <span class="version">v{{ mode.version }}</span>
        <span v-if="mode.debug" class="badge">debug</span>
      </div>
      <nav class="nav">
        <button :class="{active: page === 'login'}" @click="showPage('login')">登录</button>
        <button :class="{active: page === 'course'}" @click="showPage('course')">课程</button>
        <button v-if="mode.debug" :class="{active: page === 'debug'}" @click="showPage('debug')">调试</button>
      </nav>
    </header>

    <main class="content">
      <!-- 登录页 -->
      <section v-if="page === 'login'" class="card">
        <template v-if="view === 'success'">
          <p class="page-title">{{ justLoggedIn ? '登录成功' : '已登录' }}</p>
          <p class="page-sub">{{ account ? '账号 ' + account : '登录态已保存到本机' }}</p>

          <dl class="facts">
            <div>
              <dt>登录状态</dt>
              <dd>{{ loggedIn ? '有效' : '无' }}</dd>
            </div>
            <div>
              <dt>记住账号</dt>
              <dd>{{ session && session.credSaved ? '已保存' : '未保存' }}</dd>
            </div>
            <div>
              <dt>课程信息</dt>
              <dd>{{ session && session.hasTarget ? '已配置课程' : '未配置课程' }}</dd>
            </div>
            <template v-if="mode.debug">
              <div>
                <dt>state 目录</dt>
                <dd class="mono">{{ session ? session.stateDir : '' }}</dd>
              </div>
              <div>
                <dt>cookie 数量</dt>
                <dd>{{ session ? session.cookieCount : 0 }}</dd>
              </div>
            </template>
          </dl>

          <div class="actions">
            <button class="primary" @click="showPage('course')">进入课程</button>
            <button @click="checkSession" :disabled="checkBusy">{{ checkBusy ? '检查中' : '检查会话' }}</button>
            <button @click="doLogout" :disabled="logoutBusy">{{ logoutBusy ? '退出中' : '退出登录' }}</button>
          </div>
          <label class="check"><input v-model="logoutAll" type="checkbox"/> 同时删除已保存凭据</label>
          <p v-if="checkMessage" class="muted">{{ checkMessage }}</p>
        </template>

        <template v-else>
          <div class="tabs">
            <button :class="{active: tab === 'qr'}" @click="tab = 'qr'">扫码登录</button>
            <button :class="{active: tab === 'pw'}" @click="tab = 'pw'">账号密码登录</button>
          </div>

          <div v-if="tab === 'qr'" class="pane">
            <p class="hint">用企业微信或「华中大微校园」扫一扫，二维码 3 分钟内有效。</p>
            <div v-if="qrImage" class="qr-wrap">
              <img class="qr" :src="qrImage" alt="登录二维码"/>
              <p class="muted">
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
                <span v-else class="muted">验证码未获取</span>
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
      </section>

      <!-- 课程页 -->
      <section v-else-if="page === 'course'" class="card">
        <template v-if="!loggedIn">
          <p class="page-title">还没有登录</p>
          <p class="hint">登录后才能查看课程列表和任务点。</p>
          <div class="actions">
            <button class="primary" @click="showPage('login')">去登录</button>
          </div>
        </template>

        <template v-else>
          <div class="page-head">
            <h2>我的课程</h2>
            <div class="actions">
              <button @click="loadCourses(true)" :disabled="courseLoading || courseBusy">
                {{ courseLoading ? '获取中' : '获取课程列表' }}
              </button>
              <button v-if="hasCourse" @click="refreshCourse" :disabled="courseBusy">
                {{ courseBusy ? '抓取中' : '抓取课程结构' }}
              </button>
            </div>
          </div>

          <p v-if="courseDetail" class="hint">{{ courseDetail.message }}</p>
          <p v-if="courseError" class="inline-error">{{ courseError }}</p>

          <div v-if="courseList.length" class="course-list">
            <div v-for="item in courseList" :key="item.courseId" class="course-row">
              <span class="course-index">{{ item.index }}</span>
              <span class="course-name">{{ item.name || '未命名课程' }}</span>
              <span class="course-teacher">{{ item.teacher }}</span>
              <span v-if="isCurrent(item)" class="course-current">当前</span>
              <button @click="selectCourse(item.index)" :disabled="courseBusy || courseLoading">选择</button>
            </div>
          </div>

          <template v-if="hasCourse">
            <div class="course-summary">
              <span>章节 <b>{{ courseDetail.summary.chapters }}</b></span>
              <span>视频 <b>{{ courseDetail.summary.videosPassed }}/{{ courseDetail.summary.videos }}</b></span>
              <span>作业 <b>{{ courseDetail.summary.works }}</b></span>
              <span>总时长 <b>{{ courseDetail.summary.durationHours }}</b> 小时</span>
              <span v-if="courseDetail.summary.speedLimited">限 1 倍速 <b>{{ courseDetail.summary.speedLimited }}</b> 个</span>
              <span v-if="courseDetail.stale" class="warn-text">缓存属于另一门课</span>
            </div>

            <div class="chapter-list">
              <details v-for="ch in courseDetail.chapters" :key="ch.id" class="chapter" :open="ch.index <= 2">
                <summary>
                  <span class="chapter-name">{{ ch.title || ('章节 ' + ch.index) }}</span>
                  <span class="muted">视频 {{ ch.videosPassed }}/{{ ch.videos }}，作业 {{ ch.works }}</span>
                </summary>
                <ul class="task-list">
                  <li v-for="(v, index) in videosByChapter[ch.id] || []" :key="'v' + ch.id + index">
                    <span class="task-tag">视频</span>
                    <span class="task-title">{{ v.title || '未命名视频' }}</span>
                    <span class="muted">{{ formatDuration(v.duration) }}</span>
                    <span v-if="v.isPassed" class="ok-text">已完成</span>
                    <span v-else class="muted">未完成</span>
                    <span v-if="v.speedLimited" class="muted">限 1 倍速</span>
                  </li>
                  <li v-for="(w, index) in worksByChapter[ch.id] || []" :key="'w' + ch.id + index">
                    <span class="task-tag">作业</span>
                    <span class="task-title">{{ w.title || '未命名作业' }}</span>
                  </li>
                  <li v-if="!((videosByChapter[ch.id] || []).length || (worksByChapter[ch.id] || []).length)" class="muted">
                    本章没有可展示的任务点
                  </li>
                </ul>
              </details>
            </div>
          </template>
        </template>
      </section>

      <!-- 调试页（仅 debug 构建） -->
      <section v-else-if="page === 'debug' && mode.debug" class="card">
        <div class="page-head">
          <h2>调试信息</h2>
          <div class="actions">
            <button @click="refreshDebug" :disabled="debugBusy">{{ debugBusy ? '刷新中' : '刷新' }}</button>
            <button @click="clearDebug">清空日志</button>
          </div>
        </div>

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
      </section>
    </main>

    <p v-if="notice" class="notice" :class="noticeKind">{{ notice }}</p>
    <footer class="footer">登录信息只保存在本机，和 hsc 命令行共用。</footer>
  </div>
</template>

<style scoped>
.app {
  max-width: 760px;
  margin: 0 auto;
  padding: 20px 16px 24px;
  box-sizing: border-box;
}

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid #dee0e3;
}

.brand {
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.logo {
  font-size: 20px;
  font-weight: 700;
  color: #3370ff;
}

.brand-name {
  font-size: 15px;
  font-weight: 600;
}

.version {
  color: #8f959e;
  font-size: 12px;
}

.badge {
  border: 1px solid #f5a623;
  border-radius: 10px;
  padding: 0 6px;
  color: #a35c00;
  background: #fff7e6;
  font-size: 12px;
}

.nav {
  display: flex;
  gap: 4px;
}

.nav button {
  border: none;
  background: transparent;
  padding: 6px 12px;
  border-radius: 4px;
  color: #646a73;
}

.nav button:hover:not(:disabled) {
  background: #f2f3f5;
  color: #1f2329;
}

.nav button.active {
  background: #eaf1ff;
  color: #3370ff;
  font-weight: 500;
}

.content {
  margin-top: 16px;
}

.card {
  background: #ffffff;
  border: 1px solid #dee0e3;
  border-radius: 8px;
  padding: 20px;
  box-sizing: border-box;
}

.page-title {
  margin: 0 0 4px;
  font-size: 26px;
  font-weight: 600;
}

.page-sub {
  margin: 0 0 16px;
  color: #646a73;
}

.page-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}

.page-head h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
}

.tabs {
  display: flex;
  gap: 4px;
  margin-bottom: 16px;
}

.tabs button {
  border: none;
  background: transparent;
  padding: 6px 12px;
  border-radius: 4px;
  color: #646a73;
}

.tabs button.active {
  background: #eaf1ff;
  color: #3370ff;
  font-weight: 500;
}

.pane {
  min-height: 260px;
}

.hint {
  margin: 0 0 12px;
  color: #8f959e;
  font-size: 13px;
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
  border: 1px solid #dee0e3;
}

.qr-empty {
  margin: 24px 0;
  text-align: center;
  color: #8f959e;
}

.actions {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.actions button {
  flex: none;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 12px;
}

.field label {
  color: #646a73;
}

.field input {
  width: 100%;
  box-sizing: border-box;
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
  border: 1px solid #dee0e3;
  border-radius: 4px;
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
}

.facts {
  margin: 0 0 16px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.facts > div {
  display: flex;
  gap: 12px;
}

.facts dt {
  flex: none;
  width: 80px;
  color: #8f959e;
}

.facts dd {
  margin: 0;
  min-width: 0;
  word-break: break-all;
}

.check {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 10px;
  color: #646a73;
  font-size: 12px;
}

.course-list {
  border: 1px solid #eff0f1;
  border-radius: 4px;
  margin: 10px 0;
}

.course-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 10px;
  border-bottom: 1px solid #f5f6f7;
  font-size: 13px;
}

.course-row:last-child {
  border-bottom: none;
}

.course-index {
  flex: none;
  width: 22px;
  color: #8f959e;
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
  color: #646a73;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.course-current {
  flex: none;
  font-size: 12px;
  color: #3370ff;
}

.course-summary {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  margin: 10px 0;
  color: #646a73;
  font-size: 13px;
}

.course-summary b {
  color: #1f2329;
  font-weight: 600;
}

.chapter-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 8px;
}

.chapter {
  border: 1px solid #eff0f1;
  border-radius: 4px;
  padding: 6px 10px;
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
  font-size: 12px;
  color: #8f959e;
  border: 1px solid #dee0e3;
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

.muted {
  color: #8f959e;
}

.ok-text {
  color: #1a6b2f;
}

.warn-text {
  color: #a35c00;
}

.inline-error {
  margin: 8px 0;
  padding: 6px 10px;
  border-radius: 4px;
  background: #fde2e2;
  color: #a61b1b;
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
  color: #8f959e;
  font-size: 13px;
}

.debug-grid b {
  font-weight: 400;
  min-width: 0;
  word-break: break-all;
}

.card h3 {
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
  border: 1px solid #eff0f1;
  border-radius: 4px;
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
  color: #8f959e;
}

.mono {
  font-family: Consolas, "Cascadia Mono", monospace;
}

.debug-json {
  margin: 0;
  padding: 8px;
  max-height: 220px;
  overflow: auto;
  background: #f7f8fa;
  border-radius: 4px;
  font-size: 12px;
  white-space: pre-wrap;
}

.notice {
  margin: 16px 0 0;
  padding: 8px 12px;
  border-radius: 4px;
  background: #f2f3f5;
  color: #1f2329;
}

.notice.error {
  background: #fde2e2;
  color: #a61b1b;
}

.notice.ok {
  background: #e4f7e8;
  color: #1a6b2f;
}

.footer {
  margin-top: 20px;
  color: #8f959e;
  font-size: 12px;
}

@media (max-width: 600px) {
  .topbar {
    flex-direction: column;
    align-items: flex-start;
  }

  .course-teacher {
    display: none;
  }
}
</style>
