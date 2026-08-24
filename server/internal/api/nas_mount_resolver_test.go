// [V7 §9.4 UI-first 实测回归] 用户在管理后台只填主机视角路径
// (/mnt/BTORAGE/Asia-Movie), 系统必须:
//  1. 从容器内 SMB 挂载源 (//ip/share) 自动推导别名并完成映射;
//  2. 把命中的别名映射持久化进 configs — 重启后仍生效,
//     且在「主机路径映射」界面可见可编辑;
//  3. 二次解析走 explicit 路径.

package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"xmedia/internal/domain"
	"xmedia/internal/indexengine"
)

// fakeCfgRepo api 包内最小 ConfigRepository 替身.
type fakeCfgRepo struct {
	v map[string]string
}

func newFakeCfgRepo() *fakeCfgRepo { return &fakeCfgRepo{v: map[string]string{}} }

func (f *fakeCfgRepo) Get(_ context.Context, k string) (string, bool, error) {
	v, ok := f.v[k]
	return v, ok, nil
}
func (f *fakeCfgRepo) Set(_ context.Context, k, val string) error { f.v[k] = val; return nil }
func (f *fakeCfgRepo) All(_ context.Context) (map[string]string, error) {
	out := map[string]string{}
	for k, val := range f.v {
		out[k] = val
	}
	return out, nil
}

func smbDetected() []domain.MountInfoEntry {
	return []domain.MountInfoEntry{
		{Filesystem: "cifs", MountTarget: "/mnt/nas-root", Source: "//192.168.7.154/BTORAGE"},
	}
}

func TestNASMountResolver_SMBAliasResolveAndPersist(t *testing.T) {
	cfgs := newFakeCfgRepo()
	r := &nasMountResolver{
		mounts:   domain.NASMountMap{},
		configs:  cfgs,
		detected: smbDetected(),
	}

	got, src := r.resolveWithSource("/mnt/BTORAGE/Asia-Movie")
	if got != "/mnt/nas-root/Asia-Movie" || src != "smb_alias" {
		t.Fatalf("首次解析 = (%q,%q), want (/mnt/nas-root/Asia-Movie, smb_alias)", got, src)
	}

	r.persistDerivedMapping(context.Background(), "/mnt/BTORAGE/Asia-Movie", got)

	// 持久化后: configs 有 nas_mount_/mnt/BTORAGE → /mnt/nas-root.
	key := domain.ConfigKeyPrefixNASMount + "/mnt/BTORAGE"
	if v, ok, _ := cfgs.Get(context.Background(), key); !ok || v != "/mnt/nas-root" {
		t.Fatalf("configs[%q] = (%q,%v), want /mnt/nas-root,true", key, v, ok)
	}
	// 内存缓存同步 → 二次解析走 explicit.
	if _, src2 := r.resolveWithSource("/mnt/BTORAGE/West-Movie"); src2 != "explicit" {
		t.Fatalf("持久化后二次解析 source = %q, want explicit", src2)
	}
}

func TestNASMountResolver_NoMountDetectedPassthrough(t *testing.T) {
	cfgs := newFakeCfgRepo()
	r := &nasMountResolver{mounts: domain.NASMountMap{}, configs: cfgs, detected: nil}

	got, src := r.resolveWithSource("/mnt/BTORAGE/Asia-Movie")
	if got != "/mnt/BTORAGE/Asia-Movie" || src != "passthrough" {
		t.Fatalf("无挂载时应 passthrough 原值, got (%q,%q)", got, src)
	}
	if len(cfgs.v) != 0 {
		t.Fatalf("passthrough 不应写任何映射, got %v", cfgs.v)
	}
}

// [实测回归] 全部启用源不可达时, 扫描必须在触发前被拦截 (400 + 原因),
// 而不是异步空跑后 done/0 文件让界面"无反应".
func TestIndexNASFullPrecheckBlocksWhenAllUnreachable(t *testing.T) {
	h := &indexAdminHandlers{
		engine: &indexengine.Service{},
		index:  nil,
		nasSources: func() fakeNASSourcesRepo {
			r := newFakeNASSourcesRepo()
			_, _ = r.Create(context.Background(), &domain.NASSource{Name: "a", Path: "/mnt/BTORAGE/x", Enabled: true})
			return r
		}(),
		resolver: &nasMountResolver{mounts: domain.NASMountMap{}, detected: nil},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/admin/index/nas/full", nil)
	rec := httptest.NewRecorder()
	h.handleIndexNASFull(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("全不可达时应 400 拦截, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "不可达") {
		t.Fatalf("错误信息应说明不可达原因: %s", rec.Body.String())
	}
}

func TestNASMountReresolveAllRewritesStalePaths(t *testing.T) {
	h := &Handler{
		nasSources:      newFakeNASSourcesRepo(),
		nasMountResolver: &nasMountResolver{
			mounts:   domain.NASMountMap{},
			configs:  newFakeCfgRepo(),
			detected: smbDetected(),
		},
	}
	repo := h.nasSources.(fakeNASSourcesRepo)
	id1, _ := repo.Create(context.Background(), &domain.NASSource{Name: "Asia", Path: "/mnt/BTORAGE/Asia-Movie", Enabled: true})

	results, hint := h.reresolveAllPaths(context.Background())
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	if !results[0].Changed {
		t.Fatalf("存量宿主路径应被改写")
	}
	if hint != "" {
		t.Fatalf("有 SMB 挂载时 deploy_hint 应为空, got %q", hint)
	}
	cur, _ := repo.Get(context.Background(), id1)
	if cur.Path != "/mnt/nas-root/Asia-Movie" {
		t.Fatalf("path = %q, want /mnt/nas-root/Asia-Movie", cur.Path)
	}
}

// [实测回归] 无 SMB 挂载时 reresolve 必须返回 deploy_hint + 逐条即时可达性,
// 让前端能解释"为什么点了没变绿"而不是让用户对着 not_accessible 猜.
func TestNASMountReresolveAllDeployHintWhenNoMount(t *testing.T) {
	h := &Handler{
		nasSources:      newFakeNASSourcesRepo(),
		nasMountResolver: &nasMountResolver{
			mounts:   domain.NASMountMap{},
			configs:  newFakeCfgRepo(),
			detected: nil, // 容器无任何 SMB 挂载
		},
	}
	repo := h.nasSources.(fakeNASSourcesRepo)
	id1, _ := repo.Create(context.Background(), &domain.NASSource{Name: "Asia", Path: "/mnt/BTORAGE/Asia-Movie", Enabled: true})

	results, hint := h.reresolveAllPaths(context.Background())
	if hint == "" {
		t.Fatalf("无挂载时 deploy_hint 不应为空")
	}
	if !strings.Contains(hint, "NAS_MEDIA_PATH") {
		t.Fatalf("deploy_hint 应含 NAS_MEDIA_PATH 指引, got %q", hint)
	}
	if results[0].Changed {
		t.Fatalf("passthrough 下不应改写")
	}
	if results[0].Accessible != string(domain.NASAccessibilityNotAccessible) {
		t.Fatalf("accessible = %q, want not_accessible", results[0].Accessible)
	}
	got, _ := repo.Get(context.Background(), id1)
	if got.LastAccessibility != domain.NASAccessibilityNotAccessible {
		t.Fatalf("UpdateHealth 应回写 not_accessible, got %q", got.LastAccessibility)
	}
}

// fakeNASSourcesRepo 内存版 NAS source 仓储替身.
type fakeNASSourcesRepo struct {
	m      map[int64]*domain.NASSource
	nextID int64
}

func newFakeNASSourcesRepo() fakeNASSourcesRepo {
	return fakeNASSourcesRepo{m: map[int64]*domain.NASSource{}}
}

func (f fakeNASSourcesRepo) Create(_ context.Context, s *domain.NASSource) (int64, error) {
	f.nextID++
	s.ID = f.nextID
	cp := *s
	f.m[s.ID] = &cp
	return s.ID, nil
}
func (f fakeNASSourcesRepo) Update(_ context.Context, s *domain.NASSource) error {
	cp := *s
	f.m[s.ID] = &cp
	return nil
}
func (f fakeNASSourcesRepo) Delete(_ context.Context, id int64) error {
	delete(f.m, id)
	return nil
}
func (f fakeNASSourcesRepo) Get(_ context.Context, id int64) (*domain.NASSource, error) {
	if s, ok := f.m[id]; ok {
		cp := *s
		return &cp, nil
	}
	return nil, domain.Errorf(domain.CodeNotFound, "not found")
}
func (f fakeNASSourcesRepo) List(_ context.Context) ([]*domain.NASSource, error) {
	out := make([]*domain.NASSource, 0, len(f.m))
	for _, s := range f.m {
		cp := *s
		out = append(out, &cp)
	}
	return out, nil
}
func (f fakeNASSourcesRepo) ListEnabled(ctx context.Context) ([]*domain.NASSource, error) {
	all, _ := f.List(ctx)
	out := make([]*domain.NASSource, 0, len(all))
	for _, s := range all {
		if s.Enabled {
			out = append(out, s)
		}
	}
	return out, nil
}
func (f fakeNASSourcesRepo) PathTaken(_ context.Context, path string, excludeID int64) (bool, error) {
	for id, s := range f.m {
		if s.Path == path && id != excludeID {
			return true, nil
		}
	}
	return false, nil
}
func (f fakeNASSourcesRepo) NameTaken(_ context.Context, name string, excludeID int64) (bool, error) {
	for id, s := range f.m {
		if s.Name == name && id != excludeID {
			return true, nil
		}
	}
	return false, nil
}
func (f fakeNASSourcesRepo) UpdateHealth(_ context.Context, id int64, acc domain.NASAccessibility, count int64, at time.Time) error {
	if s, ok := f.m[id]; ok {
		s.LastAccessibility = acc
		s.FileCount = count
		t := at
		s.LastCheckedAt = &t
	}
	return nil
}
