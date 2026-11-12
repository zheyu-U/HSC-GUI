package main

import (
	"context"
	"fmt"
	"os"
	goruntime "runtime"
	"sync"
	"time"

	"hsc-gui/internal/hsc"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const appVersion = "0.8.1"

// App 是暴露给前端的绑定对象，方法清单同步维护在 docs/api.md。
type App struct {
	ctx context.Context

	debugMu  sync.Mutex
	debugLog []DebugEntry
}

// AppModeInfo 构建模式与版本信息。
type AppModeInfo struct {
	Mode    string `json:"mode"`
	Debug   bool   `json:"debug"`
	Version string `json:"version"`
}

// DebugInfo 调试面板用的运行环境信息。
type DebugInfo struct {
	Mode        string `json:"mode"`
	StateDir    string `json:"stateDir"`
	Cwd         string `json:"cwd"`
	ExePath     string `json:"exePath"`
	GoVersion   string `json:"goVersion"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	CookieCount int    `json:"cookieCount"`
	LoggedIn    bool   `json:"loggedIn"`
}

// DebugEntry 一条调试日志。
type DebugEntry struct {
	Time    string `json:"time"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

// NewApp 创建 App。
func NewApp() *App {
	return &App{}
}

// startup 在窗口创建前调用，负责确定 state 目录（与 CLI 同一份）。
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if err := hsc.InitState(); err != nil {
		a.record("错误", "初始化 state 目录失败: "+err.Error())
		fmt.Println("初始化 state 目录失败:", err)
		return
	}
	a.record("启动", "state 目录: "+hsc.StateDir())
}

// record 只在 debug 构建里记录调试日志，release 不占内存也不暴露内部信息。
func (a *App) record(kind, message string) {
	if !debugBuild {
		return
	}
	a.debugMu.Lock()
	defer a.debugMu.Unlock()
	a.debugLog = append(a.debugLog, DebugEntry{
		Time:    time.Now().Format("15:04:05"),
		Kind:    kind,
		Message: message,
	})
	if len(a.debugLog) > 300 {
		a.debugLog = a.debugLog[len(a.debugLog)-300:]
	}
}

// AppMode 返回当前构建模式与版本，前端据此决定显示 release 还是 debug 界面。
func (a *App) AppMode() AppModeInfo {
	mode := "release"
	if debugBuild {
		mode = "debug"
	}
	return AppModeInfo{Mode: mode, Debug: debugBuild, Version: appVersion}
}

// DebugInfo 返回运行环境信息，仅 debug 构建使用。
func (a *App) DebugInfo() (DebugInfo, error) {
	if !debugBuild {
		return DebugInfo{}, fmt.Errorf("release 构建不提供调试信息")
	}
	mode := "debug"
	info := DebugInfo{
		Mode:      mode,
		StateDir:  hsc.StateDir(),
		GoVersion: goruntime.Version(),
		OS:        goruntime.GOOS,
		Arch:      goruntime.GOARCH,
	}
	if cwd, err := os.Getwd(); err == nil {
		info.Cwd = cwd
	}
	if exe, err := os.Executable(); err == nil {
		info.ExePath = exe
	}
	if s, err := hsc.SessionStatus(); err == nil {
		info.CookieCount = s.CookieCount
		info.LoggedIn = s.LoggedIn
	}
	return info, nil
}

// DebugLog 返回最近的绑定调用与事件记录，仅 debug 构建使用。
func (a *App) DebugLog() []DebugEntry {
	if !debugBuild {
		return nil
	}
	a.debugMu.Lock()
	defer a.debugMu.Unlock()
	out := make([]DebugEntry, len(a.debugLog))
	copy(out, a.debugLog)
	return out
}

// ClearDebugLog 清空调试日志。
func (a *App) ClearDebugLog() {
	if !debugBuild {
		return
	}
	a.debugMu.Lock()
	a.debugLog = nil
	a.debugMu.Unlock()
}

// StartQRLogin 生成二维码并开始后台轮询，进度经 login:qr 事件推送。
func (a *App) StartQRLogin() (hsc.QRLoginResult, error) {
	a.record("调用", "StartQRLogin")
	res, err := hsc.StartQRLogin(func(ev hsc.LoginEvent) {
		a.record("事件", ev.State+" "+ev.Message)
		runtime.EventsEmit(a.ctx, "login:qr", ev)
	})
	if err != nil {
		a.record("错误", "StartQRLogin: "+err.Error())
		return res, err
	}
	a.record("结果", "二维码已生成")
	return res, nil
}

// CancelQRLogin 取消扫码轮询。
func (a *App) CancelQRLogin() {
	a.record("调用", "CancelQRLogin")
	hsc.CancelQRLogin()
}

// StartCaptcha 取一张验证码并返回自动识别建议值。
func (a *App) StartCaptcha() (hsc.CaptchaView, error) {
	a.record("调用", "StartCaptcha")
	res, err := hsc.StartCaptcha()
	if err != nil {
		a.record("错误", "StartCaptcha: "+err.Error())
		return res, err
	}
	a.record("结果", fmt.Sprintf("验证码已识别: 建议 %s，置信度 %.2f，字符数 %d",
		res.SuggestedCode, res.Confidence, res.Count))
	return res, nil
}

// LoginPassword 提交账号密码与验证码。
func (a *App) LoginPassword(username, password, code string) (hsc.LoginResult, error) {
	a.record("调用", "LoginPassword 账号 "+username)
	res, err := hsc.LoginPassword(username, password, code)
	if err != nil {
		a.record("错误", "LoginPassword: "+err.Error())
		return res, err
	}
	a.record("结果", "LoginPassword: "+res.Message)
	return res, nil
}

// SessionStatus 返回本地会话与凭据状态。
func (a *App) SessionStatus() (hsc.SessionInfo, error) {
	res, err := hsc.SessionStatus()
	if err != nil {
		a.record("错误", "SessionStatus: "+err.Error())
	}
	return res, err
}

// CheckSession 对站点做一次登录态校验（需要已配置课程）。
func (a *App) CheckSession() (hsc.CheckResult, error) {
	a.record("调用", "CheckSession")
	res, err := hsc.CheckSession()
	if err != nil {
		a.record("错误", "CheckSession: "+err.Error())
		return res, err
	}
	a.record("结果", "CheckSession: "+res.Message)
	return res, nil
}

// Logout 清除登录态；all 为 true 时连凭据一起清。
func (a *App) Logout(all bool) (hsc.LogoutResult, error) {
	a.record("调用", fmt.Sprintf("Logout all=%v", all))
	res, err := hsc.Logout(all)
	if err != nil {
		a.record("错误", "Logout: "+err.Error())
		return res, err
	}
	a.record("结果", "Logout: "+res.Message)
	return res, nil
}

// ListCourses 获取当前账号的课程列表；refresh 为 true 时重新拉取。
func (a *App) ListCourses(refresh bool) ([]hsc.CourseItem, error) {
	a.record("调用", fmt.Sprintf("ListCourses refresh=%v", refresh))
	list, err := hsc.ListCourses(refresh)
	if err != nil {
		a.record("错误", "ListCourses: "+err.Error())
		return nil, err
	}
	a.record("结果", fmt.Sprintf("课程列表 %d 门", len(list)))
	return list, nil
}

// SelectCourse 按序号选择课程，必要时抓取课程结构。
func (a *App) SelectCourse(index int) (hsc.CourseDetail, error) {
	a.record("调用", fmt.Sprintf("SelectCourse index=%d", index))
	detail, err := hsc.SelectCourse(index)
	if err != nil {
		a.record("错误", "SelectCourse: "+err.Error())
		return detail, err
	}
	a.record("结果", "已选择课程: "+detail.Target.Name)
	return detail, nil
}

// RefreshCourse 重新抓取当前课程的章节与任务点。
func (a *App) RefreshCourse() (hsc.CourseDetail, error) {
	a.record("调用", "RefreshCourse")
	detail, err := hsc.RefreshCourse()
	if err != nil {
		a.record("错误", "RefreshCourse: "+err.Error())
		return detail, err
	}
	a.record("结果", fmt.Sprintf("课程结构: %d 章节 / %d 视频 / %d 作业",
		detail.Summary.Chapters, detail.Summary.Videos, detail.Summary.Works))
	return detail, nil
}

// GetCourseDetail 读取本地课程结构缓存，不发起网络请求。
func (a *App) GetCourseDetail() (hsc.CourseDetail, error) {
	detail, err := hsc.GetCourseDetail()
	if err != nil {
		a.record("错误", "GetCourseDetail: "+err.Error())
	}
	return detail, err
}

// ScanCourse 扫描当前课程的文档任务点，结果写入 state/course/scan.json。
func (a *App) ScanCourse() (hsc.CourseDetail, error) {
	a.record("调用", "ScanCourse")
	detail, err := hsc.ScanCourse()
	if err != nil {
		a.record("错误", "ScanCourse: "+err.Error())
		return detail, err
	}
	a.record("结果", fmt.Sprintf("文档任务 %d 个，已完成 %d 个", detail.Summary.Docs, detail.Summary.DocsPassed))
	return detail, nil
}

// CompleteDocs 完成未完成的文档任务，完成后重新扫描刷新状态。
func (a *App) CompleteDocs() (hsc.CourseDetail, error) {
	a.record("调用", "CompleteDocs")
	detail, err := hsc.CompleteDocs()
	if err != nil {
		a.record("错误", "CompleteDocs: "+err.Error())
		return detail, err
	}
	a.record("结果", fmt.Sprintf("文档任务 %d 个，已完成 %d 个", detail.Summary.Docs, detail.Summary.DocsPassed))
	return detail, nil
}

// StartPlay 后台播放视频任务点，进度经 play:progress 事件推送。
func (a *App) StartPlay(objectID string, speed float64) (hsc.PlayStartResult, error) {
	a.record("调用", fmt.Sprintf("StartPlay objectId=%q speed=%g", objectID, speed))
	res, err := hsc.StartPlay(objectID, speed, func(ev hsc.PlayEvent) {
		if ev.State != "progress" {
			a.record("事件", "play "+ev.State+" "+ev.Message)
		}
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "play:progress", ev)
		}
	})
	if err != nil {
		a.record("错误", "StartPlay: "+err.Error())
		return res, err
	}
	a.record("结果", res.Message)
	return res, nil
}

// CancelPlay 停止正在进行的播放。
func (a *App) CancelPlay() {
	a.record("调用", "CancelPlay")
	hsc.CancelPlay()
}

// GetQuizOverview 读取作业与测验列表和抓题情况，不发起网络请求。
func (a *App) GetQuizOverview() (hsc.QuizOverview, error) {
	a.record("调用", "GetQuizOverview")
	out, err := hsc.GetQuizOverview()
	if err != nil {
		a.record("错误", "GetQuizOverview: "+err.Error())
		return out, err
	}
	a.record("结果", fmt.Sprintf("作业与测验 %d 份，已抓题 %d 份", len(out.Items), out.FetchedCount))
	return out, nil
}

// DumpQuizzes 抓取未提交的作业/测验题目（上游 quiz dump）。
func (a *App) DumpQuizzes() (hsc.QuizOverview, error) {
	a.record("调用", "DumpQuizzes")
	out, err := hsc.DumpQuizzes()
	if err != nil {
		a.record("错误", "DumpQuizzes: "+err.Error())
		return out, err
	}
	a.record("结果", fmt.Sprintf("抓题完成：%d 份，共 %d 题", out.FetchedCount, out.QuestionCount))
	return out, nil
}

// GetQuizQuestions 读取某份作业/测验的题目（只读，不含答案）。
func (a *App) GetQuizQuestions(workID string) (hsc.QuizDetailView, error) {
	a.record("调用", "GetQuizQuestions "+workID)
	out, err := hsc.GetQuizQuestions(workID)
	if err != nil {
		a.record("错误", "GetQuizQuestions: "+err.Error())
		return out, err
	}
	a.record("结果", fmt.Sprintf("%s：%d 题", out.Title, len(out.Questions)))
	return out, nil
}
