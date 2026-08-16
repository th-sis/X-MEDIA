package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"xmedia/internal/domain"
)

// Bangumi（bgm.tv）客户端（§7.3）：动漫条目搜索与元数据，作为 TMDB 的补充源。
// 默认公共 API：https://api.bgm.tv（无鉴权的公开接口，限流约 1 rps）。

const defaultBangumiBase = "https://api.bgm.tv"

// bangumiBase 读取配置的 Bangumi API 地址（缺省 public API）。
func (s *Service) bangumiBase(ctx context.Context) string {
	if s.configs == nil {
		return defaultBangumiBase
	}
	v, ok, err := s.configs.Get(ctx, domain.ConfigBangumiAPIBase)
	if err != nil || !ok || strings.TrimSpace(v) == "" {
		return defaultBangumiBase
	}
	return strings.TrimSpace(v)
}

// SearchBangumi 搜索动漫条目（/search/subject/{keywords}?type=2&responseGroup=medium）。
// type=2 限定动画。返回统一 MediaSummary（ExternalSource=bangumi）。
func (s *Service) SearchBangumi(ctx context.Context, keywords string) ([]MediaSummary, error) {
	keywords = strings.TrimSpace(keywords)
	if keywords == "" {
		return nil, domain.Errorf(domain.CodeValidation, "搜索关键词不能为空")
	}
	base := s.bangumiBase(ctx)
	path := base + "/search/subject/" + url.PathEscape(keywords) + "?type=2&responseGroup=medium"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "xmedia/7.0 (https://github.com/nousresearch)")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bangumi status %d", resp.StatusCode)
	}
	var body struct {
		List []struct {
			ID      int64  `json:"id"`
			Name    string `json:"name"`
			NameCN  string `json:"name_cn"`
			Summary string `json:"summary"`
			Rating  struct {
				Score float64 `json:"score"`
			} `json:"rating"`
			Images struct {
				Large string `json:"large"`
			} `json:"images"`
			AirDate string `json:"air_date"`
		} `json:"list"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	out := make([]MediaSummary, 0, len(body.List))
	for _, item := range body.List {
		title := item.Name
		if item.NameCN != "" {
			title = item.NameCN
		}
		year := 0
		if len(item.AirDate) >= 4 {
			fmt.Sscanf(item.AirDate[:4], "%d", &year)
		}
		out = append(out, MediaSummary{
			ExternalID:     item.ID,
			ExternalSource: "bangumi",
			MediaType:      "tv",
			Title:          title,
			TitleOrig:      item.Name,
			Year:           year,
			VoteAvg:        item.Rating.Score,
			PosterURL:      item.Images.Large,
			Overview:       item.Summary,
		})
	}
	return out, nil
}

// BangumiDetail 获取动漫详情（含季集信息，bgm 以"话数"表达）。
func (s *Service) BangumiDetail(ctx context.Context, externalID int64) (*MediaDetail, error) {
	base := s.bangumiBase(ctx)
	path := base + fmt.Sprintf("/subject/%d?responseGroup=medium", externalID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "xmedia/7.0 (https://github.com/nousresearch)")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bangumi status %d", resp.StatusCode)
	}
	var body struct {
		ID      int64  `json:"id"`
		Name    string `json:"name"`
		NameCN  string `json:"name_cn"`
		Summary string `json:"summary"`
		Eps     int    `json:"eps"`
		Rating  struct {
			Score float64 `json:"score"`
		} `json:"rating"`
		Images struct {
			Large string `json:"large"`
		} `json:"images"`
		AirDate string `json:"air_date"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	if body.ID == 0 {
		return nil, domain.Errf(domain.CodeNotFound)
	}
	title := body.Name
	if body.NameCN != "" {
		title = body.NameCN
	}
	year := 0
	if len(body.AirDate) >= 4 {
		fmt.Sscanf(body.AirDate[:4], "%d", &year)
	}
	// bgm 无季概念：话数即集数，映射为单季
	seasons := []SeasonInfo{}
	if body.Eps > 0 {
		seasons = append(seasons, SeasonInfo{
			SeasonNumber: 1,
			Name:         "全 " + fmt.Sprint(body.Eps) + " 话",
			EpisodeCount: body.Eps,
		})
	}
	return &MediaDetail{
		MediaSummary: MediaSummary{
			ExternalID:     body.ID,
			ExternalSource: "bangumi",
			MediaType:      "tv",
			Title:          title,
			TitleOrig:      body.Name,
			Year:           year,
			VoteAvg:        body.Rating.Score,
			PosterURL:      body.Images.Large,
			Overview:       body.Summary,
		},
		Episodes:    body.Eps,
		Seasons:     len(seasons),
		SeasonsList: seasons,
	}, nil
}
