package app

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"xmedia/internal/domain"
)

// runStartupValidation 启动时执行关键配置验证（[v7 整改] §11.1）。
// 失败不阻塞启动：结果写入 configs.validation_last_result，供 /api/health 读取。
func runStartupValidation(ctx context.Context, st *storeBundle) {
	panURL, _, _ := st.store.Configs.Get(ctx, domain.ConfigPansearchURL)
	accountCount := 0
	if list, err := st.store.Accounts.List(ctx); err == nil {
		accountCount = len(list)
	}
	_ = validateCriticalConfigs(ctx, st.store.Configs, accountCount, panURL)
}

// validateCriticalConfigs 启动时执行关键配置验证（[v7 整改] §11.1）。
// 失败不阻塞启动：写入 configs 表 (validation_last_result) + slog.Warn。
func validateCriticalConfigs(ctx context.Context, configs domain.ConfigRepository, accountsCount int, pansearchURL string) domain.ValidationResult {
	res := domain.ValidationResult{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Warnings:  map[string]string{},
	}

	// 1. TMDB Key 格式（32 字符 hex 或 v3 格式）
	tmdbKey, _, _ := configs.Get(ctx, domain.ConfigTMDBAPIKey)
	tmdbKey = strings.TrimSpace(tmdbKey)
	if tmdbKey == "" {
		res.TMDBKey = domain.ValidationCheck{Status: "warning", Message: "未配置 TMDB API Key（演示模式可用，但搜索/榜单降级）"}
		res.Warnings["tmdb_key"] = "未配置"
	} else {
		// v3 auth = 32 位 hex；v4 = 64 字符 base64
		if matched, _ := regexp.MatchString(`^[0-9a-fA-F]{32}$`, tmdbKey); matched {
			res.TMDBKey = domain.ValidationCheck{Status: "ok", Message: "TMDB v3 key 格式正确"}
		} else if matched, _ := regexp.MatchString(`^[A-Za-z0-9_-]{40,80}$`, tmdbKey); matched {
			res.TMDBKey = domain.ValidationCheck{Status: "warning", Message: "TMDB Key 格式非 v3（可能为 v4），建议使用 v3 auth key"}
		} else {
			res.TMDBKey = domain.ValidationCheck{Status: "error", Message: "TMDB Key 格式不正确（应为 32 位 hex）"}
			res.Issues = append(res.Issues, "tmdb_key_invalid")
		}
	}

	// 2. PanSou URL 可达性
	panURL := strings.TrimSpace(pansearchURL)
	if panURL == "" {
		panURL = domain.ConfigDefaults[domain.ConfigPansearchURL]
	}
	if !strings.HasPrefix(panURL, "http://") && !strings.HasPrefix(panURL, "https://") {
		res.PanSouURL = domain.ValidationCheck{Status: "error", Message: "PanSou URL 格式错误"}
		res.Issues = append(res.Issues, "pansearch_url_invalid")
	} else if reachable, _ := checkHTTPReachable(ctx, panURL+"/api/health"); reachable {
		res.PanSouURL = domain.ValidationCheck{Status: "ok", Message: "PanSou 可达"}
	} else {
		res.PanSouURL = domain.ValidationCheck{Status: "warning", Message: "PanSou 暂不可达（盘搜功能将降级）"}
		res.Warnings["pansearch_url"] = "unreachable"
	}

	// 3. 至少 1 个网盘账号
	if accountsCount > 0 {
		res.HasAnyAccount = domain.ValidationCheck{Status: "ok", Message: fmt.Sprintf("已配置 %d 个网盘账号", accountsCount)}
	} else {
		res.HasAnyAccount = domain.ValidationCheck{Status: "warning", Message: "未配置任何网盘账号（仅演示模式可播）"}
		res.Warnings["accounts"] = "none"
	}

	// 4. P2 磁力开关
	magnetEnabled, _, _ := configs.Get(ctx, domain.ConfigResolveMagnetEnabled)
	if strings.TrimSpace(magnetEnabled) == "" || magnetEnabled == "true" {
		res.MagnetEnabled = domain.ValidationCheck{Status: "ok", Message: "P2 磁力兜底已启用"}
	} else {
		res.MagnetEnabled = domain.ValidationCheck{Status: "warning", Message: "P2 磁力兜底已关闭"}
	}

	res.OK = len(res.Issues) == 0

	// 持久化供 /api/health 读取
	if data, err := json.Marshal(res); err == nil {
		_ = configs.Set(ctx, "validation_last_result", string(data))
	}
	return res
}

// checkHTTPReachable 简易 HTTP 探测（仅用于启动验证，不做严格健康判定）。
func checkHTTPReachable(ctx context.Context, url string) (bool, error) {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	client := &http.Client{Timeout: 2 * time.Second, Transport: tr}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500, nil
}
