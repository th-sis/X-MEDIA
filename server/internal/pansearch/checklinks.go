package pansearch

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"xmedia/internal/domain"
)

// CheckItem 链接检测输入（§8.2 链接检测 API）。
type CheckItem struct {
	DiskType string `json:"disk_type"`
	URL      string `json:"url"`
	Password string `json:"password"`
}

// CheckResult 链接检测结果（§8.2）。
type CheckResult struct {
	DiskType string `json:"disk_type"`
	URL      string `json:"url"`
	State    string `json:"state"`   // ok / bad
	Summary  string `json:"summary"` // 链接有效 / 链接失效
}

// CheckLinks 调用 PanSou /api/check/links 批量检测分享链接有效性（§8.4）。
// PanSou 不可达时返回错误（调用方降级）。
func (s *Service) CheckLinks(ctx context.Context, items []CheckItem) ([]CheckResult, error) {
	if len(items) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(map[string]any{"items": items})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL(ctx)+"/api/check/links", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errUpstreamStatus(resp.StatusCode)
	}
	var out struct {
		Results []CheckResult `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Results, nil
}

func errUpstreamStatus(code int) error {
	return &upstreamStatusError{code: code}
}

type upstreamStatusError struct{ code int }

func (e *upstreamStatusError) Error() string { return "pansou http " + itoa(e.code) }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// SortResults 质量排序（§8.5）：
// 1. 排除 CAM（除非 camBlock=false）
// 2. 4K > 1080P > 720P > 未知
// 3. 同质量按 datetime 降序（越新越优先）
// 4. 按网盘优先级排序（priority 中先出现的网盘排前）
func SortResults(results []domain.PanSearchResult, priority []string, camBlock bool) []domain.PanSearchResult {
	prioRank := map[string]int{}
	for i, p := range priority {
		prioRank[p] = i
	}
	filtered := make([]domain.PanSearchResult, 0, len(results))
	for _, r := range results {
		if camBlock && r.Quality == "CAM" {
			continue
		}
		filtered = append(filtered, r)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		a, b := filtered[i], filtered[j]
		ar, aok := prioRank[a.Source]
		br, bok := prioRank[b.Source]
		if aok != bok {
			return aok
		}
		if aok && ar != br {
			return ar < br
		}
		aq, bq := qualityRank(a.Quality), qualityRank(b.Quality)
		if aq != bq {
			return aq < bq
		}
		return parseTime(a.Datetime).After(parseTime(b.Datetime))
	})
	return filtered
}

// qualityRank 返回质量排序权重（越小越优先）。
func qualityRank(q string) int {
	switch strings.ToUpper(strings.TrimSpace(q)) {
	case "4K":
		return 0
	case "1080P":
		return 1
	case "720P":
		return 2
	default:
		return 3
	}
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
