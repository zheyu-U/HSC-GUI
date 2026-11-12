package hsc

// 作业与测验自动答题编排的本地测试：不发起网络请求，覆盖配置读写与脱敏、
// 题库统计和清空、题库命中的作答循环、参数错误的中文提示。

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// prepareQuizState 准备一个空的 state 目录。useTempState 只改了全局变量，
// 不建目录；loadConfig 首次生成 config.toml 时要求 state 目录已存在。
func prepareQuizState(t *testing.T) string {
	t.Helper()
	dir := useTempState(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("创建 state 目录失败: %v", err)
	}
	return dir
}

// workingQuizSettings 返回一份合法、启用自动答题的设置。
func workingQuizSettings() QuizSettings {
	return QuizSettings{
		Enabled:        true,
		BaseURL:        "https://api.example.com/v1",
		APIKey:         "sk-test-1234567890",
		Model:          "gpt-4o-mini",
		Temperature:    0.2,
		MaxTokens:      4096,
		TimeoutSeconds: 60,
		AutoSubmit:     false,
		Types:          []string{"choice", "multi", "judge"},
		DelayMs:        300,
		Retries:        2,
	}
}

func TestQuizSettingsSaveAndMask(t *testing.T) {
	prepareQuizState(t)

	saved, err := saveQuizSettings(workingQuizSettings())
	if err != nil {
		t.Fatalf("保存设置失败: %v", err)
	}
	if saved.APIKey != "" {
		t.Fatalf("读取时不能回传明文 apiKey，got %q", saved.APIKey)
	}
	if saved.APIKeyMasked == "" || strings.Contains(saved.APIKeyMasked, "1234567890") {
		t.Fatalf("apiKeyMasked 应是脱敏值，got %q", saved.APIKeyMasked)
	}

	// 配置文件里要保存真实 Key，CLI 才能直接读。
	cfg, _, err := loadConfig()
	if err != nil {
		t.Fatalf("读取配置失败: %v", err)
	}
	if cfg.LLM.APIKey != "sk-test-1234567890" {
		t.Fatalf("配置文件里应保存明文 Key，got %q", cfg.LLM.APIKey)
	}
	if !cfg.LLM.Enabled || cfg.LLM.BaseURL != "https://api.example.com/v1" || cfg.LLM.MaxTokens != 4096 {
		t.Fatalf("配置字段不对: %+v", cfg.LLM)
	}
	if len(cfg.Quiz.Types) != 3 || cfg.Quiz.Types[0] != "choice" {
		t.Fatalf("题型没保存对: %+v", cfg.Quiz.Types)
	}

	// 空 apiKey 表示保留原值。
	in := workingQuizSettings()
	in.APIKey = ""
	in.Model = "deepseek-chat"
	if _, err := saveQuizSettings(in); err != nil {
		t.Fatalf("保留原 Key 的保存失败: %v", err)
	}
	cfg, _, err = loadConfig()
	if err != nil {
		t.Fatalf("再次读取配置失败: %v", err)
	}
	if cfg.LLM.APIKey != "sk-test-1234567890" {
		t.Fatalf("空 apiKey 应保留原值，got %q", cfg.LLM.APIKey)
	}
	if cfg.LLM.Model != "deepseek-chat" {
		t.Fatalf("模型应更新，got %q", cfg.LLM.Model)
	}
}

func TestQuizSettingsValidation(t *testing.T) {
	prepareQuizState(t)

	cases := []struct {
		name string
		edit func(*QuizSettings)
		want string
	}{
		{"温度越界", func(s *QuizSettings) { s.Temperature = 5 }, "温度"},
		{"max_tokens 为 0", func(s *QuizSettings) { s.MaxTokens = 0 }, "max_tokens"},
		{"超时为 0", func(s *QuizSettings) { s.TimeoutSeconds = 0 }, "超时"},
		{"间隔为负", func(s *QuizSettings) { s.DelayMs = -1 }, "间隔"},
		{"重试为负", func(s *QuizSettings) { s.Retries = -1 }, "重试"},
		{"地址不是 http", func(s *QuizSettings) { s.BaseURL = "api.example.com" }, "接口地址"},
		{"题型非法", func(s *QuizSettings) { s.Types = []string{"unknown"} }, "题型"},
		{"启用但没 Key 且原值也为空", func(s *QuizSettings) { s.APIKey = "" }, "API Key"},
	}
	for _, c := range cases {
		in := workingQuizSettings()
		c.edit(&in)
		if _, err := saveQuizSettings(in); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: 期望包含 %q 的错误，got %v", c.name, c.want, err)
		}
	}

	// 题库里的题型去重、去空白后保存。
	in := workingQuizSettings()
	in.Types = []string{" choice ", "choice", "judge"}
	saved, err := saveQuizSettings(in)
	if err != nil {
		t.Fatalf("合法题型保存失败: %v", err)
	}
	if len(saved.Types) != 2 || saved.Types[0] != "choice" || saved.Types[1] != "judge" {
		t.Fatalf("题型应去重去空白，got %+v", saved.Types)
	}
}

func TestQuizBankCountAndClear(t *testing.T) {
	prepareQuizState(t)

	bank := Bank{
		"q1": {Kind: "choice", Stem: stemFingerprint("1+1=?"), Texts: []string{"2"}},
		"q2": {Kind: "judge", Stem: stemFingerprint("地球是圆的"), Answer: "true"},
	}
	if err := saveBank(bank); err != nil {
		t.Fatalf("写题库失败: %v", err)
	}
	info, err := getQuizBank()
	if err != nil {
		t.Fatalf("读题库失败: %v", err)
	}
	if info.Count != 2 {
		t.Fatalf("题库题量应为 2，got %d", info.Count)
	}
	if info.Message == "" {
		t.Fatal("题库非空时应有提示文案")
	}

	cleared, err := clearQuizBank()
	if err != nil {
		t.Fatalf("清空题库失败: %v", err)
	}
	if cleared.Count != 0 || cleared.Message == "" {
		t.Fatalf("清空结果不对: %+v", cleared)
	}
	after, err := getQuizBank()
	if err != nil {
		t.Fatalf("清空后读题库失败: %v", err)
	}
	if after.Count != 0 {
		t.Fatalf("清空后题库应为空，got %d", after.Count)
	}
	// 清空要写一份空 Bank，而不是删文件，CLI 读到的仍是空对象。
	var raw map[string]any
	if err := readJSON(statePath("qbank.json"), &raw); err != nil {
		t.Fatalf("清空后 qbank.json 应存在且可解析: %v", err)
	}
	if len(raw) != 0 {
		t.Fatalf("清空后 qbank.json 应为空对象，got %+v", raw)
	}
}

func TestSolveQuizzesUsesBankWithoutLLM(t *testing.T) {
	prepareQuizState(t)

	if _, err := saveQuizSettings(workingQuizSettings()); err != nil {
		t.Fatalf("准备配置失败: %v", err)
	}
	writeTestJSON(t, "quizzes.json", []Quiz{{
		Index: 0,
		Title: "第一章检测题",
		Work:  WorkPoint{WorkID: "w1", ChapterID: "ch1"},
		Questions: []Question{{
			QID:       "q1",
			TypeLabel: "单选题",
			Stem:      "1+1=?",
			Options:   []Option{{Value: "A", Text: "1"}, {Value: "B", Text: "2"}},
		}},
	}})
	if err := saveBank(Bank{"q1": {Kind: "choice", Stem: stemFingerprint("1+1=?"), Texts: []string{"2"}}}); err != nil {
		t.Fatalf("写题库失败: %v", err)
	}

	var events []QuizEvent
	res, err := solveQuizzes(0, func(ev QuizEvent) { events = append(events, ev) })
	if err != nil {
		t.Fatalf("题库命中时不应失败: %v", err)
	}
	if res.Total != 1 || res.OK != 1 || res.Cached != 1 || res.Failed != 0 {
		t.Fatalf("统计不对: %+v", res)
	}
	if !strings.Contains(res.Message, "题库命中 1") {
		t.Fatalf("结果文案应带题库命中数，got %q", res.Message)
	}

	// 题库存的是选项文本，复用时要按当前选项反查出字母 B。
	var answers map[string]map[string]Answer
	if err := readJSON(statePath("answers.json"), &answers); err != nil {
		t.Fatalf("应写出 answers.json: %v", err)
	}
	var got string
	if err := json.Unmarshal(answers["0"]["q1"], &got); err != nil {
		t.Fatalf("解析答案失败: %v", err)
	}
	if got != "B" {
		t.Fatalf("答案应为 B，got %q", got)
	}

	states := map[string]bool{}
	for _, ev := range events {
		states[ev.State] = true
	}
	for _, want := range []string{"starting", "solving", "answer", "done"} {
		if !states[want] {
			t.Fatalf("缺少 %s 事件: %+v", want, events)
		}
	}
	last := events[len(events)-1]
	if last.State != "done" || last.Total != 1 || last.Answered != 1 || last.Cached != 1 {
		t.Fatalf("done 事件字段不对: %+v", last)
	}
}

func TestSolveQuizzesValidation(t *testing.T) {
	prepareQuizState(t)

	// 默认配置 enabled=false。
	if _, err := solveQuizzes(-1, nil); err == nil || !strings.Contains(err.Error(), "启用") {
		t.Fatalf("未启用时应提示先打开启用开关，got %v", err)
	}

	if _, err := saveQuizSettings(workingQuizSettings()); err != nil {
		t.Fatalf("准备配置失败: %v", err)
	}
	// 启用但没抓题。
	if _, err := solveQuizzes(-1, nil); err == nil || !strings.Contains(err.Error(), "抓取题目") {
		t.Fatalf("没抓题时应提示先抓取题目，got %v", err)
	}

	writeTestJSON(t, "quizzes.json", []Quiz{{Index: 0, Title: "测验一"}})
	// 序号不存在。
	if _, err := solveQuizzes(3, nil); err == nil || !strings.Contains(err.Error(), "序号") {
		t.Fatalf("序号不存在时应给出提示，got %v", err)
	}

	// 一份测验但没有可作答题型时，也要正常收尾并给提示。
	in := workingQuizSettings()
	in.Types = []string{"choice"}
	if _, err := saveQuizSettings(in); err != nil {
		t.Fatalf("准备配置失败: %v", err)
	}
	res, err := solveQuizzes(0, nil)
	if err != nil {
		t.Fatalf("没有可作答题型时不应报错: %v", err)
	}
	if res.Total != 0 || !strings.Contains(res.Message, "没有需要作答的题目") {
		t.Fatalf("空作答结果的文案不对: %+v", res)
	}
}

func TestQuizStatusFromTitle(t *testing.T) {
	cases := map[string]string{
		"查看已批阅作业": "已批阅",
		"查看作业":   "已提交",
		"做作业":    "可作答",
		"500":    "未知",
		"":       "未知",
	}
	for title, want := range cases {
		if got := quizStatusFromTitle(title); got != want {
			t.Errorf("标题 %q: got %q want %q", title, got, want)
		}
	}
}
