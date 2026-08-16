package pan123open

import (
	"context"
	"net/url"
	"strings"

	"xmedia/internal/domain"
	"xmedia/internal/driver"
)

// 123 网盘分享转存协议说明：
// 123 分享转存走 Web 端登录态 API（www.123pan.com 会话 + 分享校验），
// 123_Open 驱动使用 Token 认证体系，无分享转存端点。
// 与 115 相同的协议断层（D23 全 OAuth/Token 决策的系统性副作用）。

// parse123ShareLink 从 123 网盘分享 URL 提取分享码与提取码。
// 支持 https://www.123pan.com/s/{code}.html 与 .com/s/{code} 形态，提取码在 query/密码字段。
func parse123ShareLink(raw string) (shareCode, passcode string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", domain.Errorf(domain.CodeValidation, "分享链接不能为空")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", domain.Errorf(domain.CodeValidation, "分享链接格式错误")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) == 0 {
		return "", "", domain.Errorf(domain.CodeValidation, "分享链接缺少分享码")
	}
	code := strings.TrimSpace(parts[len(parts)-1])
	code = strings.TrimSuffix(code, ".html")
	code = strings.TrimSuffix(code, ".htm")
	if code == "" {
		return "", "", domain.Errorf(domain.CodeValidation, "分享链接缺少分享码")
	}
	passcode = u.Query().Get("pwd")
	return code, passcode, nil
}

// SaveShare 实现 driver.ShareSaver。
// Token 凭据无分享转存能力，诚实返回能力错误（真实失败非假成功）。
func (d *Driver) SaveShare(ctx context.Context, req driver.ShareRequest) (*driver.ShareResult, error) {
	if _, _, err := parse123ShareLink(req.ShareURL); err != nil {
		return nil, err
	}
	return nil, domain.Errorf(domain.CodePermissionDenied,
		"123 网盘分享转存需要 Web 登录态会话，当前 123_Open（Token）凭据不支持；请使用支持分享转存的网盘账号或等待 123 会话凭据支持")
}
