package main

// 作业与测验自动答题的 Wails 绑定。方法清单与事件字段同步维护在 docs/api.md。
// 进度走 quiz:progress 事件；协议逻辑都在 internal/hsc 里，本文件只做转发与调试记录。

import (
	"fmt"

	"hsc-gui/internal/hsc"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// GetQuizSettings 读取 state/config.toml 里的自动答题设置，API Key 只返回脱敏值。
func (a *App) GetQuizSettings() (hsc.QuizSettings, error) {
	a.record("调用", "GetQuizSettings")
	out, err := hsc.GetQuizSettings()
	if err != nil {
		a.record("错误", "GetQuizSettings: "+err.Error())
		return out, err
	}
	a.record("结果", "QuizSettings: "+out.Message)
	return out, nil
}

// SaveQuizSettings 保存自动答题设置；apiKey 留空表示保留原来的 Key。
func (a *App) SaveQuizSettings(in hsc.QuizSettings) (hsc.QuizSettings, error) {
	a.record("调用", fmt.Sprintf("SaveQuizSettings enabled=%v model=%s", in.Enabled, in.Model))
	out, err := hsc.SaveQuizSettings(in)
	if err != nil {
		a.record("错误", "SaveQuizSettings: "+err.Error())
		return out, err
	}
	a.record("结果", "QuizSettings 已保存")
	return out, nil
}

// SolveQuizzes 自动作答：only 为 -1 时做全部，>= 0 时只做该序号的一份。
// 进度经 quiz:progress 事件推送，统计结果在返回值里。
func (a *App) SolveQuizzes(only int) (hsc.QuizSolveResult, error) {
	a.record("调用", fmt.Sprintf("SolveQuizzes only=%d", only))
	res, err := hsc.SolveQuizzes(only, func(ev hsc.QuizEvent) {
		// 逐题进度太密，只把开始、作答结果和结束记进调试日志。
		if ev.State != "solving" {
			a.record("事件", "quiz "+ev.State+" "+ev.Message)
		}
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "quiz:progress", ev)
		}
	})
	if err != nil {
		a.record("错误", "SolveQuizzes: "+err.Error())
		return res, err
	}
	a.record("结果", res.Message)
	return res, nil
}

// SubmitQuizzes 交卷：only 为 -1 时交全部，>= 0 时只交该序号的一份。
func (a *App) SubmitQuizzes(only int) (hsc.QuizSubmitResult, error) {
	a.record("调用", fmt.Sprintf("SubmitQuizzes only=%d", only))
	res, err := hsc.SubmitQuizzes(only)
	if err != nil {
		a.record("错误", "SubmitQuizzes: "+err.Error())
		return res, err
	}
	a.record("结果", res.Message)
	return res, nil
}

// GetQuizBank 读取题库题量。
func (a *App) GetQuizBank() (hsc.QuizBankInfo, error) {
	a.record("调用", "GetQuizBank")
	res, err := hsc.GetQuizBank()
	if err != nil {
		a.record("错误", "GetQuizBank: "+err.Error())
		return res, err
	}
	a.record("结果", res.Message)
	return res, nil
}

// ClearQuizBank 清空题库缓存。
func (a *App) ClearQuizBank() (hsc.QuizBankInfo, error) {
	a.record("调用", "ClearQuizBank")
	res, err := hsc.ClearQuizBank()
	if err != nil {
		a.record("错误", "ClearQuizBank: "+err.Error())
		return res, err
	}
	a.record("结果", res.Message)
	return res, nil
}

// PeekQuizzes 读取每份测验的批阅页标题，判断已批阅/已提交/可作答；只读。
func (a *App) PeekQuizzes(only int) ([]hsc.QuizPeekItem, error) {
	a.record("调用", fmt.Sprintf("PeekQuizzes only=%d", only))
	items, err := hsc.PeekQuizzes(only)
	if err != nil {
		a.record("错误", "PeekQuizzes: "+err.Error())
		return items, err
	}
	a.record("结果", fmt.Sprintf("批阅状态 %d 份", len(items)))
	return items, nil
}
