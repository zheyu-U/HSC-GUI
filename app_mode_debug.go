//go:build debug

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"hsc-gui/internal/hsc"
)

// debugBuild 由构建标签控制：-tags debug 时开启调试面板、调试日志与落盘日志。
const debugBuild = true

// 调试日志落盘：一天一个文件，单文件超过 2MB 轮转成 .1.log，避免无限增长。
const debugLogMaxBytes int64 = 2 << 20

type debugFileSink struct {
	mu   sync.Mutex
	day  string
	path string
	file *os.File
	size int64
}

var debugSink debugFileSink

// debugLogPath 返回今天调试日志的路径，state 目录未就绪时返回空串。
func debugLogPath() string {
	dir := hsc.StateDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "logs", "gui-debug-"+time.Now().Format("20060102")+".log")
}

// writeDebugFile 把一条调试记录追加到当天日志文件；写失败不影响界面。
func (a *App) writeDebugFile(ts time.Time, kind, message string) {
	if hsc.StateDir() == "" {
		return
	}
	debugSink.write(ts, kind, message)
}

func (s *debugFileSink) write(ts time.Time, kind, message string) {
	dir := filepath.Join(hsc.StateDir(), "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	day := ts.Format("20060102")
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.file != nil && s.day != day {
		s.closeLocked()
	}
	if s.file == nil {
		path := filepath.Join(dir, "gui-debug-"+day+".log")
		rotateExisting(path)
		file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return
		}
		s.file = file
		s.day = day
		s.path = path
		s.size = 0
		if info, err := file.Stat(); err == nil {
			s.size = info.Size()
		}
		if s.size == 0 {
			header := fmt.Sprintf("# hsc-gui %s debug log · %s · %s\n",
				appVersion, ts.Format("2006-01-02 15:04:05"), hsc.StateDir())
			if n, err := file.WriteString(header); err == nil {
				s.size += int64(n)
			}
		}
	}
	if s.size >= debugLogMaxBytes {
		s.rotateLocked()
	}
	line := fmt.Sprintf("%s [%s] %s\n", ts.Format("2006-01-02 15:04:05.000"), kind, message)
	if n, err := s.file.WriteString(line); err == nil {
		s.size += int64(n)
	}
}

func (s *debugFileSink) closeLocked() {
	if s.file != nil {
		_ = s.file.Close()
		s.file = nil
	}
}

// rotateExisting 把已经超限的历史文件改成 .1.log（覆盖旧的备份）。
func rotateExisting(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() < debugLogMaxBytes {
		return
	}
	backup := path + ".1"
	_ = os.Remove(backup)
	_ = os.Rename(path, backup)
}

func (s *debugFileSink) rotateLocked() {
	path := s.path
	s.closeLocked()
	backup := path + ".1"
	_ = os.Remove(backup)
	_ = os.Rename(path, backup)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	s.file = file
	s.size = 0
}
