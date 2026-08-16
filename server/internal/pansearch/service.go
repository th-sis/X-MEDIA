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
type Service struct {
	configs   domain.ConfigRepository
	cacheRepo domain.PansearchCacheRepository
	client    *http.Client
}

func NewService(configs domain.ConfigRepository, cache domain.PansearchCacheRepository) *Service {
	return &Service{
		configs:   configs,
		cacheRepo: cache,
		client:    &http.Client{Timeout: 8 * time.Second},
	}
}

func (s *Service) baseURL(ctx context.Context) string {
	v, ok, err := s.configs.Get(ctx, domain.ConfigPansearchURL)
	if err != nil || !ok || strings.TrimSpace(v) == "" {
		return domain.ConfigDefaults[domain.ConfigPansearchURL]
	}
	return strings.TrimRight(strings.TrimSpace(v), "/")
}

// Health 检测 PanSou 可达性。
func (s *Service) Health(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL(ctx)+"/api/health", nil)
	if err != nil {
		return false
	}
	resp, err := s.client.Do(req)
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
func (s *Service) Search(ctx context.Context, req SearchRequest) ([]domain.PanSearchResult, error) {
	cloudTypes := strings.Join(req.CloudTypes, ",")
	// 读缓存（1 小时内有效，link_count=0 且 30 分钟前则跳过重搜）
	if cached, cnt, at, err := s.cacheRepo.Get(ctx, req.Keyword, cloudTypes); err == nil {
		if cnt == 0 && at != nil && time.Since(*at) < 30*time.Minute {
			// stale 且未过 30 分钟，跳过缓存
		} else if at != nil && time.Since(*at) < time.Hour {
			var results []domain.PanSearchResult
			if json.Unmarshal([]byte(cached), &results) == nil {
				return results, nil
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
	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, nil // PanSou 不可达，返回空
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
	// 写缓存
	if data, err := json.Marshal(results); err == nil {
		_ = s.cacheRepo.Set(ctx, req.Keyword, cloudTypes, string(data), len(results))
	}
	return results, nil
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

// Invalidate 清空指定关键词的缓存（[v7 整改] §8.5 缓存失效钩子）。
// 触发时机：账号新增/转存成功/索引刷新。
func (s *Service) Invalidate(ctx context.Context, keyword string) error {
	if s.cacheRepo == nil || strings.TrimSpace(keyword) == "" {
		return nil
	}
	return s.cacheRepo.Delete(ctx, keyword)
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
