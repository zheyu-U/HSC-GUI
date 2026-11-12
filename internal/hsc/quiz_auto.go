package hsc

// 作业与测验自动答题的 GUI 编排：读配置、跑答题循环、汇总结果、推事件。
// LLM 调用、答案解析、题库规则仍在 vendored 的上游文件里
// （quiz_llm.go / quiz_solve.go / quiz_bank.go / quiz_peek.go / config_toml.go），
// 本文件不复制协议逻辑，也不直接打印。

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
)

// QuizSettings 自动答题设置，对应 state/config.toml（与 CLI 同一份）。
// apiKey 只用于保存：留空表示保留原来的 Key；读取时只给 apiKeyMasked，
// 不回传明文，避免前端把脱敏值再存回去。
type QuizSettings struct {
	Enabled        bool     `json:"enabled"`
	BaseURL        string   `json:"baseUrl"`
	APIKeyMasked   string   `json:"apiKeyMasked"`
	APIKey         string   `json:"apiKey"`
	Model          string   `json:"model"`
	Temperature    float64  `json:"temperature"`
	MaxTokens      int      `json:"maxTokens"`
	TimeoutSeconds int      `json:"timeoutSeconds"`
	AutoSubmit     bool     `json:"autoSubmit"`
	Types          []string `json:"types"`
	DelayMs        int      `json:"delayMs"`
	Retries        int      `json:"retries"`
	Message        string   `json:"message"`
}

// quizAllowedTypes 与上游题型归一（questionKind）保持一致。
var quizAllowedTypes = []string{"choice", "multi", "judge", "fill", "essay"}

// quizDefaultTypes 与上游 config_toml.go 的默认 types 一致。
var quizDefaultTypes = []string{"choice", "multi", "judge", "fill"}

// GetQuizSettings 读取 state/config.toml；文件缺失时按上游默认值生成。
func GetQuizSettings() (QuizSettings, error) {
	if err := InitState(); err != nil {
		return QuizSettings{}, err
	}
	return getQuizSettings()
}

// getQuizSettings 是 GetQuizSettings 的本地部分，单独拆出来方便测试。
func getQuizSettings() (QuizSettings, error) {
	cfg, _, err := loadConfig()
	if err != nil {
		return QuizSettings{}, fmt.Errorf("读取 state/config.toml 失败: %w", err)
	}
	out := QuizSettings{
		Enabled:        cfg.LLM.Enabled,
		BaseURL:        cfg.LLM.BaseURL,
		APIKeyMasked:   maskKey(cfg.LLM.APIKey),
		APIKey:         "",
		Model:          cfg.LLM.Model,
		Temperature:    cfg.LLM.Temperature,
		MaxTokens:      cfg.LLM.MaxTokens,
		TimeoutSeconds: cfg.LLM.TimeoutSeconds,
		AutoSubmit:     cfg.Quiz.AutoSubmit,
		Types:          append([]string{}, cfg.Quiz.Types...),
		DelayMs:        cfg.Quiz.DelayMs,
		Retries:        cfg.Quiz.Retries,
	}
	switch {
	case !cfg.LLM.Enabled:
		out.Message = "自动答题未启用：填好接口地址和 API Key 后打开启用开关。"
	case strings.TrimSpace(cfg.LLM.APIKey) == "":
		out.Message = "还没有填 API Key，自动答题跑不起来。"
	default:
		out.Message = "设置已加载；保存时 API Key 留空表示沿用原来的值。"
	}
	return out, nil
}

// SaveQuizSettings 保存自动答题设置到 state/config.toml。
// apiKey 为空时保留原值；字段不合法时返回能直接展示的中文提示。
func SaveQuizSettings(in QuizSettings) (QuizSettings, error) {
	if err := InitState(); err != nil {
		return QuizSettings{}, err
	}
	return saveQuizSettings(in)
}

// saveQuizSettings 是 SaveQuizSettings 的本地部分，单独拆出来方便测试。
func saveQuizSettings(in QuizSettings) (QuizSettings, error) {
	cfg, _, err := loadConfig()
	if err != nil {
		return QuizSettings{}, fmt.Errorf("读取 state/config.toml 失败: %w", err)
	}

	// 空 apiKey 表示保留配置文件里的原值，不把脱敏值写回去。
	key := strings.TrimSpace(in.APIKey)
	if key == "" {
		key = cfg.LLM.APIKey
	}
	baseURL := strings.TrimSpace(in.BaseURL)
	model := strings.TrimSpace(in.Model)
	if err := validateQuizSettings(in, baseURL, model, key); err != nil {
		return QuizSettings{}, err
	}
	types, err := normalizeQuizTypes(in.Types)
	if err != nil {
		return QuizSettings{}, err
	}

	cfg.LLM = LLMConfig{
		Enabled:        in.Enabled,
		BaseURL:        baseURL,
		APIKey:         key,
		Model:          model,
		Temperature:    in.Temperature,
		MaxTokens:      in.MaxTokens,
		TimeoutSeconds: in.TimeoutSeconds,
	}
	cfg.Quiz = QuizConfig{
		AutoSubmit: in.AutoSubmit,
		Types:      types,
		DelayMs:    in.DelayMs,
		Retries:    in.Retries,
	}

	data, err := toml.Marshal(cfg)
	if err != nil {
		return QuizSettings{}, fmt.Errorf("生成配置内容失败: %w", err)
	}
	content := "# hsc 配置（GUI 保存，CLI 读同一份）\n" +
		"# 自动答题（hsc quiz solve）使用 [llm] 里的接口，格式兼容 OpenAI\n\n" + string(data)
	if err := os.WriteFile(configPath(), []byte(content), 0o600); err != nil {
		return QuizSettings{}, fmt.Errorf("保存 state/config.toml 失败: %w", err)
	}

	out, err := getQuizSettings()
	if err != nil {
		return QuizSettings{}, err
	}
	out.Message = "设置已保存。"
	return out, nil
}

// validateQuizSettings 校验用户输入，错误文案直接给前端展示。
func validateQuizSettings(in QuizSettings, baseURL, model, key string) error {
	if in.Temperature < 0 || in.Temperature > 2 {
		return fmt.Errorf("温度要在 0 到 2 之间，一般填 0.2")
	}
	if in.MaxTokens <= 0 || in.MaxTokens > 128000 {
		return fmt.Errorf("max_tokens 要在 1 到 128000 之间，一般填 4096")
	}
	if in.TimeoutSeconds <= 0 || in.TimeoutSeconds > 600 {
		return fmt.Errorf("超时时间要在 1 到 600 秒之间")
	}
	if in.DelayMs < 0 || in.DelayMs > 60000 {
		return fmt.Errorf("每题间隔要在 0 到 60000 毫秒之间")
	}
	if in.Retries < 0 || in.Retries > 10 {
		return fmt.Errorf("单题重试次数要在 0 到 10 之间")
	}
	if baseURL != "" {
		u, err := url.Parse(baseURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("接口地址要写成 http:// 或 https:// 开头的完整地址，例如 https://api.openai.com/v1")
		}
	}
	if len([]rune(model)) > 120 {
		return fmt.Errorf("模型名太长，检查是不是粘错了内容")
	}
	if in.Enabled {
		if baseURL == "" {
			return fmt.Errorf("启用自动答题前，先填接口地址")
		}
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("启用自动答题前，先填 API Key")
		}
	}
	return nil
}

// normalizeQuizTypes 去空白、去重，并校验题型取值；空列表按上游默认值处理。
func normalizeQuizTypes(in []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, t := range in {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		allowed := false
		for _, a := range quizAllowedTypes {
			if a == t {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("题型只能填 choice、multi、judge、fill、essay，收到 %q", t)
		}
		seen[t] = true
		out = append(out, t)
	}
	if len(out) == 0 {
		out = append([]string{}, quizDefaultTypes...)
	}
	return out, nil
}

// QuizSolveResult 一轮自动答题的统计。
type QuizSolveResult struct {
	Total   int    `json:"total"`
	OK      int    `json:"ok"`
	Cached  int    `json:"cached"`
	Failed  int    `json:"failed"`
	Message string `json:"message"`
}

// QuizEvent 答题进度，经 Wails Events 推给前端（quiz:progress）。
type QuizEvent struct {
	State    string `json:"state"`
	Message  string `json:"message"`
	Index    int    `json:"index"`
	Total    int    `json:"total"`
	Question string `json:"question"`
	Answered int    `json:"answered"`
	Cached   int    `json:"cached"`
	Failed   int    `json:"failed"`
}

var (
	quizSolveMu      sync.Mutex
	quizSolveRunning bool
)

// SolveQuizzes 自动作答：only >= 0 只做该序号的一份，only < 0 做全部。
// 每答一题通过 onEvent 推送进度；结束统计写入 answers.json 与 qbank。
func SolveQuizzes(only int, onEvent func(QuizEvent)) (QuizSolveResult, error) {
	if err := InitState(); err != nil {
		return QuizSolveResult{}, err
	}
	return solveQuizzes(only, onEvent)
}

// solveQuizzes 是 SolveQuizzes 的本地部分，单独拆出来方便测试。
// 循环与重试策略参考上游 cmdQuizSolve（见上游 go/quiz_solve.go）。
func solveQuizzes(only int, onEvent func(QuizEvent)) (QuizSolveResult, error) {
	quizSolveMu.Lock()
	if quizSolveRunning {
		quizSolveMu.Unlock()
		return QuizSolveResult{}, fmt.Errorf("自动答题已在进行，等这一轮结束再试")
	}
	quizSolveRunning = true
	quizSolveMu.Unlock()
	defer func() {
		quizSolveMu.Lock()
		quizSolveRunning = false
		quizSolveMu.Unlock()
	}()

	emit := func(ev QuizEvent) {
		if onEvent != nil {
			onEvent(ev)
		}
	}
	fail := func(format string, args ...any) (QuizSolveResult, error) {
		msg := fmt.Sprintf(format, args...)
		emit(QuizEvent{State: "error", Message: msg})
		return QuizSolveResult{}, fmt.Errorf("%s", msg)
	}

	cfg, _, err := loadConfig()
	if err != nil {
		return fail("读取 state/config.toml 失败：%v", err)
	}
	if !cfg.LLM.Enabled {
		return fail("自动答题未启用：先去测验设置打开启用开关，并填好接口地址和 API Key")
	}
	if strings.TrimSpace(cfg.LLM.APIKey) == "" {
		return fail("自动答题没有 API Key：先去测验设置填好后再试")
	}

	var quizzes []Quiz
	if err := readJSON(statePath("quizzes.json"), &quizzes); err != nil {
		return fail("还没有抓取题目：先在作业与测验页点“抓取题目”")
	}

	// 已有答案先读出来合并，不覆盖之前答过的题（与上游一致）。
	answers := map[string]map[string]Answer{}
	_ = readJSON(statePath("answers.json"), &answers)
	bank := loadBank()

	allowed := func(kind string) bool {
		for _, t := range cfg.Quiz.Types {
			if t == kind {
				return true
			}
		}
		return false
	}

	// 先按题型过滤并统计总题数，进度事件才有确定的分母。
	selected := make([]Quiz, 0, len(quizzes))
	total := 0
	for _, q := range quizzes {
		if only >= 0 && q.Index != only {
			continue
		}
		selected = append(selected, q)
		for _, qq := range q.Questions {
			if allowed(questionKind(qq)) {
				total++
			}
		}
	}
	if only >= 0 && len(selected) == 0 {
		return fail("没有找到序号为 %d 的测验：先重新抓取题目再试", only)
	}

	emit(QuizEvent{State: "starting", Total: total,
		Message: fmt.Sprintf("准备作答 %d 份测验，共 %d 题", len(selected), total)})

	ok, cached, failed := 0, 0, 0
	done := 0
	for _, q := range selected {
		key := fmt.Sprint(q.Index)
		if answers[key] == nil {
			answers[key] = map[string]Answer{}
		}
		for _, qq := range q.Questions {
			kind := questionKind(qq)
			if !allowed(kind) {
				continue
			}
			done++
			question := firstNonEmpty(qq.QID, truncate(qq.Stem, 20))
			emit(QuizEvent{State: "solving", Index: done, Total: total, Question: question,
				Answered: ok, Cached: cached, Failed: failed,
				Message: fmt.Sprintf("正在作答第 %d/%d 题：%s", done, total, truncate(qq.Stem, 30))})

			if hit, found := answerFromBank(bank, qq, kind); found {
				raw, _ := jsonMarshal(hit)
				answers[key][qq.QID] = Answer(raw)
				ok++
				cached++
				emit(QuizEvent{State: "answer", Index: done, Total: total, Question: question,
					Answered: ok, Cached: cached, Failed: failed,
					Message: fmt.Sprintf("题库命中，跳过调用接口：%s", fmtAns(hit))})
				continue
			}

			ans, err := solveOne(cfg, qq, kind)
			if err != nil {
				failed++
				emit(QuizEvent{State: "answer", Index: done, Total: total, Question: question,
					Answered: ok, Cached: cached, Failed: failed,
					Message: fmt.Sprintf("作答失败，已跳过：%v", err)})
				continue
			}
			storeAnswer(bank, qq, kind, ans)
			raw, _ := jsonMarshal(ans)
			answers[key][qq.QID] = Answer(raw)
			ok++
			emit(QuizEvent{State: "answer", Index: done, Total: total, Question: question,
				Answered: ok, Cached: cached, Failed: failed,
				Message: fmt.Sprintf("已作答：%s", fmtAns(ans))})
			sleepSec(float64(cfg.Quiz.DelayMs) / 1000.0)
		}
	}

	if err := writeJSON(statePath("answers.json"), answers); err != nil {
		return fail("答案写入 state/quiz/answers.json 失败：%v", err)
	}
	if err := saveBank(bank); err != nil {
		return fail("题库写入 state/quiz/qbank.json 失败：%v", err)
	}

	message := fmt.Sprintf("作答 %d 题，成功 %d（题库命中 %d），失败 %d", total, ok, cached, failed)
	if total == 0 {
		message = "没有需要作答的题目：检查设置里的题型，或先抓取题目"
	}
	// 与上游 cmdQuizSolve 一致：开了自动交卷且做全部时，答完直接交卷。
	if cfg.Quiz.AutoSubmit && only < 0 {
		sub, serr := submitQuizzes(-1)
		if serr != nil {
			message += "；自动交卷失败：" + serr.Error()
		} else {
			message += "；" + sub.Message
		}
	}

	emit(QuizEvent{State: "done", Index: total, Total: total,
		Answered: ok, Cached: cached, Failed: failed, Message: message})
	return QuizSolveResult{Total: total, OK: ok, Cached: cached, Failed: failed, Message: message}, nil
}

// QuizSubmitResult 交卷结果。
type QuizSubmitResult struct {
	OK      int    `json:"ok"`
	Skipped int    `json:"skipped"`
	Failed  int    `json:"failed"`
	Message string `json:"message"`
}

// SubmitQuizzes 交卷；only >= 0 只交指定序号的一份，only < 0 交全部。
// 交卷协议在上游 cmdQuizSubmit（见上游 go/quiz.go），这里只解析它写进日志的统计。
func SubmitQuizzes(only int) (QuizSubmitResult, error) {
	if err := InitState(); err != nil {
		return QuizSubmitResult{}, err
	}
	return submitQuizzes(only)
}

// submitQuizzes 是 SubmitQuizzes 的本地部分，单独拆出来方便测试。
func submitQuizzes(only int) (QuizSubmitResult, error) {
	mark := markQuizLog()
	if err := cmdQuizSubmit(only); err != nil {
		msg := "交卷失败：" + err.Error()
		return QuizSubmitResult{Message: msg}, fmt.Errorf("%s", msg)
	}
	if ok, skipped, failed, found := readQuizSubmitCounts(mark); found {
		return QuizSubmitResult{
			OK:      ok,
			Skipped: skipped,
			Failed:  failed,
			Message: fmt.Sprintf("提交完成：成功 %d，跳过 %d，失败 %d", ok, skipped, failed),
		}, nil
	}
	return QuizSubmitResult{
		Message: "已调用交卷，日志里没有统计；到作业与测验页重新抓取题目确认提交状态。",
	}, nil
}

// 上游 logf 把运行日志追加到 state/gcli.log（见 internal/hsc/util.go），
// 这里同时兼容 state/logs/gcli.log，只解析本次调用期间新增的部分。
var reQuizSubmitDone = regexp.MustCompile(`提交完成：成功\s*(\d+)，跳过\s*(\d+)，失败\s*(\d+)`)

// quizLogMark 记录调用前的日志长度，避免读到上一次的"提交完成"。
type quizLogMark map[string]int64

func quizLogPaths() []string {
	return []string{statePath("gcli.log"), statePath("logs", "gcli.log")}
}

func markQuizLog() quizLogMark {
	mark := quizLogMark{}
	for _, p := range quizLogPaths() {
		if fi, err := os.Stat(p); err == nil {
			mark[p] = fi.Size()
		} else {
			mark[p] = -1
		}
	}
	return mark
}

// readQuizSubmitCounts 从本次调用新增的日志里找最后一条提交统计。
func readQuizSubmitCounts(mark quizLogMark) (ok, skipped, failed int, found bool) {
	for _, p := range quizLogPaths() {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		switch prev := mark[p]; {
		case prev < 0:
			// 调用前没有这个文件，整个文件都是本次写的。
		case int64(len(b)) > prev:
			b = b[prev:]
		default:
			continue
		}
		matches := reQuizSubmitDone.FindAllStringSubmatch(string(b), -1)
		if len(matches) == 0 {
			continue
		}
		last := matches[len(matches)-1]
		ok, _ = strconv.Atoi(last[1])
		skipped, _ = strconv.Atoi(last[2])
		failed, _ = strconv.Atoi(last[3])
		return ok, skipped, failed, true
	}
	return 0, 0, 0, false
}

// QuizBankInfo 题库题量。
type QuizBankInfo struct {
	Count   int    `json:"count"`
	Message string `json:"message"`
}

// GetQuizBank 读取 state/quiz/qbank.json 的题量。
func GetQuizBank() (QuizBankInfo, error) {
	if err := InitState(); err != nil {
		return QuizBankInfo{}, err
	}
	return getQuizBank()
}

// getQuizBank 是 GetQuizBank 的本地部分，单独拆出来方便测试。
func getQuizBank() (QuizBankInfo, error) {
	bank := loadBank()
	info := QuizBankInfo{Count: len(bank)}
	if info.Count == 0 {
		info.Message = "题库为空，自动答题时会边答边积累，重复题目不用再调接口。"
	} else {
		info.Message = fmt.Sprintf("题库共 %d 题，自动答题会优先用题库，命中时不调用接口。", info.Count)
	}
	return info, nil
}

// ClearQuizBank 清空题库缓存（写一份空 Bank）。
func ClearQuizBank() (QuizBankInfo, error) {
	if err := InitState(); err != nil {
		return QuizBankInfo{}, err
	}
	return clearQuizBank()
}

// clearQuizBank 是 ClearQuizBank 的本地部分，单独拆出来方便测试。
func clearQuizBank() (QuizBankInfo, error) {
	if err := saveBank(Bank{}); err != nil {
		return QuizBankInfo{}, fmt.Errorf("清空题库失败: %w", err)
	}
	return QuizBankInfo{Count: 0, Message: "题库已清空。"}, nil
}

// QuizPeekItem 一份测验的批阅页状态。
type QuizPeekItem struct {
	Index  int    `json:"index"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// PeekQuizzes 逐份读取批阅页标题判断状态：已批阅 / 已提交 / 可作答 / 未知。
// 只读；某一页打不开时该项记未知，不中断整轮。
func PeekQuizzes(only int) ([]QuizPeekItem, error) {
	if err := InitState(); err != nil {
		return nil, err
	}
	return peekQuizzes(only)
}

// peekQuizzes 是 PeekQuizzes 的本地部分，单独拆出来方便测试。
func peekQuizzes(only int) ([]QuizPeekItem, error) {
	var quizzes []Quiz
	if err := readJSON(statePath("quizzes.json"), &quizzes); err != nil {
		return nil, fmt.Errorf("还没有抓取题目：先在作业与测验页点“抓取题目”，再查看批阅结果")
	}
	items := []QuizPeekItem{}
	for _, q := range quizzes {
		if only >= 0 && q.Index != only {
			continue
		}
		item := QuizPeekItem{
			Index:  q.Index,
			Title:  firstNonEmpty(q.Title, "未命名作业或测验"),
			Status: "未知",
		}
		// 页面标题能区分状态：做作业 / 查看作业 / 查看已批阅作业；
		// 打不开或平台错误页保持未知，不影响其它测验。
		if _, _, body, err := c0Follow(q.PageURL, ""); err == nil {
			pageTitle := ""
			if m := reTitlePage.FindStringSubmatch(string(body)); m != nil {
				pageTitle = strings.TrimSpace(m[1])
			}
			item.Status = quizStatusFromTitle(pageTitle)
		}
		items = append(items, item)
	}
	return items, nil
}

// quizStatusFromTitle 把平台页面标题归成四种状态。
func quizStatusFromTitle(title string) string {
	switch {
	case strings.Contains(title, "已批阅"):
		return "已批阅"
	case strings.Contains(title, "查看作业"), strings.Contains(title, "已提交"):
		return "已提交"
	case strings.Contains(title, "做作业"):
		return "可作答"
	default:
		return "未知"
	}
}
