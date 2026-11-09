//go:build !debug

package main

// debugBuild 由构建标签控制：默认 release。
// debug 构建命令见 AGENTS.md：wails build -tags debug
const debugBuild = false
