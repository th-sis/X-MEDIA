package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"xmedia/internal/domain"
	"xmedia/internal/indexengine"
)

// [V7 整改 commit #4] NAS 主机路径 -> 容器路径 自动映射。
//
// 用户在管理后台填主机路径（/mnt/BTORAGE/Asia-Movie），
// 后端自动 prefix rewrite 成容器内路径（/mnt/nas-root/Asia-Movie），
// 然后存到 nas_sources.path（容器内路径是真相源）。
//
// 映射规则来源（优先级）：
//  1. 用户手动配置（configs 表 nas_mount_* KV）
//  2. 自动探测（/proc/self/mountinfo 启动快照）
//  3. 原样返回（假设用户已直接填容器内路径）

// nasMountResolver 持有 mount 映射 + 探测快照，给 handler 用。
// 启动时构造一次，handler 每次请求复用（性能）。
type nasMountResolver struct {
	// mounts 用户在 configs 表配置的 host_path -> container_path 映射。
	mounts domain.NASMountMap
	// configs 用于 smb_alias 命中后的映射持久化（可为 nil，仅内存态）。
	configs domain.ConfigRepository
	// detected 启动时 /proc/self/mountinfo 探测结果。
	detected []domain.MountInfoEntry
}

// newNASMountResolver 构造 resolver：探测 mountinfo + 读 configs mount map。
// 若探测失败（裸机部署无 /proc/self/mountinfo），返回 resolver + nil error，
// detected 留空 —— rewrite 仍可走 mounted 配置。
func newNASMountResolver(configs domain.ConfigRepository) *nasMountResolver {
	r := &nasMountResolver{
		mounts: domain.NASMountMap{},
	}
	if configs != nil {
		r.configs = configs
		if all, err := configs.All(context.Background()); err == nil {
			r.mounts = domain.LoadNASMountMap(all)
		}
	}
	if detected, err := domain.ProbeNASMounts(); err == nil {
		r.detected = detected
	}
	return r
}

// resolve 把用户填的路径 rewrite 成容器内路径。返回 rewrite 后的字符串。
// rewrite 失败（empty）时返回原值。
func (r *nasMountResolver) resolve(rawPath string) string {
	resolved, _ := r.resolveWithSource(rawPath)
	return resolved
}

// deployHint [V7 §9.4 UI-first] 容器内没有任何 SMB 挂载时给出部署侧指引 —
// Docker 无法在运行时追加 volume，挂载只能在容器创建时完成；把这一步的
// 操作说明直接给到用户，而不是让用户面对一句"路径不可访问"。
func (r *nasMountResolver) deployHint() string {
	if len(r.detected) > 0 {
		return ""
	}
	return "检测到容器内未挂载任何 NAS 卷：请在 NAS 主机上执行 export NAS_MEDIA_PATH=<宿主机SMB目录> && docker compose up -d --force-recreate xmedia 后重试。"
}

// resolveWithSource 同 resolve, 但额外返回映射来源:
// explicit / auto_detected / smb_alias / passthrough.
func (r *nasMountResolver) resolveWithSource(rawPath string) (string, string) {
	if rawPath == "" {
		return rawPath, "passthrough"
	}
	return domain.ResolveNASPath(rawPath, r.mounts, r.detected)
}

// persistDerivedMapping [V7 §9.4 UI-first] 当路径经由 SMB 别名推导命中时,
// 把该别名映射持久化进 configs (nas_mount_<alias> = target):
//   - 重启后仍生效 (resolver 启动时会 LoadNASMountMap);
//   - 在管理后台「主机路径映射」界面可见、可编辑、可删除.
// 同时同步内存缓存, 使后续请求直接走 explicit 路径.
func (r *nasMountResolver) persistDerivedMapping(ctx context.Context, raw, resolved string) {
	if r.configs == nil || len(r.detected) == 0 {
		return
	}
	for alias, target := range domain.DeriveNASMountMap(r.detected) {
		var rest string
		switch {
		case raw == alias:
			rest = ""
		case strings.HasPrefix(raw, alias+"/"):
			rest = strings.TrimPrefix(raw, alias)
		default:
			continue
		}
		if target+rest != resolved {
			continue
		}
		key := domain.ConfigKeyPrefixNASMount + alias
		if v, ok, _ := r.configs.Get(ctx, key); ok && v == target {
			if _, exists := r.mounts[alias]; !exists {
				r.mounts[alias] = target
			}
			return
		}
		if err := r.configs.Set(ctx, key, target); err == nil {
			r.mounts[alias] = target
		}
		return
	}
}

// reresolveResult 单条存量 source 的重解析结果.
type reresolveResult struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Old     string `json:"old_path"`
	New     string `json:"new_path"`
	Source  string `json:"source"`
	Changed bool   `json:"changed"`
	// Accessible 改写后的即时可达性 (ok/not_accessible) — 让前端点击后
	// 无需等 5 分钟周期监测即可看到状态变化.
	Accessible string `json:"accessible,omitempty"`
	Error      string `json:"error,omitempty"`
}

// reresolveAllPaths [V7 §9.4 UI-first] 对全部 NAS source 的存量 path 重新走
// 一遍映射改写 — 用于修复历史版本入库的主机视角路径 (如 /mnt/BTORAGE/*),
// 用户无需逐条手工编辑. 只改写, 不触碰 enabled 等字段.
// 每条改写成功后立即 stat 并回写 last_accessibility; 容器无 SMB 挂载时
// 返回的 deployHint 非空 (Docker volume 只能创建时挂载, 给出精确命令).
func (h *Handler) reresolveAllPaths(ctx context.Context) ([]reresolveResult, string) {
	out := []reresolveResult{}
	if h.nasSources == nil || h.nasMountResolver == nil {
		return out, ""
	}
	list, err := h.nasSources.List(ctx)
	if err != nil {
		return out, ""
	}
	now := time.Now()
	for _, src := range list {
		item := reresolveResult{ID: src.ID, Name: src.Name, Old: src.Path, New: src.Path}
		newPath, source := h.nasMountResolver.resolveWithSource(src.Path)
		item.Source = source
		if newPath != src.Path {
			changed := *src
			changed.Path = newPath
			if uerr := h.nasSources.Update(ctx, &changed); uerr != nil {
				item.Error = uerr.Error()
			} else {
				item.New = newPath
				item.Changed = true
				h.nasMountResolver.persistDerivedMapping(ctx, src.Path, newPath)
			}
		}
		// 即时健康回写: 无论是否改写, 都按当前 path stat 一次,
		// 让列表「可访问」列与本次操作同步刷新.
		cur, gerr := h.nasSources.Get(ctx, src.ID)
		if gerr == nil && cur != nil {
			acc := domain.NASAccessibilityNotAccessible
			if info, serr := os.Stat(cur.Path); serr == nil && info.IsDir() {
				acc = domain.NASAccessibilityOK
			}
			item.Accessible = string(acc)
			_ = h.nasSources.UpdateHealth(ctx, cur.ID, acc, cur.FileCount, now)
		}
		out = append(out, item)
	}
	return out, h.nasMountResolver.deployHint()
}

// nasSourceView 是返回给前端的展示态（[V7 §9.4+ 扩展] G1.C，G18 UI 用）。
type nasSourceView struct {
	ID                int64   `json:"id"`
	Name              string  `json:"name"`
	Path              string  `json:"path"`
	Enabled           bool    `json:"enabled"`
	FileCount         int64   `json:"file_count"`
	LastAccessibility string  `json:"last_accessibility"`
	LastCheckedAt     *string `json:"last_checked_at,omitempty"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
}

func viewFromNASSource(s *domain.NASSource) nasSourceView {
	v := nasSourceView{
		ID:                s.ID,
		Name:              s.Name,
		Path:              s.Path,
		Enabled:           s.Enabled,
		FileCount:         s.FileCount,
		LastAccessibility: string(s.LastAccessibility),
		CreatedAt:         s.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         s.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if s.LastCheckedAt != nil {
		ts := s.LastCheckedAt.UTC().Format(time.RFC3339)
		v.LastCheckedAt = &ts
	}
	return v
}

func (h *Handler) listNASSources(w http.ResponseWriter, r *http.Request) {
	repo := h.nasSources
	if repo == nil {
		writeErr(w, domain.Errorf(domain.CodeInternal, "NAS sources repo not wired"))
		return
	}
	list, err := repo.List(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	out := make([]nasSourceView, 0, len(list))
	for _, s := range list {
		out = append(out, viewFromNASSource(s))
	}
	writeOK(w, out)
}

func (h *Handler) createNASSource(w http.ResponseWriter, r *http.Request) {
	repo := h.nasSources
	if repo == nil {
		writeErr(w, domain.Errorf(domain.CodeInternal, "NAS sources repo not wired"))
		return
	}
	var body struct {
		Name    string `json:"name"`
		Path    string `json:"path"`
		Enabled *bool  `json:"enabled,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, domain.Wrap(domain.CodeValidation, err))
		return
	}
	name := strings.TrimSpace(body.Name)
	rawPath := strings.TrimSpace(body.Path)
	if name == "" {
		writeErr(w, domain.Errorf(domain.CodeValidation, "name 不能为空"))
		return
	}
	if !isAbsolutePath(rawPath) {
		writeErr(w, domain.Errorf(domain.CodeValidation, "path 必须是绝对路径"))
		return
	}
	// [V7 改造 commit #4] 主机路径 -> 容器路径自动 rewrite。
	// 用户在管理后台填主机路径（/mnt/BTORAGE/Asia-Movie），
	// 后端根据 configs 表映射 + /proc/self/mountinfo 自动转容器内路径。
	path := rawPath
	if h.nasMountResolver != nil {
		path = h.nasMountResolver.resolve(rawPath)
	}
	// [defensive] 若 resolve 未发生实际 rewrite（passthrough），
	// 说明缺少 host->container 映射规则；此时禁止把主机路径静默入库，
	// 否则 Capabilities/扫描会在容器内 os.Stat 失败。
	// 已正确 rewrite 的路径不在此处阻断（容器挂载延迟由部署侧处理）。
	if path == rawPath {
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			msg := fmt.Sprintf("路径 %q 在容器内不可访问，且未找到可用的主机路径映射；可直接使用容器内路径（如 /mnt/nas-root/...），或在「NAS 配置 → 主机路径映射」中添加映射", rawPath)
			if h.nasMountResolver != nil {
				if hint := h.nasMountResolver.deployHint(); hint != "" {
					msg += "。" + hint
				}
			}
			writeErr(w, domain.Errorf(domain.CodeValidation, "%s", msg))
			return
		}
	}
	if taken, err := repo.NameTaken(r.Context(), name, 0); err != nil {
		writeErr(w, err)
		return
	} else if taken {
		writeErr(w, domain.Errorf(domain.CodeValidation, "name 已被使用: %s", name))
		return
	}
	if taken, err := repo.PathTaken(r.Context(), path, 0); err != nil {
		writeErr(w, err)
		return
	} else if taken {
		writeErr(w, domain.Errorf(domain.CodeValidation, "path 已被使用: %s", path))
		return
	}
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	src := &domain.NASSource{Name: name, Path: path, Enabled: enabled}
	id, err := repo.Create(r.Context(), src)
	if err != nil {
		writeErr(w, err)
		return
	}
	src.ID = id
	writeOK(w, viewFromNASSource(src))
}

// nasSourceReresolveAll POST /api/admin/nas-sources/reresolve
// [V7 §9.4 UI-first] 存量 source 路径批量重映射：历史版本入库的主机视角路径
// (/mnt/BTORAGE/*) 一键改写为容器内路径，无需逐条手工编辑。
// 容器无 SMB 挂载时返回 deploy_hint，前端据此展示部署指引。
func (h *Handler) nasSourceReresolveAll(w http.ResponseWriter, r *http.Request) {
	results, deployHint := h.reresolveAllPaths(r.Context())
	changed := 0
	for _, it := range results {
		if it.Changed {
			changed++
		}
	}
	writeOK(w, map[string]any{
		"total":       len(results),
		"changed":     changed,
		"results":     results,
		"deploy_hint": deployHint,
	})
}

func (h *Handler) updateNASSource(w http.ResponseWriter, r *http.Request) {
	repo := h.nasSources
	if repo == nil {
		writeErr(w, domain.Errorf(domain.CodeInternal, "NAS sources repo not wired"))
		return
	}
	id, err := parseIDFromPath(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	var body struct {
		Name    *string `json:"name,omitempty"`
		Path    *string `json:"path,omitempty"`
		Enabled *bool   `json:"enabled,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, domain.Wrap(domain.CodeValidation, err))
		return
	}
	cur, err := repo.Get(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	if body.Name != nil {
		newName := strings.TrimSpace(*body.Name)
		if newName == "" {
			writeErr(w, domain.Errorf(domain.CodeValidation, "name 不能为空"))
			return
		}
		if taken, err := repo.NameTaken(r.Context(), newName, id); err != nil {
			writeErr(w, err)
			return
		} else if taken {
			writeErr(w, domain.Errorf(domain.CodeValidation, "name 已被使用: %s", newName))
			return
		}
		cur.Name = newName
	}
	if body.Path != nil {
		rawPath := strings.TrimSpace(*body.Path)
		if !isAbsolutePath(rawPath) {
			writeErr(w, domain.Errorf(domain.CodeValidation, "path 必须是绝对路径"))
			return
		}
		// [V7 改造 commit #4] 主机路径 -> 容器路径自动 rewrite.
		// [UI-first 增强] 命中别名映射时同步持久化（与 create 一致）。
		newPath := rawPath
		if h.nasMountResolver != nil {
			var src string
			newPath, src = h.nasMountResolver.resolveWithSource(rawPath)
			if src == "smb_alias" || src == "auto_detected" {
				h.nasMountResolver.persistDerivedMapping(r.Context(), rawPath, newPath)
			}
		}
		// [defensive] passthrough 且容器内不可达时拒绝，避免主机路径静默入库。
		if newPath == rawPath {
			if info, err := os.Stat(newPath); err != nil || !info.IsDir() {
				msg := fmt.Sprintf("路径 %q 在容器内不可访问，且未找到可用的主机路径映射；可直接使用容器内路径（如 /mnt/nas-root/...），或在「NAS 配置 → 主机路径映射」中添加映射", rawPath)
				if h.nasMountResolver != nil {
					if hint := h.nasMountResolver.deployHint(); hint != "" {
						msg += "。" + hint
					}
				}
				writeErr(w, domain.Errorf(domain.CodeValidation, "%s", msg))
				return
			}
		}
		if taken, err := repo.PathTaken(r.Context(), newPath, id); err != nil {
			writeErr(w, err)
			return
		} else if taken {
			writeErr(w, domain.Errorf(domain.CodeValidation, "path 已被使用: %s", newPath))
			return
		}
		cur.Path = newPath
	}
	if body.Enabled != nil {
		cur.Enabled = *body.Enabled
	}
	if err := repo.Update(r.Context(), cur); err != nil {
		writeErr(w, err)
		return
	}
	fresh, _ := repo.Get(r.Context(), id)
	if fresh == nil {
		fresh = cur
	}
	writeOK(w, viewFromNASSource(fresh))
}

func (h *Handler) deleteNASSource(w http.ResponseWriter, r *http.Request) {
	repo := h.nasSources
	if repo == nil {
		writeErr(w, domain.Errorf(domain.CodeInternal, "NAS sources repo not wired"))
		return
	}
	id, err := parseIDFromPath(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := repo.Delete(r.Context(), id); err != nil {
		writeErr(w, err)
		return
	}
	writeOK(w, map[string]any{"deleted": id})
}

func (h *Handler) toggleNASSource(w http.ResponseWriter, r *http.Request) {
	repo := h.nasSources
	if repo == nil {
		writeErr(w, domain.Errorf(domain.CodeInternal, "NAS sources repo not wired"))
		return
	}
	id, err := parseIDFromPath(r, "id")
	if err != nil {
		writeErr(w, err)
		return
	}
	cur, err := repo.Get(r.Context(), id)
	if err != nil {
		writeErr(w, err)
		return
	}
	cur.Enabled = !cur.Enabled
	if err := repo.Update(r.Context(), cur); err != nil {
		writeErr(w, err)
		return
	}
	fresh, _ := repo.Get(r.Context(), id)
	if fresh == nil {
		fresh = cur
	}
	writeOK(w, viewFromNASSource(fresh))
}

func (h *Handler) nasSourceTestPath(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.TrimSpace(r.URL.Query().Get("path"))
	if rawPath == "" {
		writeErr(w, domain.Errorf(domain.CodeValidation, "path 必须提供"))
		return
	}
	if !isAbsolutePath(rawPath) {
		writeErr(w, domain.Errorf(domain.CodeValidation, "path 必须是绝对路径"))
		return
	}
	// [V7 改造 commit #4] 使得用户在测试页填主机路径也能正确校验。
	path := rawPath
	if h.nasMountResolver != nil {
		path = h.nasMountResolver.resolve(rawPath)
	}
	info, err := os.Stat(path)
	if err != nil {
		writeJSON(w, http.StatusOK, Resp{
			Success:   false,
			Message:   "路径不可访问: " + err.Error(),
			ErrorType: string(domain.CodeValidation),
			Data: map[string]any{
				"path":       path,
				"exists":     false,
				"is_dir":     false,
				"file_count": int64(0),
			},
		})
		return
	}
	isDir := info.IsDir()
	if !isDir {
		writeJSON(w, http.StatusOK, Resp{
			Success:   false,
			Message:   "该路径不是目录",
			ErrorType: string(domain.CodeValidation),
			Data: map[string]any{
				"path":       path,
				"exists":     true,
				"is_dir":     false,
				"file_count": int64(0),
			},
		})
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		writeJSON(w, http.StatusOK, Resp{
			Success:   false,
			Message:   "无读取权限: " + err.Error(),
			ErrorType: string(domain.CodePermissionDenied),
			Data: map[string]any{
				"path":       path,
				"exists":     true,
				"is_dir":     true,
				"readable":   false,
				"file_count": int64(0),
			},
		})
		return
	}
	fileCount := int64(0)
	var samples []string
	for _, e := range entries {
		if !e.IsDir() {
			fileCount++
			continue
		}
		if len(samples) < 5 {
			samples = append(samples, e.Name())
		}
	}
	result := map[string]any{
		"path":       path,
		"exists":     true,
		"is_dir":     true,
		"readable":   true,
		"file_count": fileCount,
	}
	if len(samples) > 0 {
		result["sample"] = samples
	}
	writeOK(w, result)
}

func (h *Handler) nasSourceBulkHealth(w http.ResponseWriter, r *http.Request) {
	repo := h.nasSources
	if repo == nil {
		writeErr(w, domain.Errorf(domain.CodeInternal, "NAS sources repo not wired"))
		return
	}
	list, err := repo.List(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	now := time.Now().UTC()
	results := make([]map[string]any, 0, len(list))
	for _, src := range list {
		acc := domain.NASAccessibilityNotAccessible
		count := int64(0)
		// [V7 改造 commit #4] src.Path 已是容器内路径 (创建/更新时已 rewrite),
		// 这里直接 os.Stat 。
		// [G4 修正] 与 ScanNASFull 同口径: 用 filepath.WalkDir 递归数视频文件
		// (mkv/mp4/ts/avi/iso 等), 而不是只看一级子文件.
		// 这样 /mnt/STORAGE/Asia-Movie 这种顶级分类目录, 一级都是子目录也能正确数.
		if info, statErr := os.Stat(src.Path); statErr == nil && info.IsDir() {
			acc = domain.NASAccessibilityOK
			count = countVideoFiles(src.Path)
		}
		if err := repo.UpdateHealth(r.Context(), src.ID, acc, count, now); err != nil {
			results = append(results, map[string]any{
				"id":        src.ID,
				"name":      src.Name,
				"path":      src.Path,
				"status":    string(acc),
				"count":     count,
				"persisted": false,
			})
			continue
		}
		results = append(results, map[string]any{
			"id":        src.ID,
			"name":      src.Name,
			"path":      src.Path,
			"status":    string(acc),
			"count":     count,
			"persisted": true,
		})
	}
	writeOK(w, map[string]any{"checked": len(results), "results": results})
}

// countVideoFiles 递归遍历 root, 统计 indexengine.IsVideoFile() 认定的视频文件数.
// 与 ScanNASFull.discoverPhaseA 保持同一口径 (V7 §9.7.1 Phase A 视频文件白名单),
// 保证"全部可访问检测"的 count 与实际扫描入库的视频数一致.
//
// SMB 瞬时断开的容忍: 任何条目 stat 失败时, WalkDir 跳过该子树 (不向上抛 err),
// 不会因为一个坏文件就中断整轮统计.
func countVideoFiles(root string) int64 {
	var n int64
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 跳过不可读条目 (SMB 瞬时断开容忍, 与 ScanNASFull 一致)
		}
		if d.IsDir() {
			return nil
		}
		if indexengine.IsVideoFile(d.Name()) {
			n++
		}
		return nil
	})
	return n
}

func parseIDFromPath(r *http.Request, key string) (int64, error) {
	raw := chiURLParam(r, key)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, domain.Errorf(domain.CodeValidation, "id 无效: %s", raw)
	}
	return id, nil
}

func chiURLParam(r *http.Request, key string) string {
	if v := r.PathValue(key); v != "" {
		return v
	}
	return ""
}

func isAbsolutePath(p string) bool {
	if strings.HasPrefix(p, "/") {
		return true
	}
	if len(p) >= 3 && p[1] == ':' && (p[2] == '/' || p[2] == '\\') {
		c := p[0]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			return true
		}
	}
	return false
}
