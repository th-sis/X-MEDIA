package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"xmedia/internal/domain"
)

// nasMountAdminHandlers [V7 整改 commit #4] NAS 主机路径 -> 容器路径 映射管理。
//
// 端点：
//   GET    /api/admin/nas-mounts                  - 列出所有映射 + 自动探测结果
//   POST   /api/admin/nas-mounts                  - 新增一条映射 {host_path, container_path}
//   PUT    /api/admin/nas-mounts/{host_path}    - 更新一条映射的 container_path
//   DELETE /api/admin/nas-mounts/{host_path}    - 删除一条映射
//   POST   /api/admin/nas-mounts/probe           - 强制重新探测 /proc/self/mountinfo
//   POST   /api/admin/nas-mounts/resolve         - body {path} -> {resolved, source} 校验
//
// 存哪里：configs 表 key=nas_mount_<host_path>，value=container_path。
// 配合 domain.LoadNASMountMap / domain.RenderNASMountMap。

type nasMountAdminHandlers struct {
	configs  domain.ConfigRepository
	resolver *nasMountResolver
}

// nasMountView 列表 / 详情展示态。
type nasMountView struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
}

// nasMountListView 列表响应（含探测 + 配置）。
type nasMountListView struct {
	Configured []nasMountView  `json:"configured"`
	Detected   []domain.MountInfoEntry `json:"detected"`
}

// handleNASMountsList GET /api/admin/nas-mounts
func (h *nasMountAdminHandlers) handleNASMountsList(w http.ResponseWriter, r *http.Request) {
	if h.configs == nil {
		writeErr(w, domain.Errorf(domain.CodeInternal, "configs repo not wired"))
		return
	}
	all, err := h.configs.All(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	mounts := domain.LoadNASMountMap(all)
	out := nasMountListView{Configured: []nasMountView{}, Detected: []domain.MountInfoEntry{}}
	for host, container := range mounts {
		out.Configured = append(out.Configured, nasMountView{
			HostPath:      host,
			ContainerPath: container,
		})
	}
	if h.resolver != nil {
		out.Detected = h.resolver.detected
	}
	writeOK(w, out)
}

// handleNASMountsCreate POST /api/admin/nas-mounts {host_path, container_path}
func (h *nasMountAdminHandlers) handleNASMountsCreate(w http.ResponseWriter, r *http.Request) {
	if h.configs == nil {
		writeErr(w, domain.Errorf(domain.CodeInternal, "configs repo not wired"))
		return
	}
	var body struct {
		HostPath      string `json:"host_path"`
		ContainerPath string `json:"container_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, domain.Wrap(domain.CodeValidation, err))
		return
	}
	host := strings.TrimRight(strings.TrimSpace(body.HostPath), "/")
	container := strings.TrimRight(strings.TrimSpace(body.ContainerPath), "/")
	if host == "" || !strings.HasPrefix(host, "/") {
		writeErr(w, domain.Errorf(domain.CodeValidation, "host_path 必须是绝对路径且非空"))
		return
	}
	if container == "" || !strings.HasPrefix(container, "/") {
		writeErr(w, domain.Errorf(domain.CodeValidation, "container_path 必须是绝对路径且非空"))
		return
	}
	key := domain.ConfigKeyPrefixNASMount + host
	if err := h.configs.Set(r.Context(), key, container); err != nil {
		writeErr(w, err)
		return
	}
	// 更新 resolver 内存缓存（让同一进程后续请求立刻生效，避免重启）。
	if h.resolver != nil {
		h.resolver.mounts[host] = container
	}
	writeOK(w, nasMountView{HostPath: host, ContainerPath: container})
}

// handleNASMountsUpdate PUT /api/admin/nas-mounts/{host_path} {container_path}
func (h *nasMountAdminHandlers) handleNASMountsUpdate(w http.ResponseWriter, r *http.Request) {
	if h.configs == nil {
		writeErr(w, domain.Errorf(domain.CodeInternal, "configs repo not wired"))
		return
	}
	host := strings.TrimRight(chiURLParam(r, "host_path"), "/")
	if host == "" || !strings.HasPrefix(host, "/") {
		writeErr(w, domain.Errorf(domain.CodeValidation, "host_path 必须是绝对路径"))
		return
	}
	var body struct {
		ContainerPath string `json:"container_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, http.ErrBodyReadAfterClose) {
		writeErr(w, domain.Wrap(domain.CodeValidation, err))
		return
	}
	container := strings.TrimRight(strings.TrimSpace(body.ContainerPath), "/")
	if container == "" || !strings.HasPrefix(container, "/") {
		writeErr(w, domain.Errorf(domain.CodeValidation, "container_path 必须是绝对路径且非空"))
		return
	}
	key := domain.ConfigKeyPrefixNASMount + host
	if err := h.configs.Set(r.Context(), key, container); err != nil {
		writeErr(w, err)
		return
	}
	if h.resolver != nil {
		h.resolver.mounts[host] = container
	}
	writeOK(w, nasMountView{HostPath: host, ContainerPath: container})
}

// handleNASMountsDelete DELETE /api/admin/nas-mounts/{host_path}
func (h *nasMountAdminHandlers) handleNASMountsDelete(w http.ResponseWriter, r *http.Request) {
	if h.configs == nil {
		writeErr(w, domain.Errorf(domain.CodeInternal, "configs repo not wired"))
		return
	}
	host := strings.TrimRight(chiURLParam(r, "host_path"), "/")
	if host == "" || !strings.HasPrefix(host, "/") {
		writeErr(w, domain.Errorf(domain.CodeValidation, "host_path 必须是绝对路径"))
		return
	}
	key := domain.ConfigKeyPrefixNASMount + host
	// [V7 整改 commit #4] ConfigRepository 没 Delete —— 用 Set("") 模拟删除, LoadNASMountMap 会过滤空值。
	if err := h.configs.Set(r.Context(), key, ""); err != nil {
		writeErr(w, err)
		return
	}
	if h.resolver != nil {
		delete(h.resolver.mounts, host)
	}
	writeOK(w, map[string]any{"host_path": host, "deleted": true})
}

// handleNASMountsProbe POST /api/admin/nas-mounts/probe - 强制重新探测 /proc/self/mountinfo
func (h *nasMountAdminHandlers) handleNASMountsProbe(w http.ResponseWriter, r *http.Request) {
	if h.resolver == nil {
		writeErr(w, domain.Errorf(domain.CodeInternal, "resolver not wired"))
		return
	}
	detected, err := domain.ProbeNASMounts()
	if err != nil {
		writeErr(w, err)
		return
	}
	h.resolver.detected = detected
	writeOK(w, map[string]any{"detected": detected, "count": len(detected)})
}

// handleNASMountsResolve POST /api/admin/nas-mounts/resolve {path} -> {resolved, source}
// 调试 / 验证用：让前端能给用户立刻看到某条主机路径会 rewrite 成什么。
func (h *nasMountAdminHandlers) handleNASMountsResolve(w http.ResponseWriter, r *http.Request) {
	if h.resolver == nil {
		writeErr(w, domain.Errorf(domain.CodeInternal, "resolver not wired"))
		return
	}
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, domain.Wrap(domain.CodeValidation, err))
		return
	}
	resolved, source := domain.ResolveNASPath(body.Path, h.resolver.mounts, h.resolver.detected)
	writeOK(w, map[string]any{
		"input":    body.Path,
		"resolved": resolved,
		"source":   source, // explicit / auto_detected / passthrough
	})
}
