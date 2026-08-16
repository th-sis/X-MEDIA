package guangya

import (
	"context"
	"net/url"
	"strings"

	"xmedia/internal/domain"
	"xmedia/internal/driver"
)

// 光鸭网盘分享转存协议说明：
// 光鸭为小众网盘，分享转存需 Web 登录态会话；Guangya 驱动使用 Token
// 认证体系，无分享转存端点。与 115 相同的协议断层。

// parseGuangyaShareLink 从光鸭分享 URL 提取分享标识。
// 通用解析：取路径最后一段作为分享标识（域名不限）。
func parseGuangyaShareLink(raw string) (shareID string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", domain.Errorf(domain.CodeValidation, "分享链接不能为空")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", domain.Errorf(domain.CodeValidation, "分享链接格式错误")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[len(parts)-1]) == "" {
		return "", domain.Errorf(domain.CodeValidation, "分享链接缺少分享标识")
	}
	shareID = strings.TrimSpace(parts[len(parts)-1])
	return shareID, nil
}

// SaveShare 实现 driver.ShareSaver。
// Token 凭据无分享转存能力，诚实返回能力错误（真实失败非假成功）。
func (d *Driver) SaveShare(ctx context.Context, req driver.ShareRequest) (*driver.ShareResult, error) {
	if _, err := parseGuangyaShareLink(req.ShareURL); err != nil {
		return nil, err
	}
	return nil, domain.Errorf(domain.CodePermissionDenied,
		"光鸭分享转存需要 Web 登录态会话，当前 Guangya（Token）凭据不支持；请使用支持分享转存的网盘账号")
}
