package pansearch

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"xmedia/internal/domain"
)

// Service PanSou 盘搜 HTTP 客户端（Sidecar 模式，localhost:8888）。
//
// 实现设计文档 V7 §8 PanSearchService 接口：
//   - §8.4 Search/CheckLinks/Health 三个方法（Search 在本文件；CheckLinks 在 checklinks.go；Health 在本文件）
//   - §8.5 质量排序 + 排序 + CAM 过滤实现于 checklinks.go 的 SortResults（本文件 Search 末尾调用）
//   - §8.5.1 缓存失效：link_count 字段 + 30 分钟跳过逻辑（本文件 Search 内部）
//   - §8.7 auth_token 鉴权（PanSou 启 AUTH_ENABLED 时）— 本文件 authHeader
//   - §8.8 降级：Health 检查 + P1 跳过（调用方控制，本文件 Health 方法）
type Service struct {
	httpClient *http.Client
	configs    domain.ConfigRepository
	cacheRepo  domain.PansearchCacheRepository
}

// NewService 按设计文档 §8.4 装配 Service。
func NewService(configs domain.ConfigRepository, cache domain.PansearchCacheRepository) *Service {
	return &Service{
		httpClient: &http.Client{Timeout: 8 * time.Second},
		configs:    configs,
		cacheRepo:  cache,
	}
}

// baseURL 从 configs 表读取 PanSou URL，默认 http://localhost:8888（§8.7）。
func (s *Service) baseURL(ctx context.Context) string {
	v, _, _ := s.configs.Get(ctx, domain.ConfigPansearchURL)
	if strings.TrimSpace(v) == "" {
		return domain.ConfigDefaults[domain.ConfigPansearchURL]
	}
	return strings.TrimRight(strings.TrimSpace(v), "/")
}

// authHeader [B-5, §8.7] 当 ConfigPansearchAuthOn=true 时返回 "Bearer <token>"，否则空。
// PanSou 启 AUTH_ENABLED 后所有 API（含 /api/health）都需此 header。
func (s *Service) authHeader(ctx context.Context) string {
	enabled, _, _ := s.configs.Get(ctx, domain.ConfigPansearchAuthOn)
	if strings.TrimSpace(strings.ToLower(enabled)) != "true" {
		return ""
	}
	token, _, _ := s.configs.Get(ctx, domain.ConfigPansearchToken)
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	return "Bearer " + token
}

// Health 检测 PanSou 可达性。
func (s *Service) Health(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL(ctx)+"/api/health", nil)
	if err != nil {
		return false
	}
	if h := s.authHeader(ctx); h != "" {
		req.Header.Set("Authorization", h)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// SearchRequest 搜索请求。
type SearchRequest struct {
	Keyword    string
	CloudTypes []string
}

// Search 调用 PanSou /api/search；不可达时返回空结果。
//
// 流程：
//  1. §8.5.1 缓存命中（1h 内 + link_count>0）→ 返回缓存
//  2. PanSou /api/search POST → 解析 merged_by_type → PanSearchResult
//  3. §8.5 调 checklinks.SortResults 应用 CAM 过滤 + 网盘优先级 + 质量 + 时间排序
//  4. §8.5.1 写缓存（link_count = 有效结果数）
func (s *Service) Search(ctx context.Context, req SearchRequest) ([]domain.PanSearchResult, error) {
	cloudTypes := strings.Join(req.CloudTypes, ",")

	// §8.5.1 读缓存
	if cached, cnt, at, err := s.cacheRepo.Get(ctx, req.Keyword, cloudTypes); err == nil {
		if cnt == 0 && at != nil && time.Since(*at) < 30*time.Minute {
			// stale 且未过 30 分钟，跳过缓存
		} else if at != nil && time.Since(*at) < time.Hour {
			var results []domain.PanSearchResult
			if json.Unmarshal([]byte(cached), &results) == nil {
				return s.sortResults(ctx, results), nil
			}
		}
	}

	body, err := json.Marshal(map[string]any{
		"kw":          req.Keyword,
		"cloud_types": req.CloudTypes,
		"res":         "merge",
		"src":         "all",
	})
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL(ctx)+"/api/search", strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if h := s.authHeader(ctx); h != "" {
		httpReq.Header.Set("Authorization", h)
	}
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil // PanSou 不可达，返回空（§8.8 降级）
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}
	var merged struct {
		MergedByType map[string][]struct {
			URL      string `json:"url"`
			Password string `json:"password"`
			Note     string `json:"note"`
			Datetime string `json:"datetime"`
			Source   string `json:"source"`
		} `json:"merged_by_type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&merged); err != nil {
		return nil, err
	}
	var results []domain.PanSearchResult
	for diskType, links := range merged.MergedByType {
		source := mapDiskType(diskType)
		for _, l := range links {
			results = append(results, domain.PanSearchResult{
				Title:    l.Note,
				Source:   source,
				ShareURL: l.URL,
				Password: l.Password,
				Datetime: l.Datetime,
				Quality:  detectQuality(l.Note),
				Format:   detectFormat(l.Note),
			})
		}
	}

	// §8.5 应用 CAM 过滤 + 网盘优先级 + 质量 + 时间排序
	results = s.sortResults(ctx, results)

	// §8.5.1 写缓存（link_count = 有效结果数）
	if data, err := json.Marshal(results); err == nil {
		_ = s.cacheRepo.Set(ctx, req.Keyword, cloudTypes, string(data), len(results))
	}
	return results, nil
}

// sortResults [§8.5] 从 configs 读取网盘优先级 + CAM 过滤开关，委托给 checklinks.SortResults。
func (s *Service) sortResults(ctx context.Context, results []domain.PanSearchResult) []domain.PanSearchResult {
	if len(results) == 0 {
		return results
	}
	// 网盘优先级：逗号分隔字符串 → slice
	priorityRaw, _, _ := s.configs.Get(ctx, domain.ConfigPansearchPriority)
	priority := splitCSV(priorityRaw)
	// CAM 过滤：默认 true（除非 ConfigPansearchCAMBlock == "false"）
	camBlock, _, _ := s.configs.Get(ctx, domain.ConfigPansearchCAMBlock)
	camEnabled := strings.TrimSpace(strings.ToLower(camBlock)) != "false"
	return SortResults(results, priority, camEnabled)
}

// splitCSV 切分逗号分隔字符串（去除空白 + 跳过空段）。
func splitCSV(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Invalidate 清空指定关键词的缓存（[v7 整改] §8.5 缓存失效钩子）。
// 触发时机：账号新增/转存成功/索引刷新。
func (s *Service) Invalidate(ctx context.Context, keyword string) error {
	if s.cacheRepo == nil || strings.TrimSpace(keyword) == "" {
		return nil
	}
	return s.cacheRepo.Delete(ctx, keyword)
}

func mapDiskType(t string) string {
	switch t {
	case "115":
		return "pan115"
	case "123":
		return "pan123"
	case "baidu":
		return "baidu"
	case "guangya":
		return "guangya"
	default:
		return t
	}
}

func detectQuality(note string) string {
	up := strings.ToUpper(note)
	switch {
	case strings.Contains(up, "4K"):
		return "4K"
	case strings.Contains(up, "1080"):
		return "1080P"
	case strings.Contains(up, "720"):
		return "720P"
	case strings.Contains(up, "CAM"), strings.Contains(note, "枪版"):
		return "CAM"
	default:
		return ""
	}
}

func detectFormat(note string) string {
	up := strings.ToLower(note)
	for _, f := range []string{"mkv", "mp4", "ts", "avi", "rmvb", "iso"} {
		if strings.Contains(up, f) {
			return f
		}
	}
	return ""
}