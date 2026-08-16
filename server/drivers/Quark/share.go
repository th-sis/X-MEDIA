package quark

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"xmedia/internal/domain"
	"xmedia/internal/driver"
)

// 分享转存协议（2026-08-02 双实现三角测量验证）：
//   POST /share/sharepage/token   {pwd_id, passcode}            -> data.stoken
//   GET  /share/sharepage/detail  {pwd_id, stoken, pdir_fid=0...} -> data.list[]
//   POST /share/sharepage/save    {fid_list, fid_token_list, to_pdir_fid, ...} -> data.task_id
//   GET  /task                    {task_id}                     -> data.status==2 完成

type shareTokenResp struct {
	Stoken string `json:"stoken"`
	Status int    `json:"status"`
	PwdID  string `json:"pwd_id"`
}

type shareFileItem struct {
	FID      string `json:"fid"`
	FileName string `json:"file_name"`
	Size     int64  `json:"size"`
	FidToken string `json:"fid_token"`
	Dir      bool   `json:"dir"`
}

type shareDetailResp struct {
	List []shareFileItem `json:"list"`
}

type shareSaveResp struct {
	TaskID string `json:"task_id"`
	Status int    `json:"status"`
}

type shareTaskResp struct {
	Status int `json:"status"`
}

// parseShareLink 从夸克分享 URL 提取 pwd_id 与 passcode。
// 支持 https://pan.quark.cn/s/{id} 与 ?pwd= 参数两种形态。
func parseShareLink(raw string) (pwdID, passcode string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", domain.Errorf(domain.CodeValidation, "分享链接不能为空")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", domain.Errorf(domain.CodeValidation, "分享链接格式错误")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[len(parts)-1]) == "" {
		return "", "", domain.Errorf(domain.CodeValidation, "分享链接缺少 pwd_id")
	}
	pwdID = strings.TrimSpace(parts[len(parts)-1])
	passcode = u.Query().Get("pwd")
	return pwdID, passcode, nil
}

// SaveShare 实现 driver.ShareSaver（§6.8.5）：转存夸克分享到指定目录。
func (d *Driver) SaveShare(ctx context.Context, req driver.ShareRequest) (*driver.ShareResult, error) {
	pwdID, passcode, err := parseShareLink(req.ShareURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Password) != "" {
		passcode = strings.TrimSpace(req.Password)
	}
	target := d.normalizeParent(req.TargetParentID)

	stoken, err := d.shareToken(ctx, pwdID, passcode)
	if err != nil {
		return nil, err
	}
	item, err := d.shareFirstFile(ctx, pwdID, stoken)
	if err != nil {
		return nil, err
	}
	taskID, err := d.shareSave(ctx, pwdID, stoken, item, target)
	if err != nil {
		return nil, err
	}
	if err := d.waitShareTask(ctx, taskID); err != nil {
		return nil, err
	}
	// 转存完成后文件拥有新的 fid：列目标目录按文件名匹配
	fid, err := d.findSavedFileID(ctx, target, item.FileName)
	if err != nil {
		return nil, err
	}
	return &driver.ShareResult{
		FileID:    fid,
		FileName:  item.FileName,
		FileSize:  item.Size,
		FileCount: 1,
	}, nil
}

func (d *Driver) shareToken(ctx context.Context, pwdID, passcode string) (string, error) {
	var out shareTokenResp
	if _, err := d.apiRequest(ctx, "POST", "/share/sharepage/token", nil, map[string]any{
		"pwd_id":   pwdID,
		"passcode": passcode,
	}, &out); err != nil {
		return "", err
	}
	if strings.TrimSpace(out.Stoken) == "" {
		return "", domain.Errorf(domain.CodeDriverError, "夸克分享 token 为空（分享可能已失效或密码错误）")
	}
	return out.Stoken, nil
}

func (d *Driver) shareFirstFile(ctx context.Context, pwdID, stoken string) (*shareFileItem, error) {
	query := url.Values{}
	query.Set("pwd_id", pwdID)
	query.Set("stoken", stoken)
	query.Set("pdir_fid", "0")
	query.Set("force", "0")
	query.Set("_page", "1")
	query.Set("_size", "50")
	query.Set("_sort", "file_name:asc")
	query.Set("_fetch_total", "1")
	var out shareDetailResp
	if _, err := d.apiRequest(ctx, "GET", "/share/sharepage/detail", query, nil, &out); err != nil {
		return nil, err
	}
	for _, item := range out.List {
		if !item.Dir && strings.TrimSpace(item.FID) != "" {
			return &item, nil
		}
	}
	return nil, domain.Errorf(domain.CodeDriverError, "夸克分享内没有可转存的文件")
}

func (d *Driver) shareSave(ctx context.Context, pwdID, stoken string, item *shareFileItem, target string) (string, error) {
	var out shareSaveResp
	if _, err := d.apiRequest(ctx, "POST", "/share/sharepage/save", nil, map[string]any{
		"fid_list":       []string{item.FID},
		"fid_token_list": []string{item.FidToken},
		"to_pdir_fid":    target,
		"pwd_id":         pwdID,
		"stoken":         stoken,
		"pdir_fid":       "0",
		"scene":          "link",
	}, &out); err != nil {
		return "", err
	}
	if strings.TrimSpace(out.TaskID) == "" {
		return "", domain.Errorf(domain.CodeDriverError, "夸克分享转存未返回任务 ID")
	}
	return out.TaskID, nil
}

// waitShareTask 轮询转存任务直到完成（800ms × 最多 15 次，与上游实现一致）。
func (d *Driver) waitShareTask(ctx context.Context, taskID string) error {
	for attempt := 0; attempt < 15; attempt++ {
		query := url.Values{}
		query.Set("task_id", taskID)
		query.Set("retry_index", strconv.Itoa(attempt))
		var out shareTaskResp
		if _, err := d.apiRequest(ctx, "GET", "/task", query, nil, &out); err != nil {
			return err
		}
		if out.Status == 2 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(800 * time.Millisecond):
		}
	}
	return domain.Errorf(domain.CodeDriverError, "夸克分享转存任务超时未完成")
}

// findSavedFileID 转存完成后在目标目录按文件名匹配新文件 ID。
func (d *Driver) findSavedFileID(ctx context.Context, parentID, fileName string) (string, error) {
	items, err := d.ListFiles(ctx, parentID)
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if !item.IsDir && strings.EqualFold(strings.TrimSpace(item.Name), strings.TrimSpace(fileName)) {
			return item.ID, nil
		}
	}
	return "", domain.Errorf(domain.CodeDriverError, "夸克转存完成但未在目标目录找到文件（文件名：%s）", fileName)
}
