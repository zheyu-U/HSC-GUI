package hsc

// GUI 适配层：只做编排和展示，不复制上游协议逻辑。
// 上游文件来自 scripts/sync-upstream.ps1，重跑同步不会覆盖本文件。

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

var initOnce sync.Once
var initErr error

// InitState 初始化 state 目录。
// GUI 必须与 CLI 读写同一份 state：打包后的 exe 从快捷方式启动时工作目录
// 不一定是 exe 所在目录，没有项目标记（wails.json）时就切到 exe 目录。
func InitState() error {
	initOnce.Do(func() {
		if cwd, err := os.Getwd(); err == nil {
			if _, err := os.Stat(filepath.Join(cwd, "wails.json")); err != nil {
				if exe, e := os.Executable(); e == nil {
					_ = os.Chdir(filepath.Dir(exe))
				}
			}
		}
		initErr = initPaths()
	})
	return initErr
}

// StateDir 返回当前 state 目录，界面用来告诉用户登录态存在哪里。
func StateDir() string { return stateDir }

// LoginEvent 扫码登录进度，经 Wails Events 推给前端。
type LoginEvent struct {
	State   string `json:"state"`
	Message string `json:"message"`
}

// QRLoginResult 生成二维码的结果。
type QRLoginResult struct {
	Image     string `json:"image"`
	ExpiresAt int64  `json:"expiresAt"`
}

var qrMu sync.Mutex
var qrCancel chan struct{}
var qrRunning bool

func qrReset() {
	qrMu.Lock()
	qrRunning = false
	qrCancel = nil
	qrMu.Unlock()
}

// StartQRLogin 生成二维码并后台轮询登录态，进度通过 onEvent 推送。
// 二维码同时写到 state/auth/login-qr.png，与 CLI 路径一致。
func StartQRLogin(onEvent func(LoginEvent)) (QRLoginResult, error) {
	if err := InitState(); err != nil {
		return QRLoginResult{}, err
	}

	qrMu.Lock()
	if qrRunning {
		qrMu.Unlock()
		return QRLoginResult{}, fmt.Errorf("扫码登录已在进行，先取消当前二维码再重新生成")
	}
	qrRunning = true
	cancel := make(chan struct{})
	qrCancel = cancel
	qrMu.Unlock()

	emit := func(state, message string) {
		if onEvent != nil {
			onEvent(LoginEvent{State: state, Message: message})
		}
	}

	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		qrReset()
		return QRLoginResult{}, fmt.Errorf("生成二维码参数失败: %w", err)
	}
	uuid := hex.EncodeToString(buf)
	content := casURL + "/qyQrLogin?uuid=" + uuid + "&service="

	code, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		qrReset()
		return QRLoginResult{}, fmt.Errorf("生成二维码失败: %w", err)
	}
	png, err := code.PNG(320)
	if err != nil {
		qrReset()
		return QRLoginResult{}, fmt.Errorf("生成二维码图片失败: %w", err)
	}
	if err := os.WriteFile(statePath("login-qr.png"), png, 0o644); err != nil {
		qrReset()
		return QRLoginResult{}, fmt.Errorf("写入二维码文件失败: %w", err)
	}

	go func() {
		defer qrReset()
		emit("waiting", "请用企业微信或华中大微校园扫码")

		c := newLoggedClient()
		redirect := ""
		for i := 0; i < 60; i++ {
			select {
			case <-cancel:
				emit("cancelled", "已取消扫码登录")
				return
			default:
			}
			u := fmt.Sprintf("%s/checkQRCodeScan?random=%d&uuid=%s", casURL, nowNano()/1e9, uuid)
			_, _, body, err := c.Follow(u, map[string]string{
				"X-Requested-With": "XMLHttpRequest",
				"Accept":           "application/json, text/javascript, */*; q=0.01",
				"Referer":          casURL + "/login",
			})
			if err == nil && len(strings.TrimSpace(string(body))) > 0 {
				var r struct {
					RedirectURL string `json:"redirect_url"`
				}
				if json.Unmarshal(body, &r) == nil && r.RedirectURL != "" {
					redirect = r.RedirectURL
					emit("scanned", "已扫码，正在换取登录态")
					break
				}
			}
			select {
			case <-cancel:
				emit("cancelled", "已取消扫码登录")
				return
			case <-time.After(3 * time.Second):
			}
		}

		if redirect == "" {
			emit("expired", "二维码已失效，请重新生成")
			return
		}
		if _, _, _, err := c.Follow(redirect, nil); err != nil {
			emit("error", "换取登录态失败: "+err.Error())
			return
		}
		if err := finishLogin(c, Target{}); err != nil {
			emit("error", "登录失败: "+err.Error())
			return
		}
		emit("success", "登录成功，登录态已保存")
	}()

	return QRLoginResult{
		Image:     "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
		ExpiresAt: time.Now().Add(3 * time.Minute).Unix(),
	}, nil
}

// CancelQRLogin 取消正在进行的扫码轮询。
func CancelQRLogin() {
	qrMu.Lock()
	if qrCancel != nil {
		close(qrCancel)
		qrCancel = nil
	}
	qrMu.Unlock()
}

// CaptchaView 一次验证码会话。
type CaptchaView struct {
	Image         string  `json:"image"`
	SuggestedCode string  `json:"suggestedCode"`
	Confidence    float64 `json:"confidence"`
	Count         int     `json:"count"`
	Hint          string  `json:"hint"`
}

var pwMu sync.Mutex
var pwCurrent *pwSession

type pwSession struct {
	c         *Client
	lt        string
	execution string
	autoCode  string
}

// StartCaptcha 打开登录页并取一张新验证码，自动识别结果作为建议值。
func StartCaptcha() (CaptchaView, error) {
	if err := InitState(); err != nil {
		return CaptchaView{}, err
	}

	c := newLoggedClient()
	lt, execution, err := fetchLoginPage(c)
	if err != nil {
		return CaptchaView{}, fmt.Errorf("打开登录页失败: %w", err)
	}
	img, err := fetchCaptcha(c)
	if err != nil {
		return CaptchaView{}, fmt.Errorf("获取验证码失败: %w", err)
	}

	mime := "image/gif"
	ext := "gif"
	if len(img) < 3 || string(img[:3]) != "GIF" {
		mime = "image/png"
		ext = "png"
	}
	// 与 CLI 一样落盘一份，方便排查
	_ = os.WriteFile(statePath("login-captcha."+ext), img, 0o644)

	view := CaptchaView{
		Image: "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(img),
	}
	sess := &pwSession{c: c, lt: lt, execution: execution}
	if res, rerr := recognizeCaptcha(img); rerr == nil {
		view.SuggestedCode = res.Code
		view.Confidence = res.Conf
		view.Count = res.Count
		sess.autoCode = res.Code
		if res.Count != 4 || res.Conf < minCaptchaConf {
			view.Hint = "自动识别把握不大，请核对图片后输入，或点刷新换一张"
		}
	} else {
		view.Hint = "自动识别失败，请人工输入验证码"
	}

	pwMu.Lock()
	pwCurrent = sess
	pwMu.Unlock()
	return view, nil
}

// LoginResult 账密登录结果。
type LoginResult struct {
	OK        bool   `json:"ok"`
	Message   string `json:"message"`
	CredSaved bool   `json:"credSaved"`
	Need2FA   bool   `json:"need2fa"`
}

// LoginPassword 提交账号密码和验证码；成功后加密保存凭据（与 CLI cred save 同格式）。
func LoginPassword(username, password, code string) (LoginResult, error) {
	if err := InitState(); err != nil {
		return LoginResult{}, err
	}
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return LoginResult{}, fmt.Errorf("账号和密码不能为空")
	}

	pwMu.Lock()
	sess := pwCurrent
	pwMu.Unlock()
	if sess == nil {
		return LoginResult{}, fmt.Errorf("验证码会话已失效，请刷新验证码")
	}

	code = strings.TrimSpace(code)
	if code == "" {
		code = sess.autoCode
	}
	if len(code) != 4 {
		return LoginResult{}, fmt.Errorf("验证码是 4 位数字，请按图片输入")
	}

	res, err := submitLogin(sess.c, sess.lt, sess.execution, username, password, code)
	if err != nil {
		return LoginResult{}, fmt.Errorf("提交登录失败: %w", err)
	}
	pwMu.Lock()
	pwCurrent = nil
	pwMu.Unlock()

	if !res.OK {
		if res.Need2FA {
			return LoginResult{Message: "该账号需要企业微信二次验证，本版本不支持自动处理", Need2FA: true}, nil
		}
		msg := res.Message
		if msg == "" {
			msg = "账号或密码不正确，或验证码已过期"
		}
		return LoginResult{Message: "登录失败: " + msg}, nil
	}

	if err := finishLogin(sess.c, Target{}); err != nil {
		return LoginResult{Message: "换取登录态失败: " + err.Error()}, nil
	}

	out := LoginResult{OK: true, Message: "登录成功"}
	if err := saveCreds(username, password); err != nil {
		out.Message = "登录成功，但保存凭据失败: " + err.Error()
	} else {
		out.CredSaved = true
	}
	return out, nil
}

// SessionInfo 本地会话状态（不发起网络请求）。
type SessionInfo struct {
	LoggedIn      bool   `json:"loggedIn"`
	CookieCount   int    `json:"cookieCount"`
	Account       string `json:"account"`
	CredSaved     bool   `json:"credSaved"`
	CredEncrypted bool   `json:"credEncrypted"`
	HasTarget     bool   `json:"hasTarget"`
	StateDir      string `json:"stateDir"`
	Message       string `json:"message"`
}

// SessionStatus 读取本地登录态与凭据状态。
func SessionStatus() (SessionInfo, error) {
	if err := InitState(); err != nil {
		return SessionInfo{}, err
	}
	info := SessionInfo{StateDir: stateDir}
	jar := NewJar()
	if err := jar.Load(); err == nil {
		info.CookieCount = jar.Count()
	}
	info.LoggedIn = info.CookieCount > 0

	if c, err := loadEncryptedCreds(); err == nil {
		info.CredSaved = true
		info.CredEncrypted = true
		info.Account = maskUser(c.Username)
	} else if fileExists(statePath(txtCredFile)) {
		info.CredSaved = true
		if c, err := loadPlainCreds(); err == nil {
			info.Account = maskUser(c.Username)
		}
	}
	if _, ok := loadTargetIfExists(); ok {
		info.HasTarget = true
	}

	switch {
	case info.LoggedIn && info.HasTarget:
		info.Message = "已登录，已配置课程，可做站点校验"
	case info.LoggedIn:
		info.Message = "已登录；未配置课程，只能确认本地登录态"
	case info.CredSaved:
		info.Message = "未登录；已保存凭据，可用账密登录"
	default:
		info.Message = "未登录；可用扫码或账密登录"
	}
	return info, nil
}

// CheckResult 站点校验结果。
type CheckResult struct {
	OK        bool   `json:"ok"`
	Checked   bool   `json:"checked"`
	HasTarget bool   `json:"hasTarget"`
	Chapters  int    `json:"chapters"`
	Message   string `json:"message"`
}

// CheckSession 用当前 cookie 做站点校验；没有课程配置时只确认本地登录态。
func CheckSession() (CheckResult, error) {
	if err := InitState(); err != nil {
		return CheckResult{}, err
	}
	c := newClient(NewJar())
	if err := loadSession(c); err != nil {
		return CheckResult{OK: false, Message: err.Error()}, nil
	}
	t, ok := loadTargetIfExists()
	if !ok {
		return CheckResult{
			OK:      true,
			Checked: false,
			Message: "本地登录态存在；未配置课程，未做站点校验（CLI: hsc check）",
		}, nil
	}
	chapters, size, err := verifySite(c, t)
	if err != nil {
		return CheckResult{OK: false, Checked: true, HasTarget: true, Message: "站点校验失败: " + err.Error()}, nil
	}
	return CheckResult{
		OK:        true,
		Checked:   true,
		HasTarget: true,
		Chapters:  chapters,
		Message:   fmt.Sprintf("会话有效，章节页 %d 字节，%d 个章节", size, chapters),
	}, nil
}

// LogoutResult 登出结果。
type LogoutResult struct {
	Removed []string `json:"removed"`
	Message string   `json:"message"`
}

// Logout 清除登录态；all 为 true 时连加密凭据一起清（对应 CLI 的 logout --all）。
func Logout(all bool) (LogoutResult, error) {
	if err := InitState(); err != nil {
		return LogoutResult{}, err
	}

	jar := NewJar()
	if err := jar.Load(); err == nil && jar.Count() > 0 {
		c := newClient(jar)
		_, _, _, _ = c.Follow(casURL+"/logout", nil)
	}

	var removed []string
	for _, name := range []string{"cookies.json", "visitor.json"} {
		if fileExists(statePath(name)) && os.Remove(statePath(name)) == nil {
			removed = append(removed, name)
		}
	}
	if all {
		removed = append(removed, forgetCreds()...)
	}

	out := LogoutResult{Removed: removed}
	switch {
	case len(removed) == 0:
		out.Message = "没有可清除的登录态"
	case all:
		out.Message = "已清除登录态和已保存的凭据"
	default:
		out.Message = "已清除登录态，凭据保留，下次账密登录可直接使用"
	}
	return out, nil
}
