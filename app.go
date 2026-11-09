package main

import (
	"context"
	"fmt"

	"hsc-gui/internal/hsc"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App 是暴露给前端的绑定对象，方法清单同步维护在 docs/api.md。
type App struct {
	ctx context.Context
}

// NewApp 创建 App。
func NewApp() *App {
	return &App{}
}

// startup 在窗口创建前调用，负责确定 state 目录（与 CLI 同一份）。
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if err := hsc.InitState(); err != nil {
		fmt.Println("初始化 state 目录失败:", err)
	}
}

// StartQRLogin 生成二维码并开始后台轮询，进度经 login:qr 事件推送。
func (a *App) StartQRLogin() (hsc.QRLoginResult, error) {
	return hsc.StartQRLogin(func(ev hsc.LoginEvent) {
		runtime.EventsEmit(a.ctx, "login:qr", ev)
	})
}

// CancelQRLogin 取消扫码轮询。
func (a *App) CancelQRLogin() {
	hsc.CancelQRLogin()
}

// StartCaptcha 取一张验证码并返回自动识别建议值。
func (a *App) StartCaptcha() (hsc.CaptchaView, error) {
	return hsc.StartCaptcha()
}

// LoginPassword 提交账号密码与验证码。
func (a *App) LoginPassword(username, password, code string) (hsc.LoginResult, error) {
	return hsc.LoginPassword(username, password, code)
}

// SessionStatus 返回本地会话与凭据状态。
func (a *App) SessionStatus() (hsc.SessionInfo, error) {
	return hsc.SessionStatus()
}

// CheckSession 对站点做一次登录态校验（需要已配置课程）。
func (a *App) CheckSession() (hsc.CheckResult, error) {
	return hsc.CheckSession()
}

// Logout 清除登录态；all 为 true 时连凭据一起清。
func (a *App) Logout(all bool) (hsc.LogoutResult, error) {
	return hsc.Logout(all)
}
