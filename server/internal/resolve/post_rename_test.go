package resolve

import (
	"context"
	"errors"
	"testing"

	"xmedia/internal/domain"
	"xmedia/internal/driver"
)

// mockRenamer 在 mockDriver 基础上增加 RenameFile 行为, 用于测试 applyPostSaveRename.
// 复用 layers_test.go 的 mockDriver (fields+methods) 只需要追加 RenameFile.
type mockRenamer struct {
	mockDriver
	renameCalls []string // 顺序记录尝试过的 newName
	renameErrs  map[string]error
}

func (m *mockRenamer) RenameFile(_ context.Context, _ string, newName string) error {
	m.renameCalls = append(m.renameCalls, newName)
	if err, ok := m.renameErrs[newName]; ok {
		return err
	}
	return nil
}

func newServiceWithConfig(cfgValues map[string]string) *Service {
	if cfgValues == nil {
		cfgValues = map[string]string{}
	}
	return &Service{
		configs: &mockConfigRepo{values: cfgValues},
	}
}

func TestApplyPostSaveRename_DisabledByDefault(t *testing.T) {
	// 配置项未设置: 跳过重命名, 不调用 driver
	svc := newServiceWithConfig(nil)
	drv := &mockRenamer{}
	task := &domain.ResolveTask{Title: "Inception", Year: 2010, ExternalID: 27205}
	saved := &driver.ShareResult{FileID: "F1", FileName: "Inception.2010.1080p.BluRay.x264.mkv"}
	svc.applyPostSaveRename(context.Background(), drv, task, saved)
	if len(drv.renameCalls) != 0 {
		t.Fatalf("rename 应被禁用, 实际调用 %v", drv.renameCalls)
	}
	if saved.FileName != "Inception.2010.1080p.BluRay.x264.mkv" {
		t.Fatalf("FileName 不应被改动, 实际 %q", saved.FileName)
	}
}

func TestApplyPostSaveRename_DisabledWhenFalse(t *testing.T) {
	svc := newServiceWithConfig(map[string]string{domain.ConfigPanRenameEnabled: "false"})
	drv := &mockRenamer{}
	task := &domain.ResolveTask{Title: "Inception", Year: 2010, ExternalID: 27205}
	saved := &driver.ShareResult{FileID: "F1", FileName: "x.mkv"}
	svc.applyPostSaveRename(context.Background(), drv, task, saved)
	if len(drv.renameCalls) != 0 {
		t.Fatalf("关闭时不应调用, 实际 %v", drv.renameCalls)
	}
}

func TestApplyPostSaveRename_MovieSuccess(t *testing.T) {
	// 电影: 转存后文件名被改写为 {Title} ({Year}) {tmdb-XXX}.mkv
	svc := newServiceWithConfig(map[string]string{domain.ConfigPanRenameEnabled: "true"})
	drv := &mockRenamer{}
	task := &domain.ResolveTask{
		Title:      "Inception",
		Year:       2010,
		ExternalID: 27205,
	}
	saved := &driver.ShareResult{
		FileID:   "F1",
		FileName: "Inception.2010.1080p.BluRay.x264-GROUP.mkv",
	}
	svc.applyPostSaveRename(context.Background(), drv, task, saved)
	if len(drv.renameCalls) != 1 {
		t.Fatalf("应调用 1 次, 实际 %d 次: %v", len(drv.renameCalls), drv.renameCalls)
	}
	got := drv.renameCalls[0]
	if got != "Inception (2010) {tmdb-27205}.mkv" {
		t.Fatalf("目标名不符: got %q, want %q", got, "Inception (2010) {tmdb-27205}.mkv")
	}
	if saved.FileName != got {
		t.Fatalf("saved.FileName 未更新: %q", saved.FileName)
	}
}

func TestApplyPostSaveRename_TVEpisodeSuccess(t *testing.T) {
	// 剧集: 模板应为 {Title} S{ss}E{ee} {tmdb-XXX}.mkv
	svc := newServiceWithConfig(map[string]string{domain.ConfigPanRenameEnabled: "true"})
	drv := &mockRenamer{}
	task := &domain.ResolveTask{
		Title:      "Breaking Bad",
		Year:       2008,
		Season:     1,
		Episode:    1,
		ExternalID: 1396,
	}
	saved := &driver.ShareResult{
		FileID:   "F1",
		FileName: "Breaking.Bad.S01E01.720p.WEB-DL.mkv",
	}
	svc.applyPostSaveRename(context.Background(), drv, task, saved)
	if len(drv.renameCalls) != 1 {
		t.Fatalf("应调用 1 次, 实际 %d 次: %v", len(drv.renameCalls), drv.renameCalls)
	}
	got := drv.renameCalls[0]
	// BuildTargetFilename 实际格式: title (year) {tmdb-XXX} SxxExx
	if got != "Breaking Bad (2008) {tmdb-1396} S01E01.mkv" {
		t.Fatalf("目标名不符: got %q, want %q", got, "Breaking Bad (2008) {tmdb-1396} S01E01.mkv")
	}
}

func TestApplyPostSaveRename_ConflictFallbackToV2(t *testing.T) {
	// 目标名已存在: 失败 -> 回退 __v2
	svc := newServiceWithConfig(map[string]string{domain.ConfigPanRenameEnabled: "true"})
	drv := &mockRenamer{
		renameErrs: map[string]error{
			"Inception (2010) {tmdb-27205}.mkv": errors.New("已存在"),
		},
	}
	task := &domain.ResolveTask{Title: "Inception", Year: 2010, ExternalID: 27205}
	saved := &driver.ShareResult{FileID: "F1", FileName: "x.mkv"}
	svc.applyPostSaveRename(context.Background(), drv, task, saved)
	if len(drv.renameCalls) != 2 {
		t.Fatalf("应尝试 2 次 (target + v2), 实际 %d 次: %v", len(drv.renameCalls), drv.renameCalls)
	}
	if drv.renameCalls[1] != "Inception (2010) {tmdb-27205}__v2.mkv" {
		t.Fatalf("第二次应尝试 __v2 后缀, 实际 %q", drv.renameCalls[1])
	}
	if saved.FileName != drv.renameCalls[1] {
		t.Fatalf("saved.FileName 未更新到 v2: %q", saved.FileName)
	}
}

func TestApplyPostSaveRename_AllVariantsFailed(t *testing.T) {
	// target + v2 + v3 全部失败: warning log, FileName 保留原值
	svc := newServiceWithConfig(map[string]string{domain.ConfigPanRenameEnabled: "true"})
	drv := &mockRenamer{
		renameErrs: map[string]error{
			"Inception (2010) {tmdb-27205}.mkv":     errors.New("e1"),
			"Inception (2010) {tmdb-27205}__v2.mkv": errors.New("e2"),
			"Inception (2010) {tmdb-27205}__v3.mkv": errors.New("e3"),
		},
	}
	task := &domain.ResolveTask{Title: "Inception", Year: 2010, ExternalID: 27205}
	saved := &driver.ShareResult{FileID: "F1", FileName: "x.mkv"}
	svc.applyPostSaveRename(context.Background(), drv, task, saved)
	if len(drv.renameCalls) != 3 {
		t.Fatalf("应尝试 3 次, 实际 %d 次", len(drv.renameCalls))
	}
	if saved.FileName != "x.mkv" {
		t.Fatalf("全部失败时 FileName 应保留原值, 实际 %q", saved.FileName)
	}
}

func TestApplyPostSaveRename_SkipsNonVideo(t *testing.T) {
	// .nfo / .jpg 不应重命名
	svc := newServiceWithConfig(map[string]string{domain.ConfigPanRenameEnabled: "true"})
	drv := &mockRenamer{}
	task := &domain.ResolveTask{Title: "Inception", Year: 2010, ExternalID: 27205}
	saved := &driver.ShareResult{FileID: "F1", FileName: "poster.jpg"}
	svc.applyPostSaveRename(context.Background(), drv, task, saved)
	if len(drv.renameCalls) != 0 {
		t.Fatalf("非视频不应重命名, 实际调用 %v", drv.renameCalls)
	}
}

func TestApplyPostSaveRename_SkipsNoExtension(t *testing.T) {
	// 无扩展名不应重命名 (避免破坏目录)
	svc := newServiceWithConfig(map[string]string{domain.ConfigPanRenameEnabled: "true"})
	drv := &mockRenamer{}
	task := &domain.ResolveTask{Title: "Inception", Year: 2010, ExternalID: 27205}
	saved := &driver.ShareResult{FileID: "F1", FileName: "somefolder"}
	svc.applyPostSaveRename(context.Background(), drv, task, saved)
	if len(drv.renameCalls) != 0 {
		t.Fatalf("无扩展名不应重命名, 实际调用 %v", drv.renameCalls)
	}
}

func TestApplyPostSaveRename_AlreadyOrganizedNoOp(t *testing.T) {
	// 文件名已经符合模板: 不重命名, 不调用 driver
	svc := newServiceWithConfig(map[string]string{domain.ConfigPanRenameEnabled: "true"})
	drv := &mockRenamer{}
	task := &domain.ResolveTask{Title: "Inception", Year: 2010, ExternalID: 27205}
	saved := &driver.ShareResult{
		FileID:   "F1",
		FileName: "Inception (2010) {tmdb-27205}.mkv",
	}
	svc.applyPostSaveRename(context.Background(), drv, task, saved)
	if len(drv.renameCalls) != 0 {
		t.Fatalf("已规范时不应调用, 实际 %v", drv.renameCalls)
	}
}

func TestApplyPostSaveRename_NilTaskAndSaved(t *testing.T) {
	// 防御: nil 不应 panic
	svc := newServiceWithConfig(map[string]string{domain.ConfigPanRenameEnabled: "true"})
	drv := &mockRenamer{}
	svc.applyPostSaveRename(context.Background(), drv, nil, nil)            // 双 nil
	svc.applyPostSaveRename(context.Background(), drv, &domain.ResolveTask{}, nil) // nil saved
	if len(drv.renameCalls) != 0 {
		t.Fatalf("nil 不应触发调用, 实际 %v", drv.renameCalls)
	}
}

func TestIsTruthy(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"true", true}, {"TRUE", true}, {"1", true}, {"yes", true},
		{"on", true}, {"y", true}, {"  true  ", true},
		{"false", false}, {"0", false}, {"", false}, {"no", false}, {"foo", false},
	}
	for _, c := range cases {
		if got := isTruthy(c.in); got != c.want {
			t.Errorf("isTruthy(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestIsVideoExtension(t *testing.T) {
	video := []string{"mp4", "MKV", "avi", "mov", "webm", "ts", "m2ts", "m4v", "iso"}
	not := []string{"jpg", "png", "nfo", "txt", "srt", "ass", ""}
	for _, e := range video {
		if !isVideoExtension(e) {
			t.Errorf("isVideoExtension(%q) = false, want true", e)
		}
	}
	for _, e := range not {
		if isVideoExtension(e) {
			t.Errorf("isVideoExtension(%q) = true, want false", e)
		}
	}
}

func TestAppendVariant(t *testing.T) {
	cases := []struct {
		in     string
		suffix string
		want   string
	}{
		{"Inception (2010) {tmdb-27205}.mkv", "__v2", "Inception (2010) {tmdb-27205}__v2.mkv"},
		{"noext", "__v2", "noext__v2"},
		{"a.b.c.mkv", "__v3", "a.b.c__v3.mkv"},
	}
	for _, c := range cases {
		if got := appendVariant(c.in, c.suffix); got != c.want {
			t.Errorf("appendVariant(%q, %q) = %q, want %q", c.in, c.suffix, got, c.want)
		}
	}
}
