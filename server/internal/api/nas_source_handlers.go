package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"xmedia/internal/domain"
)

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
	path := strings.TrimSpace(body.Path)
	if name == "" {
		writeErr(w, domain.Errorf(domain.CodeValidation, "name 不能为空"))
		return
	}
	if !isAbsolutePath(path) {
		writeErr(w, domain.Errorf(domain.CodeValidation, "path 必须是绝对路径"))
		return
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
		newPath := strings.TrimSpace(*body.Path)
		if !isAbsolutePath(newPath) {
			writeErr(w, domain.Errorf(domain.CodeValidation, "path 必须是绝对路径"))
			return
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
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		writeErr(w, domain.Errorf(domain.CodeValidation, "path 必须提供"))
		return
	}
	if !isAbsolutePath(path) {
		writeErr(w, domain.Errorf(domain.CodeValidation, "path 必须是绝对路径"))
		return
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
		if info, statErr := os.Stat(src.Path); statErr == nil && info.IsDir() {
			if entries, readErr := os.ReadDir(src.Path); readErr == nil {
				acc = domain.NASAccessibilityOK
				for _, e := range entries {
					if !e.IsDir() {
						count++
					}
				}
			}
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
