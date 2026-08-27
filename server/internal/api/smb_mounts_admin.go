package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"xmedia/internal/domain"
	"xmedia/internal/smbmount"
)

// smbMountAdminHandlers [V7 §9.4 UI-first] 容器内 SMB 挂载点管理。
//
// 用户在 NAS 配置页填 SMB URL（smb://user:pass@host/share）+ 容器内挂载点，
// 后端通过特权 mount.cifs 自动挂载并持久化 DB；重启时 ReattachOnStartup 重挂。
//
// 端点：
//   GET    /api/admin/smb-mounts                - 列出所有挂载点（含实时状态）
//   POST   /api/admin/smb-mounts                - 新增并立即挂载 {name, smb_url, remote_path, mount_point, uid, gid}
//   PUT    /api/admin/smb-mounts/{id}           - 更新配置并重新挂载
//   DELETE /api/admin/smb-mounts/{id}           - 卸载并删除
//   POST   /api/admin/smb-mounts/{id}/mount     - 手动挂载（重新挂载）
//   POST   /api/admin/smb-mounts/{id}/unmount   - 手动卸载
//   POST   /api/admin/smb-mounts/refresh        - 全量按 /proc/self/mounts 校准状态
type smbMountAdminHandlers struct {
	repo    domain.SMBMountRepository
	svc     *smbmount.Service
	service smbMountService
}

// smbMountService 抽取挂载/卸载/刷新，便于测试注入 mock。
type smbMountService interface {
	Mount(ctx context.Context, m *domain.SMBMount) error
	Unmount(ctx context.Context, id int64) error
	RefreshState(ctx context.Context) error
}

// smbMountView 返回给前端的展示态（SMB URL 密码脱敏，不泄露明文凭据）。
type smbMountView struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	SMBURL        string `json:"smb_url"`
	RemotePath    string `json:"remote_path"`
	MountPoint    string `json:"mount_point"`
	UID           int    `json:"uid"`
	GID           int    `json:"gid"`
	State         string `json:"state"`
	LastError     string `json:"last_error,omitempty"`
	LastCheckedAt *string `json:"last_checked_at,omitempty"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

func viewFromSMBMount(m *domain.SMBMount) smbMountView {
	v := smbMountView{
		ID:         m.ID,
		Name:       m.Name,
		SMBURL:     redactSMBURL(m.SMBURL),
		RemotePath: m.RemotePath,
		MountPoint: m.MountPoint,
		UID:        m.UID,
		GID:        m.GID,
		State:      string(m.State),
		LastError:  m.LastError,
		CreatedAt:  m.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:  m.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if m.LastCheckedAt != nil {
		ts := m.LastCheckedAt.UTC().Format(time.RFC3339)
		v.LastCheckedAt = &ts
	}
	return v
}

// redactSMBURL 把 smb://user:pass@host/share 中的密码替换为 ***。
// 非标准格式（无 userinfo / 已是 //host/share）原样返回。
func redactSMBURL(raw string) string {
	if raw == "" {
		return ""
	}
	if !strings.HasPrefix(raw, "smb://") {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	user := u.User.Username()
	if user == "" {
		return raw
	}
	if _, ok := u.User.Password(); !ok {
		return raw
	}
	u.User = url.UserPassword(user, "***")
	return u.String()
}

func (h *smbMountAdminHandlers) list(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeErr(w, domain.Errorf(domain.CodeInternal, "smb mounts repo not wired"))
		return
	}
	list, err := h.repo.List(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	out := make([]smbMountView, 0, len(list))
	for _, m := range list {
		out = append(out, viewFromSMBMount(m))
	}
	writeOK(w, out)
}

func (h *smbMountAdminHandlers) create(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil && h.service == nil {
		writeErr(w, domain.Errorf(domain.CodeInternal, "smb mount service not wired"))
		return
	}
	var body struct {
		Name       string `json:"name"`
		SMBURL     string `json:"smb_url"`
		RemotePath string `json:"remote_path"`
		MountPoint string `json:"mount_point"`
		UID        int    `json:"uid"`
		GID        int    `json:"gid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, domain.Wrap(domain.CodeValidation, err))
		return
	}
	m := &domain.SMBMount{
		Name:       strings.TrimSpace(body.Name),
		SMBURL:     strings.TrimSpace(body.SMBURL),
		RemotePath: strings.TrimSpace(body.RemotePath),
		MountPoint: strings.TrimSpace(body.MountPoint),
		UID:        body.UID,
		GID:        body.GID,
	}
	if err := m.Validate(); err != nil {
		writeErr(w, domain.Errorf(domain.CodeValidation, "%s", err.Error()))
		return
	}
	if err := h.mountSvc().Mount(r.Context(), m); err != nil {
		writeErr(w, domain.Errorf(domain.CodeValidation, "%s", err.Error()))
		return
	}
	writeOK(w, viewFromSMBMount(m))
}

func (h *smbMountAdminHandlers) update(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeErr(w, domain.Errorf(domain.CodeInternal, "smb mounts repo not wired"))
		return
	}
	id, err := parseIDFromPath(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	cur, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	var body struct {
		Name       *string `json:"name,omitempty"`
		SMBURL     *string `json:"smb_url,omitempty"`
		RemotePath *string `json:"remote_path,omitempty"`
		MountPoint *string `json:"mount_point,omitempty"`
		UID        *int    `json:"uid,omitempty"`
		GID        *int    `json:"gid,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, domain.Wrap(domain.CodeValidation, err))
		return
	}
	if body.Name != nil {
		cur.Name = strings.TrimSpace(*body.Name)
	}
	if body.SMBURL != nil {
		cur.SMBURL = strings.TrimSpace(*body.SMBURL)
	}
	if body.RemotePath != nil {
		cur.RemotePath = strings.TrimSpace(*body.RemotePath)
	}
	if body.MountPoint != nil {
		cur.MountPoint = strings.TrimSpace(*body.MountPoint)
	}
	if body.UID != nil {
		cur.UID = *body.UID
	}
	if body.GID != nil {
		cur.GID = *body.GID
	}
	if err := cur.Validate(); err != nil {
		writeErr(w, domain.Errorf(domain.CodeValidation, "%s", err.Error()))
		return
	}
	// 先持久化配置，再重新挂载（幂等：已挂载旧点会先被替换）。
	if err := h.repo.Update(r.Context(), cur); err != nil {
		writeErr(w, err)
		return
	}
	if h.mountSvc() != nil {
		if err := h.mountSvc().Mount(r.Context(), cur); err != nil {
			writeErr(w, domain.Errorf(domain.CodeValidation, "%s", err.Error()))
			return
		}
	}
	fresh, _ := h.repo.Get(r.Context(), id)
	if fresh == nil {
		fresh = cur
	}
	writeOK(w, viewFromSMBMount(fresh))
}

func (h *smbMountAdminHandlers) delete(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeErr(w, domain.Errorf(domain.CodeInternal, "smb mounts repo not wired"))
		return
	}
	id, err := parseIDFromPath(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	// 先卸载（失败不阻断删除，仅记录状态），再删 DB 记录。
	if h.mountSvc() != nil {
		_ = h.mountSvc().Unmount(r.Context(), id)
	}
	if err := h.repo.Delete(r.Context(), id); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, map[string]any{"deleted": id})
}

func (h *smbMountAdminHandlers) mount(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDFromPath(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	m, err := h.getOrErr(r, id)
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := h.mountSvc().Mount(r.Context(), m); err != nil {
		writeErr(w, domain.Errorf(domain.CodeValidation, "%s", err.Error()))
		return
	}
	writeOK(w, viewFromSMBMount(m))
}

func (h *smbMountAdminHandlers) unmount(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDFromPath(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := h.mountSvc().Unmount(r.Context(), id); err != nil {
		writeErr(w, domain.Errorf(domain.CodeValidation, "%s", err.Error()))
		return
	}
	m, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, viewFromSMBMount(m))
}

func (h *smbMountAdminHandlers) refresh(w http.ResponseWriter, r *http.Request) {
	if err := h.mountSvc().RefreshState(r.Context()); err != nil {
		writeErr(w, err)
		return
	}
	// 刷新后返回最新列表，前端一次请求即可同步。
	if h.repo != nil {
		list, err := h.repo.List(r.Context())
		if err != nil {
			writeErr(w, err)
			return
		}
		out := make([]smbMountView, 0, len(list))
		for _, m := range list {
			out = append(out, viewFromSMBMount(m))
		}
		writeOK(w, out)
		return
	}
	writeOK(w, []smbMountView{})
}

func (h *smbMountAdminHandlers) getOrErr(r *http.Request, id int64) (*domain.SMBMount, error) {
	if h.repo == nil {
		return nil, domain.Errorf(domain.CodeInternal, "smb mounts repo not wired")
	}
	m, err := h.repo.Get(r.Context(), id)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (h *smbMountAdminHandlers) mountSvc() smbMountService {
	if h.service != nil {
		return h.service
	}
	return h.svc
}
