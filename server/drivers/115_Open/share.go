package pan115open

import (
	"context"
	"net/url"
	"strings"

	"xmedia/internal/domain"
	"xmedia/internal/driver"
)

// 115 分享转存协议说明：
// 分享接收（share/snap + share/receive）托管在 webapi.115.com，属于登录态
// （cookie/UID 会话）API；115_Open 驱动使用 proapi.115.com 的 OAuth 授权体系，
// 该体系不含分享接收端点（文档 D23 选择 OAuth 时的协议断层）。
//
// 因此 SaveShare 在 OAuth 凭据下诚实返回能力错误（真实失败而非假成功），
// 协议函数保留供未来凭据扩展（如增加 cookie 会话注入）后启用。

const (
	shareSnapPath    = "/share/snap"
	shareReceivePath = "/share/receive"
)

// parse115ShareLink 从 115 分享 URL 提取 share_code 与 receive_code。
// 支持 https://115.com/s/{code}?password={pwd} 与 https://115cdn.com/s/{code} 形态。
func parse115ShareLink(raw string) (shareCode, receiveCode string, err error) {
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
		return "", "", domain.Errorf(domain.CodeValidation, "分享链接缺少 share_code")
	}
	shareCode = strings.TrimSpace(parts[len(parts)-1])
	receiveCode = u.Query().Get("password")
	return shareCode, receiveCode, nil
}

// SaveShare 实现 driver.ShareSaver。
// 当前 OAuth 凭据无分享接收能力，返回明确的能力错误；调用方（resolve P1）
// 会按优先级跳过该结果继续尝试，不会产生假成功。
func (d *Driver) SaveShare(ctx context.Context, req driver.ShareRequest) (*driver.ShareResult, error) {
	if _, _, err := parse115ShareLink(req.ShareURL); err != nil {
		return nil, err
	}
	return nil, domain.Errorf(domain.CodePermissionDenied,
		"115 分享转存需要登录态（webapi.115.com）会话，当前 115 Open（OAuth）凭据不支持；请使用支持分享转存的网盘账号或等待 115 会话凭据支持")
}

// ---- 协议函数（未来 cookie 会话凭据接入后启用） ----

// snap115Share 获取分享信息与文件列表（webapi.115.com 会话 API）。
func (d *Driver) snap115Share(ctx context.Context, shareCode, receiveCode string) (*driver.ShareResult, error) {
	_ = ctx
	_ = shareSnapPath
	_ = shareCode
	_ = receiveCode
	// 需要 cookie 会话的 UID 鉴权，OAuth 凭据下不可达。
	return nil, domain.Errorf(domain.CodePermissionDenied, "115 分享查询需要 cookie 会话")
}

// receive115Share 执行分享转存（webapi.115.com 会话 API）。
func (d *Driver) receive115Share(ctx context.Context, shareCode, receiveCode, fileID, cid string) error {
	_ = ctx
	_ = shareReceivePath
	_ = shareCode
	_ = receiveCode
	_ = fileID
	_ = cid
	return domain.Errorf(domain.CodePermissionDenied, "115 分享转存需要 cookie 会话")
}
