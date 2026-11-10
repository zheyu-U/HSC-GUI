package hsc

// GUI 课程页视图的纯本地测试：不发起网络请求，只验证 course.json / scan.json
// 到前端结构的映射，以及播放通过后写回缓存的行为。

import (
	"path/filepath"
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
