//go:build debug

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hsc-gui/internal/hsc"
)

// 每条调试记录都要落到当天的 state/logs/gui-debug-YYYYMMDD.log。
func TestDebugLogFileWrites(t *testing.T) {
	if err := hsc.InitState(); err != nil {
		t.Fatalf("初始化 state 失败: %v", err)
	}
	debugSink = debugFileSink{}
	app := &App{}
	app.writeDebugFile(time.Now(), "调用", "TestDebugLogFileWrites 测试记录")

	path := debugLogPath()
	if path == "" {
		t.Fatal("debugLogPath 为空")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取日志失败: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "TestDebugLogFileWrites 测试记录") {
		t.Fatalf("日志里没有刚写的记录:\n%s", text)
	}
	if !strings.Contains(text, "[调用]") {
		t.Fatalf("日志缺少类型标记:\n%s", text)
	}
}

// 超过上限的历史文件要轮转成 .1.log。
func TestRotateExistingDebugLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gui-debug-test.log")
	if err := os.WriteFile(path, make([]byte, debugLogMaxBytes+1), 0o644); err != nil {
		t.Fatalf("准备大文件失败: %v", err)
	}
	rotateExisting(path)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("原文件应被移走，err=%v", err)
	}
	if info, err := os.Stat(path + ".1"); err != nil || info.Size() != debugLogMaxBytes+1 {
		t.Fatalf("备份文件不对: err=%v", err)
	}
}
