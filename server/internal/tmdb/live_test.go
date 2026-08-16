package tmdb

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"xmedia/internal/domain"
)

// stubConfigRepo 测试用配置仓库。
type stubConfigRepo struct {
	values map[string]string
}

func (s *stubConfigRepo) Get(_ context.Context, key string) (string, bool, error) {
	v, ok := s.values[key]
	return v, ok, nil
}
func (s *stubConfigRepo) Set(_ context.Context, key, value string) error {
	s.values[key] = value
	return nil
}
func (s *stubConfigRepo) All(context.Context) (map[string]string, error) { return s.values, nil }

// stubLibraryRepo 测试用媒体库仓库。
type stubLibraryRepo struct{ upserted []*domain.MediaLibrary }

func (s *stubLibraryRepo) Upsert(_ context.Context, m *domain.MediaLibrary) (int64, error) {
	s.upserted = append(s.upserted, m)
	return 1, nil
}
func (s *stubLibraryRepo) Get(context.Context, int64, string) (*domain.MediaLibrary, error) {
	return nil, domain.Errf(domain.CodeNotFound)
}
func (s *stubLibraryRepo) Touch(context.Context, int64, string) error { return nil }
func (s *stubLibraryRepo) SearchByTitle(context.Context, string, int) ([]*domain.MediaLibrary, error) {
	return nil, nil
}
func (s *stubLibraryRepo) ListForEviction(context.Context, int) ([]*domain.MediaLibrary, error) {
	return nil, nil
}
func (s *stubLibraryRepo) CountTotal(context.Context) (int, error) { return 0, nil }
func (s *stubLibraryRepo) Delete(context.Context, int64) error     { return nil }

// newMockTMDB 构造带 mock TMDB 服务器的 Service（api key 已配置）。
func newMockTMDB(t *testing.T, handler http.HandlerFunc) (*Service, *httptest.Server, *stubLibraryRepo) {
	t.Helper()
	server := httptest.NewServer(handler)
	cfg := &stubConfigRepo{values: map[string]string{"tmdb_api_key": "test-key-123"}}
	lib := &stubLibraryRepo{}
	s := NewService(cfg, lib)
	s.base = server.URL
	s.client = server.Client()
	return s, server, lib
}

// TestSearchLiveRequestShape 验证 live 搜索端点、参数与结果映射。
func TestSearchLiveRequestShape(t *testing.T) {
	s, server, _ := newMockTMDB(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/multi" {
			t.Errorf("路径 = %s, want /search/multi", r.URL.Path)
		}
		if r.URL.Query().Get("query") != "阿凡达" {
			t.Errorf("query 参数错误: %s", r.URL.RawQuery)
		}
		if r.URL.Query().Get("api_key") != "test-key-123" {
			t.Errorf("api_key 未携带: %s", r.URL.RawQuery)
		}
		if r.URL.Query().Get("language") != "zh-CN" {
			t.Errorf("language 参数错误: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"id": 19995, "title": "阿凡达", "original_title": "Avatar",
					"release_date": "2009-12-18", "vote_average": 7.5,
					"poster_path": "/avatar.jpg", "media_type": "movie"},
			},
			"total_pages": 3, "total_results": 55,
		})
	})
	defer server.Close()

	resp, err := s.Search(context.Background(), "阿凡达", 1)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].ExternalID != 19995 || resp.Items[0].MediaType != "movie" {
		t.Fatalf("结果映射错误: %#v", resp.Items)
	}
	if resp.Items[0].Year != 2009 || !strings.Contains(resp.Items[0].PosterURL, "/avatar.jpg") {
		t.Fatalf("年份/海报映射错误: %#v", resp.Items[0])
	}
	if !resp.HasMore || resp.Total != 55 {
		t.Fatalf("分页信息错误: hasMore=%v total=%d", resp.HasMore, resp.Total)
	}
}

// TestDiscoverLiveGenreMapping 验证中文分类名到 genre ID 的映射。
func TestDiscoverLiveGenreMapping(t *testing.T) {
	var gotPath, gotGenres string
	s, server, _ := newMockTMDB(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotGenres = r.URL.Query().Get("with_genres")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}, "total_pages": 1, "total_results": 0})
	})
	defer server.Close()

	if _, err := s.Discover(context.Background(), "movie", "科幻", 1); err != nil {
		t.Fatalf("Discover 失败: %v", err)
	}
	if gotPath != "/discover/movie" {
		t.Errorf("路径 = %s, want /discover/movie", gotPath)
	}
	if gotGenres != "878" {
		t.Errorf("科幻 genre ID = %q, want 878", gotGenres)
	}
}

// TestDiscoverLiveKeywordFallback 未知分类走 with_keywords。
func TestDiscoverLiveKeywordFallback(t *testing.T) {
	var gotKw string
	s, server, _ := newMockTMDB(t, func(w http.ResponseWriter, r *http.Request) {
		gotKw = r.URL.Query().Get("with_keywords")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}, "total_pages": 1, "total_results": 0})
	})
	defer server.Close()

	if _, err := s.Discover(context.Background(), "movie", "西部片", 1); err != nil {
		t.Fatalf("Discover 失败: %v", err)
	}
	if gotKw != "西部片" {
		t.Errorf("with_keywords = %q, want 西部片", gotKw)
	}
}

// TestDetailLiveCachesToLibrary 详情走 live 且写入 media_library 缓存。
func TestDetailLiveCachesToLibrary(t *testing.T) {
	s, server, lib := newMockTMDB(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/movie/19995" {
			t.Errorf("路径 = %s, want /movie/19995", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": 19995, "title": "阿凡达", "original_title": "Avatar",
			"release_date": "2009-12-18", "vote_average": 7.5, "vote_count": 28000,
			"poster_path": "/avatar.jpg", "backdrop_path": "/avatar-bd.jpg",
			"overview": "星球潘多拉", "runtime": 162,
			"genres": []map[string]any{{"name": "科幻"}, {"name": "冒险"}},
		})
	})
	defer server.Close()

	det, err := s.Detail(context.Background(), 19995, "tmdb")
	if err != nil {
		t.Fatalf("详情失败: %v", err)
	}
	if det.Title != "阿凡达" || det.Runtime != 162 || det.Year != 2009 {
		t.Fatalf("详情映射错误: %#v", det)
	}
	if len(det.Genres) != 2 || det.Genres[0] != "科幻" {
		t.Fatalf("类型映射错误: %#v", det.Genres)
	}
	if len(lib.upserted) != 1 || lib.upserted[0].ExternalID != 19995 {
		t.Fatalf("media_library 缓存应写入: %#v", lib.upserted)
	}
	if lib.upserted[0].Runtime != 162 || lib.upserted[0].TitleOrig != "Avatar" {
		t.Fatalf("缓存字段错误: %#v", lib.upserted[0])
	}
}

// TestSeasonsLiveForTV TV 详情季列表。
func TestSeasonsLiveForTV(t *testing.T) {
	s, server, _ := newMockTMDB(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tv/1399" {
			t.Errorf("路径 = %s, want /tv/1399", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": 1399, "name": "权力的游戏", "seasons": []map[string]any{
				{"season_number": 1, "name": "第 1 季", "episode_count": 10, "air_date": "2011-04-17"},
				{"season_number": 2, "name": "第 2 季", "episode_count": 10, "air_date": "2012-04-01"},
				{"season_number": 0, "name": "特别篇", "episode_count": 3},
			},
		})
	})
	defer server.Close()

	seasons, err := s.Seasons(context.Background(), 1399, "tmdb")
	if err != nil {
		t.Fatalf("季列表失败: %v", err)
	}
	if len(seasons) != 2 {
		t.Fatalf("季 0 应被过滤，got %d 季: %#v", len(seasons), seasons)
	}
	if seasons[0].SeasonNumber != 1 || seasons[0].EpisodeCount != 10 || seasons[0].AirDate != "2011-04-17" {
		t.Fatalf("季信息映射错误: %#v", seasons[0])
	}
}

// TestDemoFallbackWhenNoKey 无 API Key 时回退演示目录。
func TestDemoFallbackWhenNoKey(t *testing.T) {
	cfg := &stubConfigRepo{values: map[string]string{}}
	s := NewService(cfg, &stubLibraryRepo{})
	resp, err := s.Search(context.Background(), "阿凡达", 1)
	if err != nil {
		t.Fatalf("演示搜索失败: %v", err)
	}
	if len(resp.Items) == 0 {
		t.Fatalf("演示搜索应有结果")
	}
}

// TestBangumiSearchAndDetail mock Bangumi 端点。
func TestBangumiSearchAndDetail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/search/subject/"):
			if !strings.Contains(r.URL.Path, "type=2") && r.URL.RawQuery != "type=2&responseGroup=medium" {
				// PathEscape 后的中文在 Path 中，type 在 query
			}
			if r.URL.Query().Get("type") != "2" {
				t.Errorf("type 参数 = %q, want 2", r.URL.Query().Get("type"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"list": []map[string]any{
				{"id": 265, "name": "進撃の巨人", "name_cn": "进击的巨人",
					"summary": "巨人出现", "rating": map[string]any{"score": 8.9},
					"images": map[string]any{"large": "https://img/bgm.jpg"}, "air_date": "2013-04-06"},
			}})
		case strings.HasPrefix(r.URL.Path, "/subject/"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": 265, "name": "進撃の巨人", "name_cn": "进击的巨人",
				"summary": "巨人出现", "eps": 25,
				"rating": map[string]any{"score": 8.9},
				"images": map[string]any{"large": "https://img/bgm.jpg"}, "air_date": "2013-04-06",
			})
		default:
			t.Errorf("未预期的路径: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := &stubConfigRepo{values: map[string]string{"bangumi_api_base": server.URL}}
	s := NewService(cfg, &stubLibraryRepo{})
	s.client = server.Client()

	items, err := s.SearchBangumi(context.Background(), "进击的巨人")
	if err != nil {
		t.Fatalf("Bangumi 搜索失败: %v", err)
	}
	if len(items) != 1 || items[0].ExternalID != 265 || items[0].Title != "进击的巨人" {
		t.Fatalf("Bangumi 搜索结果错误: %#v", items)
	}
	if items[0].ExternalSource != "bangumi" || items[0].Year != 2013 {
		t.Fatalf("Bangumi 来源/年份错误: %#v", items[0])
	}

	det, err := s.BangumiDetail(context.Background(), 265)
	if err != nil {
		t.Fatalf("Bangumi 详情失败: %v", err)
	}
	if det.Episodes != 25 || len(det.SeasonsList) != 1 || det.SeasonsList[0].EpisodeCount != 25 {
		t.Fatalf("Bangumi 话数映射错误: %#v", det)
	}
}
