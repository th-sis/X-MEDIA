package resolve

import (
	"context"
	"fmt"
	"strings"

	"xmedia/internal/domain"
	"xmedia/internal/driver"
	"xmedia/internal/pansearch"
)

// activeAccounts 返回认证状态为 active 的账号列表（P1 转存候选）。
func (s *Service) activeAccounts(ctx context.Context) []domain.Account {
	if s.accountsFn == nil {
		return nil
	}
	return s.accountsFn(ctx)
}

// sourceAccount 找到指定 source_type 下第一个可用的账号。
func sourceAccount(accounts []domain.Account, source string) (domain.Account, bool) {
	for _, acc := range accounts {
		if driverSourceOf(acc.DriverType) == source && acc.IsActive {
			return acc, true
		}
	}
	return domain.Account{}, false
}

// driverSourceOf 将 LitePan 驱动名映射为 X-MEDIA source_type（与 app 层映射保持一致）。
func driverSourceOf(driverType string) string {
	switch strings.ToLower(driverType) {
	case "115_open", "115":
		return "pan115"
	case "123_open", "123":
		return "pan123"
	case "baidu_open", "baidu":
		return "baidu"
	case "quark":
		return "quark"
	case "guangya":
		return "guangya"
	case "localfs", "local":
		return "nas"
	default:
		return strings.ToLower(driverType)
	}
}

// prioritySources 解析用户配置的网盘优先级，过滤未登录项（§11.1）。
func (s *Service) prioritySources(ctx context.Context) []string {
	var configured []string
	if s.configs != nil {
		if v, ok, err := s.configs.Get(ctx, domain.ConfigResolvePriority); err == nil && ok && strings.TrimSpace(v) != "" {
			configured = parsePriorityList(v)
		}
	}
	if len(configured) == 0 {
		configured = parsePriorityList(domain.ConfigDefaults[domain.ConfigResolvePriority])
	}
	logged := map[string]bool{}
	for _, acc := range s.activeAccounts(ctx) {
		logged[driverSourceOf(acc.DriverType)] = true
	}
	out := make([]string, 0, len(configured))
	for _, src := range configured {
		if src == "nas" {
			continue // NAS 走 P0，不参与盘搜
		}
		if logged[src] {
			out = append(out, src)
		}
	}
	return out
}

func parsePriorityList(raw string) []string {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, "[]")
	raw = strings.ReplaceAll(raw, `"`, "")
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// pansearchCloudTypes 把 source_type 优先级映射为 PanSou cloud_types（§8.3）。
func pansearchCloudTypes(sources []string) []string {
	out := make([]string, 0, len(sources))
	for _, src := range sources {
		switch src {
		case "pan115":
			out = append(out, "115")
		case "pan123":
			out = append(out, "123")
		default:
			out = append(out, src)
		}
	}
	return out
}

// runP1 执行 P1 盘搜 + 分享转存（§6.4）。成功返回 true 并直接 complete。
func (s *Service) runP1(ctx context.Context, t *domain.ResolveTask) bool {
	priority := s.prioritySources(ctx)
	if len(priority) == 0 {
		return false
	}
	accounts := s.activeAccounts(ctx)
	if len(accounts) == 0 {
		return false
	}
	var media *domain.MediaLibrary
	if s.mediaLibrary != nil {
		if m, err := s.mediaLibrary.Get(ctx, t.ExternalID, t.ExternalSource); err == nil {
			media = m
		}
	}
	camBlock := true
	if s.configs != nil {
		if v, ok, err := s.configs.Get(ctx, domain.ConfigPansearchCAMBlock); err == nil && ok {
			camBlock = v != "false"
		}
	}
	keywords := buildSearchKeywords(t, media)
	for ki, kw := range keywords {
		results, err := s.pansearchSearch(ctx, pansearch.SearchRequest{
			Keyword:    kw,
			CloudTypes: pansearchCloudTypes(priority),
		})
		if err != nil || len(results) == 0 {
			continue // 下一关键词回退
		}
		if ki == 0 {
			s.push(t, domain.StagePanSearching, "搜索全网盘资源...", 30)
		} else {
			s.push(t, domain.StagePanSearching, fmt.Sprintf("关键词回退：%s ...", kw), 35)
		}
		results = pansearch.SortResults(results, priority, camBlock)
		s.push(t, domain.StagePanSearched, fmt.Sprintf("找到 %d 个资源，分析中...", len(results)), 50)

		// CheckLinks 批量检测（取前 20 条）
		valid := s.checkResults(ctx, results)
		saved := false
		for _, r := range results {
			if !valid[r.ShareURL] {
				continue
			}
			acc, ok := sourceAccount(accounts, r.Source)
			if !ok {
				continue
			}
			drv, err := s.driverGet(ctx, acc.ID)
			if err != nil {
				continue
			}
			saver, ok := drv.(driver.ShareSaver)
			if !ok {
				continue // 驱动未实现转存，尝试下一个
			}
			s.push(t, domain.StageTransferring, fmt.Sprintf("正在转存到 %s ...", r.Source), 70)
			parent, err := s.ensureSaveRoot(ctx, drv, acc.ID, r.Source)
			if err != nil {
				continue
			}
			savedRes, err := saver.SaveShare(ctx, driver.ShareRequest{
				ShareURL:       r.ShareURL,
				Password:       r.Password,
				TargetParentID: parent,
			})
			if err != nil || savedRes == nil {
				continue // 转存失败：下一个结果
			}
			// [V7 §6.9.2] 转存成功后, 把文件名改写为统一模板
			// (失败时静默, 不影响 P1 命中 — 见 post_rename.go).
			s.applyPostSaveRename(ctx, drv, t, savedRes)
			s.push(t, domain.StageResolvingLink, "获取播放链接...", 88)
			s.indexSavedFile(ctx, t, acc, r.Source, savedRes)
			ticket, err := s.signer.Sign(ctx, ticketClaimsFor(t, savedRes.FileID, r.Source, acc.ID), 0)
			if err == nil {
				t.ResultAccountID = acc.ID
				t.ResultFilePath = savedRes.FileName
				s.complete(t, r.Source, savedRes.FileID, ticket, savedRes.FileName)
				saved = true
				break
			}
		}
		if saved {
			return true
		}
	}
	return false
}

// checkResults 批量检测链接有效性（§8.4），返回有效 URL 集合。
func (s *Service) checkResults(ctx context.Context, results []domain.PanSearchResult) map[string]bool {
	valid := map[string]bool{}
	if s.pansearchCheck == nil {
		// 未注入检测能力：全部视为有效（驱动转存失败时自然跳过）
		for _, r := range results {
			valid[r.ShareURL] = true
		}
		return valid
	}
	const batch = 20
	for start := 0; start < len(results); start += batch {
		end := start + batch
		if end > len(results) {
			end = len(results)
		}
		items := make([]pansearch.CheckItem, 0, end-start)
		for _, r := range results[start:end] {
			if r.ShareURL == "" {
				continue
			}
			items = append(items, pansearch.CheckItem{DiskType: r.Source, URL: r.ShareURL, Password: r.Password})
		}
		if len(items) == 0 {
			continue
		}
		out, err := s.pansearchCheck(ctx, items)
		if err != nil {
			for _, r := range results[start:end] {
				valid[r.ShareURL] = true // 检测服务不可达：降级为全部有效
			}
			continue
		}
		for _, c := range out {
			if c.State == "ok" {
				valid[c.URL] = true
			}
		}
	}
	return valid
}

// indexSavedFile 转存成功后写入 media_index（§9.3 事件驱动在无 eventbus 时的直写路径）。
func (s *Service) indexSavedFile(ctx context.Context, t *domain.ResolveTask, acc domain.Account, source string, saved *driver.ShareResult) {
	if s.mediaIndex == nil || saved == nil {
		return
	}
	format := ""
	if idx := strings.LastIndexByte(saved.FileName, '.'); idx >= 0 {
		format = saved.FileName[idx+1:]
	}
	_, _ = s.mediaIndex.Upsert(ctx, &domain.MediaIndex{
		ExternalID:     t.ExternalID,
		ExternalSource: t.ExternalSource,
		Season:         t.Season,
		Episode:        t.Episode,
		MediaType:      t.MediaType,
		Title:          t.Title,
		Year:           t.Year,
		SourceType:     source,
		AccountID:      acc.ID,
		FilePath:       saved.FileName,
		FileID:         saved.FileID,
		FileSize:       saved.FileSize,
		FileFormat:     format,
		MatchStatus:    domain.MatchMatched,
		MatchScore:     1.0,
	})
}

// ensureSaveRoot 返回转存根目录 ID（§6.9.1）：配置存在直接用；
// 否则在网盘根目录查找或创建 X-MEDIA/ 目录并持久化。
func (s *Service) ensureSaveRoot(ctx context.Context, drv driver.Driver, accountID int64, source string) (string, error) {
	key := fmt.Sprintf("pan_%s_save_root_%d", strings.TrimPrefix(source, "pan"), accountID)
	if s.configs != nil {
		if v, ok, err := s.configs.Get(ctx, key); err == nil && ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v), nil
		}
	}
	root := drv.Config().DefaultRoot
	if root == "" {
		root = "0"
	}
	files, err := drv.ListFiles(ctx, root)
	if err != nil {
		return "", err
	}
	for _, f := range files {
		if f.IsDir && f.Name == "X-MEDIA" {
			if s.configs != nil {
				_ = s.configs.Set(ctx, key, f.ID)
			}
			return f.ID, nil
		}
	}
	creator, ok := drv.(driver.FolderCreator)
	if !ok {
		return "", fmt.Errorf("驱动不支持创建目录")
	}
	folder, err := creator.CreateFolder(ctx, root, "X-MEDIA")
	if err != nil {
		return "", err
	}
	if s.configs != nil {
		_ = s.configs.Set(ctx, key, folder.ID)
	}
	return folder.ID, nil
}
