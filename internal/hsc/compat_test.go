package hsc

// 状态兼容测试：验证 GUI 侧凭据与上游 CLI 的凭据文件互相可解。
// 需要先编译上游 CLI 并设置 HSC_CLI_BIN，例如：
//   go build -o .cache\bin\hsc.exe .   （在上游 go/ 目录）
//   $env:HSC_CLI_BIN = "<仓库>\.cache\bin\hsc.exe"
//   go test ./internal/hsc -run TestCredentialInteropWithCLI -v
// 没设置 HSC_CLI_BIN 时跳过，保证新环境 go test ./... 不受影响。

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func runCLI(t *testing.T, bin, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestCredentialInteropWithCLI(t *testing.T) {
	cli := os.Getenv("HSC_CLI_BIN")
	if cli == "" {
		t.Skip("未设置 HSC_CLI_BIN，跳过与 CLI 的凭据互通测试")
	}
	if _, err := os.Stat(cli); err != nil {
		t.Skipf("HSC_CLI_BIN 不可用: %v", err)
	}

	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	// 用独立的 state 目录，避免碰到仓库里真实的登录态
	rootDir, stateDir = "", ""
	if err := initPaths(); err != nil {
		t.Fatalf("initPaths: %v", err)
	}

	// 方向一：GUI 写，CLI 读
	if err := saveCreds("202399999", "test-pass-123"); err != nil {
		t.Fatalf("GUI 保存凭据失败: %v", err)
	}
	out, err := runCLI(t, cli, dir, "cred", "status")
	if err != nil {
		t.Fatalf("CLI cred status 失败: %v\n%s", err, out)
	}
	if !strings.Contains(out, "可解密 : 是") {
		t.Fatalf("CLI 解不开 GUI 写的凭据:\n%s", out)
	}

	// 方向二：CLI 写，GUI 读
	if err := os.RemoveAll(stateDir); err != nil {
		t.Fatal(err)
	}
	rootDir, stateDir = "", ""
	if err := initPaths(); err != nil {
		t.Fatalf("initPaths（第二轮）: %v", err)
	}
	out, err = runCLI(t, cli, dir, "cred", "save", "202388888", "cli-pass-456")
	if err != nil {
		t.Fatalf("CLI cred save 失败: %v\n%s", err, out)
	}
	c, err := loadEncryptedCreds()
	if err != nil {
		t.Fatalf("GUI 解不开 CLI 写的凭据: %v", err)
	}
	if c.Username != "202388888" || c.Password != "cli-pass-456" {
		t.Fatalf("凭据内容不一致: 账号=%q 密码=%q", c.Username, c.Password)
	}
}
