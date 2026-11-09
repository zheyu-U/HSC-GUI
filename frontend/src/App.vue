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
    qrImage.value = ''
    qrState.value = 'idle'
    qrMessage.value = ''
    checkMessage.value = ''
    setNotice(res.message, 'ok')
    await refreshSession()
    courseList.value = []
    courseDetail.value = null
    courseError.value = ''
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
  <div class="page">
    <header class="header">
      <div>
        <h1>hsc 课程助手</h1>
        <p class="subtitle">智慧华中大课程平台登录</p>
      </div>
      <div class="header-side">
        <span v-if="mode.debug" class="badge">debug 模式</span>
        <span class="version">v{{ mode.version }}</span>
      </div>
    </header>

    <main class="layout">
      <section class="card login-card">
        <div v-if="view === 'success'" class="success">
          <p class="success-title">{{ justLoggedIn ? '登录成功' : '已登录' }}</p>
          <p v-if="account" class="success-sub">已登录账号 {{ account }}</p>
          <p v-else class="success-sub">登录态已保存到本机</p>
          <p class="success-note">{{ session ? session.message : '' }}</p>
          <p class="success-tip">登录态和 hsc 命令行通用，可以直接在命令行里做后续操作。</p>
        </div>

        <template v-else>
          <div class="tabs">
            <button :class="{active: tab === 'qr'}" @click="tab = 'qr'">扫码登录</button>
            <button :class="{active: tab === 'pw'}" @click="tab = 'pw'">账号密码登录</button>
          </div>

          <div v-if="tab === 'qr'" class="pane">
            <p class="help">用企业微信或「华中大微校园」扫一扫，二维码 3 分钟内有效。</p>
            <div v-if="qrImage" class="qr-wrap">
              <img class="qr" :src="qrImage" alt="登录二维码"/>
              <p class="status">
                {{ qrMessage || '等待扫码' }}<span v-if="qrRemain > 0">，剩余 {{ qrRemain }} 秒</span>
              </p>
            </div>
            <div v-else class="qr-empty">
              <p>{{ qrBusy ? '正在获取二维码，请稍候' : '点下面的按钮获取二维码' }}</p>
            </div>
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
                <span v-else class="captcha-empty">验证码未获取</span>
              </div>
              <div class="captcha-side">
                <div class="captcha-actions">
                  <button @click="refreshCaptcha" :disabled="captchaBusy">{{ captchaBusy ? '获取中' : '换一张' }}</button>
                  <input v-model="captchaCode" class="captcha-input" type="text" maxlength="4" placeholder="4 位验证码"/>
                </div>
                <p class="captcha-hint">{{ captchaHint }}</p>
              </div>
            </div>
            <div class="actions">
              <button class="primary" @click="submitPassword" :disabled="loginBusy">{{ loginBusy ? '登录中' : '登录' }}</button>
            </div>
          </div>
        </template>
      </section>

      <aside class="card session-card">
        <h2>登录状态</h2>
        <p class="status-line">{{ session ? session.message : '读取中' }}</p>
        <dl class="facts">
          <div>
            <dt>账号</dt>
            <dd>{{ account || '未保存' }}</dd>
          </div>
          <div>
            <dt>本机登录态</dt>
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
              <dd class="path">{{ session ? session.stateDir : '' }}</dd>
            </div>
            <div>
              <dt>cookie 数量</dt>
              <dd>{{ session ? session.cookieCount : 0 }}</dd>
            </div>
          </template>
        </dl>
        <div class="actions wrap">
          <button @click="checkSession" :disabled="checkBusy">{{ checkBusy ? '检查中' : '检查会话' }}</button>
          <label class="check"><input v-model="logoutAll" type="checkbox"/> 同时删除已保存凭据</label>
          <button @click="doLogout" :disabled="logoutBusy || !loggedIn">{{ logoutBusy ? '退出中' : '退出登录' }}</button>
        </div>
        <p v-if="checkMessage" class="status">{{ checkMessage }}</p>
      </aside>
    </main>

    <section v-if="loggedIn" class="card course-card">
      <div class="course-head">
        <h2>我的课程</h2>
        <div class="actions">
          <button @click="loadCourses(true)" :disabled="courseLoading || courseBusy">
            {{ courseLoading ? '获取中' : '获取课程列表' }}
          </button>
          <button v-if="courseDetail && courseDetail.hasCourse" @click="refreshCourse" :disabled="courseBusy">
            {{ courseBusy ? '抓取中' : '抓取课程结构' }}
          </button>
        </div>
      </div>

      <p v-if="courseDetail" class="status-line">{{ courseDetail.message }}</p>
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

      <template v-if="courseDetail && courseDetail.hasCourse">
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
    </section>

    <section v-if="mode.debug" class="card debug-panel">
      <div class="debug-head">
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
      <p v-if="!debugLog.length" class="caption">暂无日志</p>

      <h3>原始会话状态</h3>
      <pre class="debug-json">{{ prettySession }}</pre>
    </section>

    <p v-if="notice" class="notice" :class="noticeKind">{{ notice }}</p>
    <footer class="footer">登录信息只保存在本机，和 hsc 命令行共用。</footer>
  </div>
</template>

<style scoped>
.page {
  max-width: 980px;
  margin: 0 auto;
  padding: 28px 20px 20px;
  box-sizing: border-box;
}

.header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
}

.header h1 {
  margin: 0;
  font-size: 24px;
  font-weight: 600;
}

.subtitle {
  margin: 4px 0 0;
  color: #646a73;
}

.header-side {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-top: 6px;
}

.badge {
  border: 1px solid #f5a623;
  border-radius: 10px;
  padding: 1px 8px;
  color: #a35c00;
  background: #fff7e6;
  font-size: 12px;
}

.version {
  color: #8f959e;
  font-size: 12px;
}

.layout {
  display: grid;
  grid-template-columns: minmax(0, 3fr) minmax(0, 2fr);
  gap: 16px;
  align-items: start;
}

.card {
  background: #ffffff;
  border: 1px solid #dee0e3;
  border-radius: 8px;
  padding: 16px;
  box-sizing: border-box;
}

.login-card {
  min-height: 360px;
}

.tabs {
  display: flex;
  gap: 8px;
  padding-bottom: 12px;
  border-bottom: 1px solid #eff0f1;
  margin-bottom: 16px;
}

.tabs button {
  border: none;
  background: transparent;
  padding: 6px 10px;
  color: #646a73;
}

.tabs button.active {
  color: #3370ff;
  border-bottom: 2px solid #3370ff;
  border-radius: 0;
}

.pane {
  min-height: 270px;
}

.help {
  margin: 0 0 16px;
  color: #646a73;
}

.qr-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 8px 0 16px;
}

.qr {
  width: 220px;
  height: 220px;
  border: 1px solid #dee0e3;
}

.qr-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 260px;
  color: #8f959e;
}

.status {
  margin: 0;
  color: #646a73;
}

.actions {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.actions.wrap {
  margin-top: 12px;
}

.success {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
  min-height: 320px;
  padding: 8px 12px;
}

.success-title {
  margin: 0 0 8px;
  font-size: 30px;
  font-weight: 600;
  color: #1a6b2f;
}

.success-sub {
  margin: 0 0 8px;
  font-size: 16px;
}

.success-note {
  margin: 0 0 8px;
  color: #646a73;
}

.success-tip {
  margin: 0;
  color: #8f959e;
  font-size: 13px;
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
  width: 120px;
  height: 44px;
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

.captcha-empty {
  color: #a8abb2;
  font-size: 12px;
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
  width: 120px;
}

.captcha-hint {
  margin: 6px 0 0;
  color: #8f959e;
  font-size: 12px;
  min-height: 18px;
}

.session-card h2 {
  margin: 0 0 8px;
  font-size: 16px;
  font-weight: 600;
}

.status-line {
  margin: 0 0 12px;
  color: #646a73;
}

.facts {
  margin: 0;
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
  width: 76px;
  color: #8f959e;
}

.facts dd {
  margin: 0;
  min-width: 0;
  word-break: break-all;
}

.facts dd.path {
  font-size: 12px;
  color: #646a73;
}

.check {
  display: flex;
  align-items: center;
  gap: 4px;
  color: #646a73;
  font-size: 12px;
}

.debug-panel {
  margin-top: 16px;
}

.debug-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.debug-head h2 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}

.debug-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
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
  width: 88px;
  color: #8f959e;
  font-size: 13px;
}

.debug-grid b {
  font-weight: 400;
  min-width: 0;
  word-break: break-all;
}

.debug-panel h3 {
  margin: 16px 0 8px;
  font-size: 14px;
  font-weight: 600;
}

.debug-log {
  margin: 0;
  padding: 0;
  list-style: none;
  max-height: 220px;
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

.caption {
  margin: 0;
  color: #8f959e;
  font-size: 12px;
}

.course-card {
  margin-top: 16px;
}

.course-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}

.course-head h2 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}

.course-list {
  border: 1px solid #eff0f1;
  border-radius: 4px;
  margin: 8px 0;
}

.course-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 6px 10px;
  border-bottom: 1px solid #f5f6f7;
  font-size: 13px;
}

.course-row:last-child {
  border-bottom: none;
}

.course-index {
  flex: none;
  width: 24px;
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
  width: 100px;
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
  gap: 16px;
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
  max-height: 300px;
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

@media (max-width: 760px) {
  .layout {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
