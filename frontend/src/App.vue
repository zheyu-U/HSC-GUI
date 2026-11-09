<script setup>
import {computed, onMounted, onUnmounted, ref} from 'vue'
import {
  CancelQRLogin,
  CheckSession,
  LoginPassword,
  Logout,
  SessionStatus,
  StartCaptcha,
  StartQRLogin,
} from '../wailsjs/go/main/App'
import {EventsOff, EventsOn} from '../wailsjs/runtime/runtime'

const tab = ref('qr')
const session = ref(null)
const notice = ref('')
const noticeKind = ref('info')

const qrImage = ref('')
const qrState = ref('idle')
const qrMessage = ref('')
const qrRemain = ref(0)
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
const logoutAll = ref(false)

const loggedIn = computed(() => !!(session.value && session.value.loggedIn))

function setNotice(message, kind) {
  notice.value = message || ''
  noticeKind.value = kind || 'info'
}

async function refreshSession() {
  try {
    session.value = await SessionStatus()
  } catch (e) {
    setNotice('读取登录态失败: ' + String(e), 'error')
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

async function generateQr() {
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
    qrMessage.value = String(e)
  }
}

async function cancelQr() {
  try {
    await CancelQRLogin()
  } catch (e) {
    setNotice('取消扫码失败: ' + String(e), 'error')
  }
  stopQrTimer()
  qrState.value = 'idle'
  qrRemain.value = 0
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
    refreshSession()
  }
  if (ev.state === 'expired' || ev.state === 'error' || ev.state === 'cancelled') {
    stopQrTimer()
  }
}

async function refreshCaptcha() {
  captchaBusy.value = true
  try {
    const res = await StartCaptcha()
    captchaImage.value = res.image
    captchaCode.value = res.suggestedCode || ''
    captchaHint.value = res.hint || (res.suggestedCode
      ? '自动识别建议 ' + res.suggestedCode + '，可直接提交或手动修改'
      : '')
  } catch (e) {
    captchaImage.value = ''
    captchaHint.value = ''
    setNotice('获取验证码失败: ' + String(e), 'error')
  } finally {
    captchaBusy.value = false
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
      setNotice(res.message, 'ok')
      await refreshSession()
    } else {
      setNotice(res.message, 'error')
      if (!res.need2fa) {
        await refreshCaptcha()
      }
    }
  } catch (e) {
    setNotice(String(e), 'error')
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
    checkMessage.value = String(e)
  } finally {
    checkBusy.value = false
  }
}

async function doLogout() {
  try {
    const res = await Logout(logoutAll.value)
    setNotice(res.message, 'ok')
    checkMessage.value = ''
    await refreshSession()
  } catch (e) {
    setNotice('退出登录失败: ' + String(e), 'error')
  }
}

let offQr = null

onMounted(() => {
  refreshSession()
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
      <h1>hsc 课程助手</h1>
      <p class="subtitle">智慧华中大课程平台登录</p>
    </header>

    <main class="layout">
      <section class="card login-card">
        <div class="tabs">
          <button :class="{active: tab === 'qr'}" @click="tab = 'qr'">扫码登录</button>
          <button :class="{active: tab === 'pw'}" @click="tab = 'pw'">账号密码登录</button>
        </div>

        <div v-if="tab === 'qr'" class="pane">
          <p class="help">用企业微信或「华中大微校园」扫码，二维码 3 分钟内有效。</p>
          <div v-if="qrImage" class="qr-wrap">
            <img class="qr" :src="qrImage" alt="登录二维码"/>
            <p class="status">
              {{ qrMessage || '等待扫码' }}<span v-if="qrRemain > 0">，剩余 {{ qrRemain }} 秒</span>
            </p>
          </div>
          <div class="actions">
            <button class="primary" @click="generateQr">{{ qrImage ? '重新生成二维码' : '生成二维码' }}</button>
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
              <img v-if="captchaImage" class="captcha" :src="captchaImage" alt="验证码" title="点击刷新" @click="refreshCaptcha"/>
              <span v-else class="captcha-empty">验证码未获取</span>
            </div>
            <div class="captcha-side">
              <div class="captcha-actions">
                <button @click="refreshCaptcha" :disabled="captchaBusy">{{ captchaBusy ? '获取中' : '刷新验证码' }}</button>
                <input v-model="captchaCode" class="captcha-input" type="text" maxlength="4" placeholder="4 位验证码"/>
              </div>
              <p class="captcha-hint">{{ captchaHint }}</p>
            </div>
          </div>
          <div class="actions">
            <button class="primary" @click="submitPassword" :disabled="loginBusy">{{ loginBusy ? '登录中' : '登录' }}</button>
          </div>
        </div>
      </section>

      <aside class="card session-card">
        <h2>会话状态</h2>
        <p class="status-line">{{ session ? session.message : '读取中' }}</p>
        <dl v-if="session" class="facts">
          <div>
            <dt>账号</dt>
            <dd>{{ session.account || '未保存' }}</dd>
          </div>
          <div>
            <dt>本地登录态</dt>
            <dd>{{ loggedIn ? '存在' : '无' }}</dd>
          </div>
          <div>
            <dt>凭据</dt>
            <dd>{{ session.credSaved ? (session.credEncrypted ? '已加密保存' : '明文保存') : '无' }}</dd>
          </div>
          <div>
            <dt>课程配置</dt>
            <dd>{{ session.hasTarget ? '已配置' : '未配置' }}</dd>
          </div>
          <div>
            <dt>state 目录</dt>
            <dd class="path">{{ session.stateDir }}</dd>
          </div>
        </dl>
        <div class="actions wrap">
          <button @click="checkSession" :disabled="checkBusy">{{ checkBusy ? '检查中' : '检查会话' }}</button>
          <label class="check"><input v-model="logoutAll" type="checkbox"/> 同时删除已保存凭据</label>
          <button @click="doLogout">退出登录</button>
        </div>
        <p v-if="checkMessage" class="status">{{ checkMessage }}</p>
      </aside>
    </main>

    <p v-if="notice" class="notice" :class="noticeKind">{{ notice }}</p>
    <footer class="footer">登录态与 hsc 命令行通用，保存在 state 目录下。</footer>
  </div>
</template>

<style scoped>
.page {
  max-width: 960px;
  margin: 0 auto;
  padding: 32px 20px 20px;
  box-sizing: border-box;
}

.header {
  margin-bottom: 24px;
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
  min-height: 300px;
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
  margin-top: 24px;
  color: #8f959e;
  font-size: 12px;
}

@media (max-width: 760px) {
  .layout {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
