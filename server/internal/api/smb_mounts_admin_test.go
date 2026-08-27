// [V7 §9.4 UI-first] smb-mounts admin API 测试：密码脱敏 + CRUD + mount/unmount 端点。
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"xmedia/internal/domain"
)

// fakeMountService 内存版 smbMountService（仅用于 handler 层测试）。
// 行为对齐真实 Service.Mount：ID==0 时先持久化（repo.Create）再置 mounted。
type fakeMountService struct {
	last *domain.SMBMount
	repo domain.SMBMountRepository
	err  error
}

func (f *fakeMountService) Mount(_ context.Context, m *domain.SMBMount) error {
	if f.err != nil {
		return f.err
	}
	if f.repo != nil && m.ID == 0 {
		id, err := f.repo.Create(context.Background(), m)
		if err != nil {
			return err
		}
		m.ID = id
	}
	if m.ID == 0 {
		m.ID = 1
	}
	m.State = domain.SMBMountStateMounted
	f.last = m
	return nil
}

func (f *fakeMountService) Unmount(_ context.Context, id int64) error {
	if f.err != nil {
		return f.err
	}
	return nil
}

func (f *fakeMountService) RefreshState(_ context.Context) error { return f.err }

func TestRedactSMBURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"smb://user:pass@192.168.7.154/BTORAGE", "smb://user:%2A%2A%2A@192.168.7.154/BTORAGE"},
		{"//192.168.7.154/BTORAGE", "//192.168.7.154/BTORAGE"},
		{"smb://nouser@host/share", "smb://nouser@host/share"},
		{"", ""},
		{"not a url", "not a url"},
	}
	for _, c := range cases {
		if got := redactSMBURL(c.in); got != c.want {
			t.Errorf("redactSMBURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// fakeSMBRepoAPI 给 handler 层测试用（简单内存实现）。
type fakeSMBRepoAPI struct {
	byID map[int64]*domain.SMBMount
	next int64
}

func newFakeSMBRepoAPI() *fakeSMBRepoAPI {
	return &fakeSMBRepoAPI{byID: map[int64]*domain.SMBMount{}, next: 1}
}

func (f *fakeSMBRepoAPI) Create(_ context.Context, m *domain.SMBMount) (int64, error) {
	id := f.next
	f.next++
	m.ID = id
	f.byID[id] = m
	return id, nil
}
func (f *fakeSMBRepoAPI) Update(_ context.Context, m *domain.SMBMount) error {
	f.byID[m.ID] = m
	return nil
}
func (f *fakeSMBRepoAPI) Delete(_ context.Context, id int64) error {
	delete(f.byID, id)
	return nil
}
func (f *fakeSMBRepoAPI) Get(_ context.Context, id int64) (*domain.SMBMount, error) {
	m, ok := f.byID[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return m, nil
}
func (f *fakeSMBRepoAPI) List(_ context.Context) ([]*domain.SMBMount, error) {
	out := make([]*domain.SMBMount, 0, len(f.byID))
	for _, m := range f.byID {
		out = append(out, m)
	}
	return out, nil
}
func (f *fakeSMBRepoAPI) UpdateRuntime(_ context.Context, id int64, state domain.SMBMountState, lastErr string) error {
	if m, ok := f.byID[id]; ok {
		m.State = state
		m.LastError = lastErr
		now := time.Now()
		m.LastCheckedAt = &now
	}
	return nil
}

func newSMBMountTestHandler() *smbMountAdminHandlers {
	repo := newFakeSMBRepoAPI()
	return &smbMountAdminHandlers{
		repo:    repo,
		service: &fakeMountService{repo: repo},
	}
}

func doRequest(h *smbMountAdminHandlers, method, path string, body any) *httptest.ResponseRecorder {
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rd)
	rec := httptest.NewRecorder()
	switch {
	case method == http.MethodGet && path == "/api/admin/smb-mounts":
		h.list(rec, req)
	case method == http.MethodPost && path == "/api/admin/smb-mounts":
		h.create(rec, req)
	case method == http.MethodPost && path == "/api/admin/smb-mounts/refresh":
		h.refresh(rec, req)
	}
	return rec
}

func TestSMBMountAdmin_CreateAndList(t *testing.T) {
	h := newSMBMountTestHandler()

	rec := doRequest(h, http.MethodPost, "/api/admin/smb-mounts", map[string]any{
		"name":        "Asia-Movie",
		"smb_url":     "smb://user:pass@192.168.7.154/BTORAGE",
		"remote_path": "Asia-Movie",
		"mount_point": "/mnt/nas-root/Asia-Movie",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("create code = %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Success bool         `json:"success"`
		Data    smbMountView `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Fatalf("create not success: %s", rec.Body.String())
	}
	if resp.Data.ID == 0 {
		t.Fatal("应回填 ID")
	}
	if resp.Data.SMBURL == "smb://user:pass@192.168.7.154/BTORAGE" {
		t.Fatalf("SMB URL 应脱敏, got %q", resp.Data.SMBURL)
	}

	rec = doRequest(h, http.MethodGet, "/api/admin/smb-mounts", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list code = %d", rec.Code)
	}
	var listResp struct {
		Data []smbMountView `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Data) != 1 {
		t.Fatalf("list len = %d, want 1", len(listResp.Data))
	}
	if listResp.Data[0].State != string(domain.SMBMountStateMounted) {
		t.Fatalf("state = %q, want mounted", listResp.Data[0].State)
	}
}

func TestSMBMountAdmin_CreateInvalid(t *testing.T) {
	h := newSMBMountTestHandler()
	rec := doRequest(h, http.MethodPost, "/api/admin/smb-mounts", map[string]any{
		"name":        "",
		"smb_url":     "not-a-url",
		"mount_point": "relative",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("无效输入应返回 400 (CodeValidation), got %d: %s", rec.Code, rec.Body.String())
	}
	var resp Resp
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Success {
		t.Fatalf("无效输入应失败: %s", rec.Body.String())
	}
}

func TestSMBMountAdmin_Unwired(t *testing.T) {
	h := &smbMountAdminHandlers{} // 未接线
	rec := doRequest(h, http.MethodGet, "/api/admin/smb-mounts", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("未接线应返回 500 (CodeInternal), got %d", rec.Code)
	}
}
