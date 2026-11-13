package hsc

// GUI 课程页视图的纯本地测试：不发起网络请求，只验证 course.json / scan.json
// 到前端结构的映射，以及播放通过后写回缓存的行为。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// useTempState 把 stateDir 指到临时目录，测试结束后还原全局状态。
func useTempState(t *testing.T) string {
	t.Helper()
	oldRoot, oldState := rootDir, stateDir
	dir := t.TempDir()
	rootDir = dir
	stateDir = filepath.Join(dir, "state")
	t.Cleanup(func() {
		rootDir, stateDir = oldRoot, oldState
	})
	return stateDir
}

func writeTestJSON(t *testing.T, name string, v any) {
	t.Helper()
	if err := writeJSON(statePath(name), v); err != nil {
		t.Fatalf("写入 %s 失败: %v", name, err)
	}
}

// 服务器确认全部完成后，本地把文档行标记成已完成（只动文档行，不动视频）。
func TestMarkDocRowsPassed(t *testing.T) {
	scan := &ScanResult{Rows: []ScanRow{
		{Module: "insertdoc", Type: "document", IsPassed: false},
		{Module: "insertdoc", Type: "document", IsPassed: true},
		{Module: "insertvideo", Type: "video", IsPassed: false},
	}}
	if marked := markDocRowsPassed(scan); marked != 1 {
		t.Fatalf("应只标记 1 个未完成文档，got %d", marked)
	}
	if !scan.Rows[0].IsPassed || !scan.Rows[1].IsPassed {
		t.Fatal("文档行应全部标记为已完成")
	}
	if scan.Rows[2].IsPassed {
		t.Fatal("视频行不应该被标记")
	}
}

// 上游 cmdDoc 只把“完成 X，跳过 Y，失败 Z”写进 gcli.log，界面按这行给结果。
func TestParseDocLog(t *testing.T) {
	text := "ok 123.pdf\n完成 3，跳过 1，失败 0\n章节 53 个\n"
	done, skipped, failed, ok := parseDocLog(text)
	if !ok || done != 3 || skipped != 1 || failed != 0 {
		t.Fatalf("解析结果不对: ok=%v done=%d skipped=%d failed=%d", ok, done, skipped, failed)
	}

	if _, _, _, ok := parseDocLog("没有统计行"); ok {
		t.Fatal("没有统计行时应返回 ok=false")
	}

	// 多次运行取最后一次
	if done, _, _, _ := parseDocLog("完成 1，跳过 0，失败 0\n完成 9，跳过 0，失败 2\n"); done != 9 {
		t.Fatalf("应取最后一次统计，got done=%d", done)
	}
}

// 播放上报的调试回调：设了就转发，没设（release）静默不 panic。
func TestPlayTraceHook(t *testing.T) {
	var got []string
	SetPlayTrace(func(msg string) { got = append(got, msg) })
	defer SetPlayTrace(nil)

	tracePlay("上报 %s played=%d", "v1", 12)
	if len(got) != 1 || got[0] != "上报 v1 played=12" {
		t.Fatalf("回调内容不对: %#v", got)
	}

	SetPlayTrace(nil)
	tracePlay("没设回调时不能 panic")
}

func TestBuildCourseDetailWithDocs(t *testing.T) {
	useTempState(t)

	target := Target{Name: "数据结构", CourseID: "c1", ClazzID: "k1"}
	writeTestJSON(t, "target.json", target)
	writeTestJSON(t, "course.json", CourseSnapshot{
		Target:   target,
		Chapters: []Chapter{{KnowledgeID: "ch1", Title: "第一章"}},
		Videos: []VideoPoint{{
			ChapterID:   "ch1",
			ObjectID:    "v1",
			Title:       "1.1 绪论",
			OtherInfo:   "rt_d",
			AttDuration: 600,
		}},
		Works: []WorkPoint{{ChapterID: "ch1", Title: "作业一"}},
	})
	writeTestJSON(t, "scan.json", ScanResult{
		CourseID: "c1",
		Rows: []ScanRow{
			{Chapter: "ch1", Module: "insertdoc", Type: "document", Title: "文档一", IsPassed: true},
			{Chapter: "ch1", Module: "insertvideo", Type: "video", Title: "视频附件"},
			{Chapter: "ch1", Module: "insertdoc", Type: "document", Title: "文档二"},
		},
	})

	detail, err := buildCourseDetail()
	if err != nil {
		t.Fatalf("buildCourseDetail: %v", err)
	}
	if !detail.DocScanned {
		t.Fatal("scan.json 属于当前课程时应标记为已扫描")
	}
	if len(detail.Docs) != 2 {
		t.Fatalf("文档任务应过滤出 2 个，实际 %d 个", len(detail.Docs))
	}
	if detail.Docs[0].Title != "文档一" || !detail.Docs[0].IsPassed {
		t.Fatalf("第一个文档状态不对: %+v", detail.Docs[0])
	}
	if detail.Summary.Docs != 2 || detail.Summary.DocsPassed != 1 {
		t.Fatalf("文档统计不对: %d/%d", detail.Summary.DocsPassed, detail.Summary.Docs)
	}
	if len(detail.Videos) != 1 || detail.Videos[0].ObjectID != "v1" {
		t.Fatalf("视频视图缺少 objectId: %+v", detail.Videos)
	}

	// scan.json 属于另一门课时不采用，界面应提示重新扫描。
	writeTestJSON(t, "scan.json", ScanResult{CourseID: "c2"})
	detail, err = buildCourseDetail()
	if err != nil {
		t.Fatalf("buildCourseDetail（另一门课）: %v", err)
	}
	if detail.DocScanned || len(detail.Docs) != 0 {
		t.Fatalf("另一门课的 scan.json 不应被采用: scanned=%v docs=%d", detail.DocScanned, len(detail.Docs))
	}
}

func TestCourseDetailSlicesNotNull(t *testing.T) {
	useTempState(t)

	target := Target{Name: "数据结构", CourseID: "c1", ClazzID: "k1"}
	writeTestJSON(t, "target.json", target)
	writeTestJSON(t, "course.json", CourseSnapshot{
		Target:   target,
		Chapters: []Chapter{{KnowledgeID: "ch1"}},
		Videos:   []VideoPoint{{ChapterID: "ch1", ObjectID: "v1", AttDuration: 600}},
	})

	detail, err := buildCourseDetail()
	if err != nil {
		t.Fatalf("buildCourseDetail: %v", err)
	}
	if detail.Docs == nil {
		t.Fatal("没有 scan.json 时 Docs 应该是空切片，不能是 null")
	}
	if detail.Videos == nil || detail.Works == nil || detail.Chapters == nil {
		t.Fatal("列表字段都应该是空切片，不能是 null")
	}

	b, err := json.Marshal(detail)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"docs":[]`, `"works":[]`} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("JSON 应包含 %s，实际: %s", want, string(b))
		}
	}
}

func TestMarkSnapshotPassed(t *testing.T) {
	useTempState(t)
	writeTestJSON(t, "course.json", CourseSnapshot{
		Videos: []VideoPoint{{ObjectID: "v1"}, {ObjectID: "v2"}},
	})

	markSnapshotPassed("v1")

	var snap CourseSnapshot
	if err := readJSON(statePath("course.json"), &snap); err != nil {
		t.Fatalf("读取 course.json: %v", err)
	}
	if !snap.Videos[0].IsPassed {
		t.Fatal("v1 应该被标记为已完成")
	}
	if snap.Videos[1].IsPassed {
		t.Fatal("v2 不应被改动")
	}
}

func TestIsDocTaskRow(t *testing.T) {
	cases := []struct {
		name string
		row  ScanRow
		want bool
	}{
		{"模块带 insertdoc", ScanRow{Module: "insertdoc"}, true},
		{"类型是 document", ScanRow{Module: "other", Type: "document"}, true},
		{"普通视频附件", ScanRow{Module: "insertvideo", Type: "video"}, false},
		{"其他附件", ScanRow{Module: "insertimage", Type: "image"}, false},
	}
	for _, c := range cases {
		if got := isDocTaskRow(c.row); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestResolveStateRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "proj")
	bin := filepath.Join(root, "build", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "wails.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(bin, "hsc-gui.exe")

	if got := resolveStateRoot(false, root, exe); got != bin {
		t.Fatalf("打包模式应使用 exe 目录，got %q", got)
	}
	if got := resolveStateRoot(true, root, exe); got != root {
		t.Fatalf("开发模式应使用项目根目录，got %q", got)
	}
	if got := resolveStateRoot(true, bin, exe); got != root {
		t.Fatalf("开发模式应从子目录向上找到项目根，got %q", got)
	}
	empty := t.TempDir()
	if got := resolveStateRoot(true, empty, exe); got != empty {
		t.Fatalf("开发模式找不到 wails.json 时应留在当前目录，got %q", got)
	}
}

func TestAccountFromCookies(t *testing.T) {
	jar := NewJar()
	jar.Restore(map[string]map[string]string{
		"hust.edu.cn": {"uname": "U202315009"},
	})
	account, tail := accountFromCookies(jar)
	if account != "U202***" {
		t.Fatalf("账号应按 maskUser 打码，got %q", account)
	}
	if tail != "009" {
		t.Fatalf("账号尾号应为 009，got %q", tail)
	}

	if account, tail := accountFromCookies(NewJar()); account != "" || tail != "" {
		t.Fatalf("没有 uname 时应返回空，got %q %q", account, tail)
	}

	chinese := NewJar()
	chinese.Restore(map[string]map[string]string{
		"hust.edu.cn": {"uname": "张三"},
	})
	if account, tail := accountFromCookies(chinese); account != "" || tail != "" {
		t.Fatalf("中文姓名不应当账号，got %q %q", account, tail)
	}
}

// TestStateDirFollowsExe 模拟从项目根目录启动打包后的 exe：
// state 应该落到 exe 目录，登录 cookie 能正常读出来。
func TestStateDirFollowsExe(t *testing.T) {
	proj := t.TempDir()
	bin := filepath.Join(proj, "build", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "wails.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	exe := filepath.Join(bin, "hsc-gui.exe")
	root := resolveStateRoot(false, proj, exe)
	if root != bin {
		t.Fatalf("打包模式应从项目根目录落到 exe 目录，got %q", root)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	oldRoot, oldState := rootDir, stateDir
	t.Cleanup(func() {
		_ = os.Chdir(oldWd)
		rootDir, stateDir = oldRoot, oldState
	})
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	rootDir, stateDir = "", ""
	if err := initPaths(); err != nil {
		t.Fatal(err)
	}

	jar := NewJar()
	jar.Restore(map[string]map[string]string{
		"hust.edu.cn": {"uname": "U202315139"},
	})
	if err := jar.Save(); err != nil {
		t.Fatal(err)
	}
	loaded := NewJar()
	if err := loaded.Load(); err != nil {
		t.Fatalf("exe 目录下的 cookies 应能加载: %v", err)
	}
	if loaded.Count() != 1 {
		t.Fatalf("cookie 数量不对: %d", loaded.Count())
	}
	_, tail := accountFromCookies(loaded)
	if tail != "139" {
		t.Fatalf("账号尾号应为 139，got %q", tail)
	}
}

func TestBuildQuizOverview(t *testing.T) {
	useTempState(t)
	target := Target{Name: "数据结构", CourseID: "c1", ClazzID: "k1"}
	writeTestJSON(t, "scan.json", ScanResult{
		CourseID: "c1",
		Rows: []ScanRow{
			{Chapter: "ch1", Module: "insertvideo", Type: "video", Title: "视频附件"},
			{Chapter: "ch1", WorkID: "w1", Title: "第一章检测题"},
			{Chapter: "ch1", WorkID: "w1", Title: "第一章检测题"},
			{Chapter: "ch2", WorkID: "w2", Title: "第二章练习题"},
		},
	})
	writeTestJSON(t, "quizzes.json", []Quiz{{
		Index: 0,
		Title: "第一章检测题",
		Work:  WorkPoint{WorkID: "w1", ChapterID: "ch1"},
		Questions: []Question{{
			QID:       "q1",
			TypeLabel: "单选题",
			Stem:      "题干",
			Score:     "2",
			Options:   []Option{{Value: "A", Text: "甲"}, {Value: "B", Text: "乙"}},
		}},
	}})

	ov, err := buildQuizOverview(target)
	if err != nil {
		t.Fatalf("buildQuizOverview: %v", err)
	}
	if !ov.Scanned {
		t.Fatal("有 scan.json 时应标记为已扫描")
	}
	if len(ov.Items) != 2 {
		t.Fatalf("作业列表应按 workId 去重后为 2，实际 %d", len(ov.Items))
	}
	if ov.Items[0].WorkID != "w1" || ov.Items[0].ChapterIndex != 1 {
		t.Fatalf("第一项不对: %+v", ov.Items[0])
	}
	if !ov.Items[0].Fetched || ov.Items[0].QuestionCount != 1 {
		t.Fatalf("第一项应已抓题且 1 题: %+v", ov.Items[0])
	}
	if ov.Items[1].Fetched || ov.Items[1].ChapterIndex != 2 {
		t.Fatalf("第二项应未抓题、第 2 章: %+v", ov.Items[1])
	}
	if ov.FetchedCount != 1 || ov.QuestionCount != 1 {
		t.Fatalf("统计不对: fetched=%d questions=%d", ov.FetchedCount, ov.QuestionCount)
	}

	detail, err := buildQuizQuestions("w1")
	if err != nil {
		t.Fatalf("buildQuizQuestions: %v", err)
	}
	if len(detail.Questions) != 1 || len(detail.Questions[0].Options) != 2 {
		t.Fatalf("题目或选项数量不对: %+v", detail)
	}
	if _, err := buildQuizQuestions("missing"); err == nil {
		t.Fatal("没有抓题的 workId 应返回错误")
	}

	if err := os.Remove(statePath("scan.json")); err != nil {
		t.Fatal(err)
	}
	ov, err = buildQuizOverview(target)
	if err != nil {
		t.Fatalf("buildQuizOverview（未扫描）: %v", err)
	}
	if ov.Scanned || len(ov.Items) != 0 {
		t.Fatalf("没有 scan.json 时应提示先扫描: %+v", ov)
	}
}

func TestSelectCourseByURLRejectsBadURL(t *testing.T) {
	useTempState(t)
	if _, err := SelectCourseByURL("https://example.com/course"); err == nil || !strings.Contains(err.Error(), "courseid") {
		t.Fatalf("缺少 courseid 应报错，实际: %v", err)
	}
}

func TestForgetCredentialsRemovesSavedCred(t *testing.T) {
	useTempState(t)
	if err := saveCreds("20260001", "secret"); err != nil {
		t.Fatalf("保存凭据失败: %v", err)
	}
	info, err := ForgetCredentials()
	if err != nil {
		t.Fatalf("忘记凭据失败: %v", err)
	}
	if info.CredSaved {
		t.Fatalf("忘记凭据后 credSaved 仍为 true")
	}
	if !strings.Contains(info.Message, "忘记") && !strings.Contains(info.Message, "没有") {
		t.Fatalf("提示不像忘记凭据: %s", info.Message)
	}
}

func TestClearLocalSessionRemovesOnlyCookies(t *testing.T) {
	_ = InitState()
	useTempState(t)
	if err := os.MkdirAll(filepath.Dir(statePath("cookies.json")), 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	if err := os.WriteFile(statePath("cookies.json"), []byte(`{"smartcourse.hust.edu.cn":{}}`), 0o644); err != nil {
		t.Fatalf("写 cookie 失败: %v", err)
	}
	writeTestJSON(t, "target.json", Target{Name: "课程", CourseID: "c1", ClazzID: "k1"})

	info, err := ClearLocalSession()
	if err != nil {
		t.Fatalf("清理登录态失败: %v", err)
	}
	if fileExists(statePath("cookies.json")) {
		t.Fatalf("cookie 文件没有被清掉")
	}
	if info.LoggedIn {
		t.Fatalf("清理后仍然显示已登录")
	}
	if !strings.Contains(info.Message, "重新登录") {
		t.Fatalf("提示不像要求重新登录: %s", info.Message)
	}
	if _, ok := loadTargetIfExists(); !ok {
		t.Fatalf("课程配置不应该被清理")
	}
}
