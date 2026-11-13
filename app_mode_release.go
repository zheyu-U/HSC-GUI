//go:build !debug

package main

import "time"

// debugBuild 由构建标签控制：默认 release。
// debug 构建命令见 AGENTS.md：wails build -tags debug
const debugBuild = false

// debugLogPath release 构建不落盘调试日志。
func debugLogPath() string {
	return ""
}

// writeDebugFile release 构建不写日志文件，保持安静也不把内部信息写到磁盘。
func (a *App) writeDebugFile(time.Time, string, string) {}
