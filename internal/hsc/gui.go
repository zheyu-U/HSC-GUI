package hsc

// GUI 适配层：只做编排和展示，不复制上游协议逻辑。
// 上游文件来自 scripts/sync-upstream.ps1，重跑同步不会覆盖本文件。

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
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
// GUI 必须与 CLI 读写同一份 state：打包后的 exe 一律切到 exe 所在目录，
// 从快捷方式、终端或项目目录启动都落到同一份 state；只有 wails dev（dev 构建标签）
// 留在项目根目录，方便本地调试。
func InitState() error {
	initOnce.Do(func() {
		cwd, _ := os.Getwd()
		exePath, _ := os.Executable()
		if root := resolveStateRoot(devMode, cwd, exePath); root != "" {
			_ = os.Chdir(root)
		}
		initErr = initPaths()
	})
	return initErr
}

// resolveStateRoot 决定 state 目录放在哪个目录：
// 开发模式用带 wails.json 的项目根目录，打包后一律用 exe 所在目录。
// 这样 exe 从任意工作目录启动都不会读到另一份 state。
func resolveStateRoot(dev bool, cwd, exePath string) string {
	if !dev {
		if exePath != "" {
			return filepath.Dir(exePath)
		}
		return cwd
	}
	if root, ok := findProjectRoot(cwd); ok {
		return root
	}
	return cwd
}

// findProjectRoot 从 dir 起向上找带 wails.json 的目录（开发模式用）。
func findProjectRoot(dir string) (string, bool) {
	if dir == "" {
		return "", false
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "wails.json")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
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
	AccountTail   string `json:"accountTail"`
	CredSaved     bool   `json:"credSaved"`
	CredEncrypted bool   `json:"credEncrypted"`
	HasTarget     bool   `json:"hasTarget"`
	StateDir      string `json:"stateDir"`
	Message       string `json:"message"`
}

// tailRunes 取字符串最后 n 个字符，用于顶栏只显示账号尾号。
func tailRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[len(r)-n:])
}

// accountFromCookies 从登录 cookie 里取账号：扫码登录不保存凭据，
// 但 hust.edu.cn 的 uname 里有账号，用它补上顶栏尾号和菜单里的账号。
func accountFromCookies(j *Jar) (string, string) {
	u := url.URL{Scheme: "https", Host: "hust.edu.cn", Path: "/"}
	for _, c := range j.Cookies(&u) {
		if c.Name != "uname" || !looksLikeAccount(c.Value) {
			continue
		}
		return maskUser(c.Value), tailRunes(c.Value, 3)
	}
	return "", ""
}

// looksLikeAccount 只接受 ASCII 账号（字母、数字、点、下划线、@、-）。
// 有些账号的 uname 是中文姓名或其他展示值，不能当账号显示尾号。
func looksLikeAccount(s string) bool {
	if len(s) < 4 || len(s) > 32 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.', r == '_', r == '@', r == '-':
		default:
			return false
		}
	}
	return true
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
		info.AccountTail = tailRunes(c.Username, 3)
	} else if fileExists(statePath(txtCredFile)) {
		info.CredSaved = true
		if c, err := loadPlainCreds(); err == nil {
			info.Account = maskUser(c.Username)
			info.AccountTail = tailRunes(c.Username, 3)
		}
	}
	// 扫码登录没有凭据文件，从登录 cookie 里补账号；密码登录已有凭据时不覆盖。
	if info.AccountTail == "" {
		if account, tail := accountFromCookies(jar); tail != "" {
			info.Account = account
			info.AccountTail = tail
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

// ---- 课程与任务点 ----

// CourseChapter 章节视图，标题可能为空（上游章节接口不返回标题）。
type CourseChapter struct {
	Index        int    `json:"index"`
	ID           string `json:"id"`
	Title        string `json:"title"`
	Videos       int    `json:"videos"`
	VideosPassed int    `json:"videosPassed"`
	Works        int    `json:"works"`
}

// CourseVideo 视频任务点视图。
type CourseVideo struct {
	ChapterIndex int     `json:"chapterIndex"`
	ChapterID    string  `json:"chapterId"`
	ObjectID     string  `json:"objectId"`
	Title        string  `json:"title"`
	Duration     int     `json:"duration"`
	IsPassed     bool    `json:"isPassed"`
	SpeedLimited bool    `json:"speedLimited"`
	PlayTime     float64 `json:"playTime"`
}

// CourseDoc 文档任务点视图，来自 scan.json（上游 scan 命令的产物）。
type CourseDoc struct {
	ChapterIndex int    `json:"chapterIndex"`
	ChapterID    string `json:"chapterId"`
	Title        string `json:"title"`
	Module       string `json:"module"`
	IsPassed     bool   `json:"isPassed"`
}

// CourseWork 作业任务点视图。
type CourseWork struct {
	ChapterIndex int    `json:"chapterIndex"`
	ChapterID    string `json:"chapterId"`
	Title        string `json:"title"`
	WorkID       string `json:"workId"`
	WorkType     string `json:"workType"`
}

// CourseSummary 课程结构概览。
type CourseSummary struct {
	Chapters      int     `json:"chapters"`
	Videos        int     `json:"videos"`
	VideosPassed  int     `json:"videosPassed"`
	Works         int     `json:"works"`
	Docs          int     `json:"docs"`
	DocsPassed    int     `json:"docsPassed"`
	DurationHours float64 `json:"durationHours"`
	SpeedLimited  int     `json:"speedLimited"`
	FetchedAt     string  `json:"fetchedAt"`
}

// CourseDetail 课程页需要的全部数据。
type CourseDetail struct {
	HasCourse  bool            `json:"hasCourse"`
	Stale      bool            `json:"stale"`
	DocScanned bool            `json:"docScanned"`
	Target     Target          `json:"target"`
	Summary    CourseSummary   `json:"summary"`
	Chapters   []CourseChapter `json:"chapters"`
	Videos     []CourseVideo   `json:"videos"`
	Works      []CourseWork    `json:"works"`
	Docs       []CourseDoc     `json:"docs"`
	Message    string          `json:"message"`
}

// ListCourses 获取当前账号的课程列表；refresh 为 true 时重新拉取，不用缓存。
func ListCourses(refresh bool) ([]CourseItem, error) {
	if err := InitState(); err != nil {
		return nil, err
	}
	c := newClient(NewJar())
	if err := loadSession(c); err != nil {
		return nil, fmt.Errorf("未登录，先完成登录再获取课程列表")
	}
	list, err := loadCourseList(c, refresh)
	if err != nil {
		return nil, fmt.Errorf("获取课程列表失败: %w", err)
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("账号下没有课程，或平台返回的列表为空")
	}
	return list, nil
}

// SelectCourse 按列表序号选课：写 state/course/target.json，必要时重抓课程结构。
func SelectCourse(index int) (CourseDetail, error) {
	if err := InitState(); err != nil {
		return CourseDetail{}, err
	}
	c := newClient(NewJar())
	if err := loadSession(c); err != nil {
		return CourseDetail{}, fmt.Errorf("未登录，先完成登录再选择课程")
	}
	if _, err := switchCourseOpt(c, index, false); err != nil {
		return CourseDetail{}, fmt.Errorf("选择课程失败: %w", err)
	}
	return buildCourseDetail()
}

// RefreshCourse 对当前课程重新抓取章节与任务点，写入 state/course/course.json。
func RefreshCourse() (CourseDetail, error) {
	if err := InitState(); err != nil {
		return CourseDetail{}, err
	}
	t, err := loadTarget()
	if err != nil {
		return CourseDetail{}, fmt.Errorf("还没有选择课程，先在课程列表里选一门")
	}
	c := newClient(NewJar())
	if err := loadSession(c); err != nil {
		return CourseDetail{}, fmt.Errorf("未登录，先完成登录再抓取课程结构")
	}
	if _, err := crawlCourse(c, t); err != nil {
		return CourseDetail{}, fmt.Errorf("抓取课程结构失败: %w", err)
	}
	return buildCourseDetail()
}

// GetCourseDetail 读取本地缓存，不发起网络请求。
func GetCourseDetail() (CourseDetail, error) {
	if err := InitState(); err != nil {
		return CourseDetail{}, err
	}
	return buildCourseDetail()
}

// buildCourseDetail 把 target + course.json 转成前端使用的视图。
func buildCourseDetail() (CourseDetail, error) {
	var out CourseDetail
	t, err := loadTarget()
	if err != nil {
		out.Message = "还没有选择课程。先获取课程列表，选一门课，再抓取课程结构。"
		return out, nil
	}
	out.Target = t

	var snap CourseSnapshot
	if err := readJSON(statePath("course.json"), &snap); err != nil {
		out.Message = "已选择课程，但还没有抓取课程结构。点“抓取课程结构”从平台拉取章节和任务点。"
		return out, nil
	}
	out.HasCourse = true
	out.Stale = snap.Target.CourseID != "" && snap.Target.CourseID != t.CourseID

	chapterIndex := make(map[string]int, len(snap.Chapters))
	chapters := make([]CourseChapter, 0, len(snap.Chapters))
	for i, ch := range snap.Chapters {
		chapterIndex[ch.KnowledgeID] = i + 1
		chapters = append(chapters, CourseChapter{
			Index: i + 1,
			ID:    ch.KnowledgeID,
			Title: ch.Title,
		})
	}

	totalDuration := 0
	passed := 0
	limited := 0
	for _, v := range snap.Videos {
		ci := chapterIndex[v.ChapterID]
		isLimited := speedLimited(v.OtherInfo)
		out.Videos = append(out.Videos, CourseVideo{
			ChapterIndex: ci,
			ChapterID:    v.ChapterID,
			ObjectID:     v.ObjectID,
			Title:        v.Title,
			Duration:     v.AttDuration,
			IsPassed:     v.IsPassed,
			SpeedLimited: isLimited,
			PlayTime:     v.PlayTime,
		})
		totalDuration += v.AttDuration
		if v.IsPassed {
			passed++
		}
		if isLimited {
			limited++
		}
		if ci >= 1 && ci <= len(chapters) {
			chapters[ci-1].Videos++
			if v.IsPassed {
				chapters[ci-1].VideosPassed++
			}
		}
	}
	for _, w := range snap.Works {
		ci := chapterIndex[w.ChapterID]
		out.Works = append(out.Works, CourseWork{
			ChapterIndex: ci,
			ChapterID:    w.ChapterID,
			Title:        w.Title,
			WorkID:       w.WorkID,
			WorkType:     w.WorkType,
		})
		if ci >= 1 && ci <= len(chapters) {
			chapters[ci-1].Works++
		}
	}

	// 文档任务来自 scan.json；没有扫描过或缓存属于另一门课时，界面上提示先扫描。
	var scan ScanResult
	scanReady := readJSON(statePath("scan.json"), &scan) == nil &&
		(scan.CourseID == "" || scan.CourseID == t.CourseID)
	out.DocScanned = scanReady
	docsPassed := 0
	if scanReady {
		for _, r := range scan.Rows {
			if !isDocTaskRow(r) {
				continue
			}
			ci := chapterIndex[r.Chapter]
			out.Docs = append(out.Docs, CourseDoc{
				ChapterIndex: ci,
				ChapterID:    r.Chapter,
				Title:        r.Title,
				Module:       r.Module,
				IsPassed:     r.IsPassed,
			})
			if r.IsPassed {
				docsPassed++
			}
		}
	}

	out.Chapters = chapters
	out.Summary = CourseSummary{
		Chapters:      len(snap.Chapters),
		Videos:        len(snap.Videos),
		VideosPassed:  passed,
		Works:         len(snap.Works),
		Docs:          len(out.Docs),
		DocsPassed:    docsPassed,
		DurationHours: math.Round(float64(totalDuration)/3600*100) / 100,
		SpeedLimited:  limited,
		FetchedAt:     snap.FetchedAt,
	}

	switch {
	case len(snap.Videos) == 0 && len(snap.Works) == 0:
		out.Message = "课程结构里没有任务点，可能页面结构变了，可以重新抓取。"
	case out.Stale:
		out.Message = "课程结构缓存属于另一门课，点“抓取课程结构”重新拉取。"
	default:
		out.Message = "课程结构已加载。"
	}
	return out, nil
}

// ---- 视频播放 ----

// PlayEvent 播放进度，经 Wails Events 推给前端（play:progress）。
type PlayEvent struct {
	State        string  `json:"state"`
	Message      string  `json:"message"`
	ObjectID     string  `json:"objectId"`
	Title        string  `json:"title"`
	Index        int     `json:"index"`
	Total        int     `json:"total"`
	Percent      float64 `json:"percent"`
	Played       float64 `json:"played"`
	Duration     float64 `json:"duration"`
	SpeedLimited bool    `json:"speedLimited"`
	Ok           int     `json:"ok"`
	Fail         int     `json:"fail"`
	Skipped      int     `json:"skipped"`
}

// PlayStartResult 开始播放的即时结果。
type PlayStartResult struct {
	Running bool   `json:"running"`
	Total   int    `json:"total"`
	Message string `json:"message"`
}

var (
	playMu      sync.Mutex
	playCancel  chan struct{}
	playRunning bool
)

// errPlayCancelled 播放被用户停止时返回；只在 gui.go 里流转，不会展示成错误。
var errPlayCancelled = errors.New("已停止播放")

// StartPlay 后台播放视频任务点：objectID 为空时播放全部未完成的任务点，
// speed 为倍速，限 1 倍速的任务点会自动降回 1 倍。
// 进度通过 onEvent 推送；通过一个就把状态写回 course.json，CLI 与 GUI 都能读到。
func StartPlay(objectID string, speed float64, onEvent func(PlayEvent)) (PlayStartResult, error) {
	if err := InitState(); err != nil {
		return PlayStartResult{}, err
	}
	if speed <= 0 {
		speed = 1
	}

	playMu.Lock()
	if playRunning {
		playMu.Unlock()
		return PlayStartResult{}, fmt.Errorf("播放已在进行，先停止再开始")
	}
	playRunning = true
	cancel := make(chan struct{})
	playCancel = cancel
	playMu.Unlock()

	reset := func() {
		playMu.Lock()
		playRunning = false
		playCancel = nil
		playMu.Unlock()
	}

	t, err := loadTarget()
	if err != nil {
		reset()
		return PlayStartResult{}, fmt.Errorf("还没有选择课程，先在课程列表里选一门")
	}
	var snap CourseSnapshot
	if err := readJSON(statePath("course.json"), &snap); err != nil {
		reset()
		return PlayStartResult{}, fmt.Errorf("还没有抓取课程结构，先点“抓取课程结构”")
	}
	if snap.Target.CourseID != "" && snap.Target.CourseID != t.CourseID {
		reset()
		return PlayStartResult{}, fmt.Errorf("课程结构缓存属于另一门课，先点“抓取课程结构”重新拉取")
	}
	c := newClient(NewJar())
	if err := loadSession(c); err != nil {
		reset()
		return PlayStartResult{}, fmt.Errorf("未登录，先完成登录再播放")
	}

	var todo []VideoPoint
	for _, v := range snap.Videos {
		if objectID != "" && v.ObjectID != objectID {
			continue
		}
		if v.IsPassed {
			continue
		}
		todo = append(todo, v)
	}
	if len(todo) == 0 {
		reset()
		if objectID != "" {
			return PlayStartResult{}, fmt.Errorf("这个视频任务点已完成，不用再播放")
		}
		return PlayStartResult{}, fmt.Errorf("没有未完成的视频任务点")
	}

	emit := func(ev PlayEvent) {
		if onEvent != nil {
			onEvent(ev)
		}
	}

	go func() {
		defer reset()
		defaultReportURL = snap.ReportURL

		totalSec := 0
		limited := 0
		for _, v := range todo {
			totalSec += v.AttDuration
			if speedLimited(v.OtherInfo) {
				limited++
			}
		}
		startMsg := fmt.Sprintf("准备播放 %d 个任务点，合计 %.1f 小时", len(todo), float64(totalSec)/3600)
		if limited > 0 && speed > 1 {
			startMsg += fmt.Sprintf("；%d 个限 1 倍速，会按 1 倍播放", limited)
		}
		emit(PlayEvent{State: "starting", Message: startMsg, Total: len(todo)})

		ok, fail, skipped := 0, 0, 0
		for i, v := range todo {
			if isPlayCancelled(cancel) {
				emit(PlayEvent{State: "cancelled", Message: fmt.Sprintf("已停止播放，通过 %d，失败 %d，跳过 %d", ok, fail, skipped), Ok: ok, Fail: fail, Skipped: skipped})
				return
			}
			oriented := v
			if os.Getenv("HSC_SKIP_REFRESH") == "" {
				if fresh, err := refreshedVideo(c, t, oriented); err == nil {
					oriented = fresh
					if oriented.IsPassed {
						markSnapshotPassed(oriented.ObjectID)
						skipped++
						emit(PlayEvent{State: "skipped", Message: "服务端标记已通过，跳过：" + oriented.Title, ObjectID: oriented.ObjectID, Title: oriented.Title, Index: i + 1, Total: len(todo), Percent: 100, SpeedLimited: speedLimited(oriented.OtherInfo), Ok: ok, Fail: fail, Skipped: skipped})
						continue
					}
				}
			}
			emit(PlayEvent{State: "video", Message: "开始播放：" + oriented.Title, ObjectID: oriented.ObjectID, Title: oriented.Title, Index: i + 1, Total: len(todo), Duration: float64(oriented.AttDuration), SpeedLimited: speedLimited(oriented.OtherInfo), Ok: ok, Fail: fail, Skipped: skipped})

			index := i + 1
			passed, err := playVideoGUI(c, t, oriented, speed, 10, cancel, func(played, dur float64) {
				percent := 0.0
				if dur > 0 {
					percent = math.Round(played/dur*1000) / 10
				}
				if percent > 100 {
					percent = 100
				}
				emit(PlayEvent{State: "progress", Message: "正在播放：" + oriented.Title, ObjectID: oriented.ObjectID, Title: oriented.Title, Index: index, Total: len(todo), Percent: percent, Played: played, Duration: dur, SpeedLimited: speedLimited(oriented.OtherInfo), Ok: ok, Fail: fail, Skipped: skipped})
			})
			switch {
			case errors.Is(err, errPlayCancelled):
				emit(PlayEvent{State: "cancelled", Message: fmt.Sprintf("已停止播放，通过 %d，失败 %d，跳过 %d", ok, fail, skipped), Ok: ok, Fail: fail, Skipped: skipped})
				return
			case err != nil:
				fail++
				emit(PlayEvent{State: "failed", Message: "播放失败：" + oriented.Title + "（" + err.Error() + "）", ObjectID: oriented.ObjectID, Title: oriented.Title, Index: index, Total: len(todo), Ok: ok, Fail: fail, Skipped: skipped})
			case passed:
				ok++
				markSnapshotPassed(oriented.ObjectID)
				emit(PlayEvent{State: "passed", Message: "已通过：" + oriented.Title, ObjectID: oriented.ObjectID, Title: oriented.Title, Index: index, Total: len(todo), Percent: 100, Duration: float64(oriented.AttDuration), Ok: ok, Fail: fail, Skipped: skipped})
			}
		}
		emit(PlayEvent{State: "finished", Message: fmt.Sprintf("播放结束：通过 %d，失败 %d，跳过 %d", ok, fail, skipped), Total: len(todo), Ok: ok, Fail: fail, Skipped: skipped})
	}()

	return PlayStartResult{
		Running: true,
		Total:   len(todo),
		Message: fmt.Sprintf("开始播放 %d 个任务点", len(todo)),
	}, nil
}

// CancelPlay 停止正在进行的播放；当前这一次上报完成后生效。
func CancelPlay() {
	playMu.Lock()
	cancel := playCancel
	playMu.Unlock()
	if cancel == nil {
		return
	}
	select {
	case <-cancel:
	default:
		close(cancel)
	}
}

func isPlayCancelled(cancel <-chan struct{}) bool {
	select {
	case <-cancel:
		return true
	default:
		return false
	}
}

// sleepPlay 等待 tick 秒，等待期间可被停止；返回 true 表示已取消。
func sleepPlay(sec float64, cancel <-chan struct{}) bool {
	if sec <= 0 {
		return isPlayCancelled(cancel)
	}
	timer := time.NewTimer(time.Duration(sec * float64(time.Second)))
	defer timer.Stop()
	select {
	case <-cancel:
		return true
	case <-timer.C:
		return false
	}
}

// playVideoGUI 是上游 playOne 的界面版：循环与重试策略保持一致，
// 但每一步通过 onProgress 回报进度，并且可以中途停止。
// 签名、上报参数、限速判断仍走上游 report / videoMeta / courseRT / speedLimited。
func playVideoGUI(c *Client, t Target, v VideoPoint, speed float64, tickSec float64, cancel <-chan struct{}, onProgress func(played, dur float64)) (bool, error) {
	st, err := videoMeta(c, v)
	if err != nil {
		return false, err
	}
	rt := courseRT(v.OtherInfo)
	dur := st.Duration
	if dur <= 0 {
		return false, fmt.Errorf("时长异常: %v", dur)
	}
	target := dur - 2
	if speedLimited(v.OtherInfo) && speed > 1 {
		speed = 1
	}
	if tickSec <= 0 {
		tickSec = 10
	}
	step := speed * tickSec
	if step <= 0 {
		step = tickSec
	}
	played := playTimeSeconds(v)
	if played < 0 {
		played = 0
	}
	// 服务端记录的进度不能超过时长；超了就从头开始（与上游 playOne 一致）。
	if played >= target {
		played = math.Max(target-step, 0)
	}
	onProgress(played, dur)
	for attempt := 0; attempt < 400; attempt++ {
		if isPlayCancelled(cancel) {
			return false, errPlayCancelled
		}
		played = math.Min(played+step, target)
		passed, err := report(c, t, v, st, played, dur, rt)
		if err != nil {
			return false, err
		}
		onProgress(played, dur)
		if passed {
			return true, nil
		}
		if played >= target {
			break
		}
		if sleepPlay(tickSec, cancel) {
			return false, errPlayCancelled
		}
	}
	// 阶梯补报：95% → 98%，每次换新 dtoken（与上游一致）。
	for _, pct := range []float64{0.95, 0.96, 0.97, 0.98} {
		if isPlayCancelled(cancel) {
			return false, errPlayCancelled
		}
		if st2, err := videoMeta(c, v); err == nil {
			st = st2
		}
		onProgress(dur*pct, dur)
		if passed, err := report(c, t, v, st, dur*pct, dur, rt); err == nil && passed {
			return true, nil
		}
		if sleepPlay(math.Max(tickSec/2, 1), cancel) {
			return false, errPlayCancelled
		}
	}
	// 换 token 再报几次（与上游一致）。
	for i := 0; i < 4; i++ {
		if isPlayCancelled(cancel) {
			return false, errPlayCancelled
		}
		st2, err := videoMeta(c, v)
		if err != nil {
			if sleepPlay(2, cancel) {
				return false, errPlayCancelled
			}
			continue
		}
		if passed, err := report(c, t, v, st2, dur*0.97, dur, rt); err == nil && passed {
			return true, nil
		}
		if sleepPlay(3, cancel) {
			return false, errPlayCancelled
		}
	}
	return false, fmt.Errorf("多次上报仍未通过")
}

// markSnapshotPassed 把已通过的视频状态写回 course.json 缓存。
// 只在服务端明确返回 isPassed=true 后调用，CLI 会按这个状态跳过已完成任务点。
func markSnapshotPassed(objectID string) {
	if objectID == "" {
		return
	}
	var snap CourseSnapshot
	if err := readJSON(statePath("course.json"), &snap); err != nil {
		return
	}
	changed := false
	for i := range snap.Videos {
		if snap.Videos[i].ObjectID == objectID && !snap.Videos[i].IsPassed {
			snap.Videos[i].IsPassed = true
			changed = true
		}
	}
	if changed {
		_ = writeJSON(statePath("course.json"), snap)
	}
}

// ---- 文档任务 ----

// isDocTaskRow 与上游 cmdDoc 的过滤条件一致：文档附件才处理。
func isDocTaskRow(r ScanRow) bool {
	return strings.Contains(r.Module, "insertdoc") || r.Type == "document"
}

// ScanCourse 逐章扫描附件（上游 scan 能力），结果写入 state/course/scan.json。
func ScanCourse() (CourseDetail, error) {
	if err := InitState(); err != nil {
		return CourseDetail{}, err
	}
	t, err := loadTarget()
	if err != nil {
		return CourseDetail{}, fmt.Errorf("还没有选择课程，先在课程列表里选一门")
	}
	c := newClient(NewJar())
	if err := loadSession(c); err != nil {
		return CourseDetail{}, fmt.Errorf("未登录，先完成登录再扫描文档任务")
	}
	if _, err := scanCourse(c, t); err != nil {
		return CourseDetail{}, fmt.Errorf("扫描文档任务失败: %w", err)
	}
	return buildCourseDetail()
}

// CompleteDocs 完成未完成的文档任务（上游 cmdDoc），完成后重新扫描刷新状态。
func CompleteDocs() (CourseDetail, error) {
	if err := InitState(); err != nil {
		return CourseDetail{}, err
	}
	t, err := loadTarget()
	if err != nil {
		return CourseDetail{}, fmt.Errorf("还没有选择课程，先在课程列表里选一门")
	}
	scan, err := loadScanFor(t)
	if err != nil {
		return CourseDetail{}, fmt.Errorf("还没有扫描文档任务，先点“扫描文档任务”")
	}
	pending := 0
	for _, r := range scan.Rows {
		if isDocTaskRow(r) && !r.IsPassed {
			pending++
		}
	}
	if pending == 0 {
		return buildCourseDetail()
	}
	if err := cmdDoc(nil); err != nil {
		return CourseDetail{}, fmt.Errorf("完成文档任务失败: %w", err)
	}
	c := newClient(NewJar())
	if err := loadSession(c); err != nil {
		return CourseDetail{}, fmt.Errorf("文档任务已处理，但读取登录态失败，请重新登录后确认状态")
	}
	if _, err := scanCourse(c, t); err != nil {
		return CourseDetail{}, fmt.Errorf("文档任务已处理，但重新扫描失败: %w；可以稍后点“重新扫描”确认状态", err)
	}
	return buildCourseDetail()
}
