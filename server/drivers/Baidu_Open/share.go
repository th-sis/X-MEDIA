package baiduopen

import (
	"context"
	"net/url"
	"strings"

	"xmedia/internal/domain"
	"xmedia/internal/driver"
)

// 百度分享转存协议说明（BaiduPCS-Go canonical 实现考古，2026-08-16）：
//   1. 解析分享链接 https://pan.baidu.com/s/{surl}?pwd={passcode}
//   2. GET /share/init?surl={surl} 携带 BDUSS cookie 获取 bdstoken
//   3. POST /share/transfer（shareid/from/fs_id/bdstoken）执行转存
// 该链路依赖 pan.baidu.com 的登录态（BDUSS cookie）；百度 Open（Baidu_Open）
// 驱动走 OAuth 授权体系，无此会话。与 115 相同的协议断层（D23 全 OAuth 决策）。

const (
	baiduShareInitPath     = "/share/init"
	baiduShareTransferPath = "/share/transfer"
)

// parseBaiduShareLink 从百度分享 URL 提取 surl 与 passcode。
// 支持 https://pan.baidu.com/s/{surl}?pwd=xxxx 与 ?pwd= 缺省形态。
func parseBaiduShareLink(raw string) (surl, passcode string, err error) {
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
		return "", "", domain.Errorf(domain.CodeValidation, "分享链接缺少 surl")
	}
	surl = strings.TrimSpace(parts[len(parts)-1])
	passcode = u.Query().Get("pwd")
	return surl, passcode, nil
}

// SaveShare 实现 driver.ShareSaver。
// OAuth 凭据无分享转存能力（需 BDUSS 登录态），诚实返回能力错误；
// resolve P1 按优先级自动跳过继续其他候选（真实失败非假成功）。
func (d *Driver) SaveShare(ctx context.Context, req driver.ShareRequest) (*driver.ShareResult, error) {
	if _, _, err := parseBaiduShareLink(req.ShareURL); err != nil {
		return nil, err
	}
	return nil, domain.Errorf(domain.CodePermissionDenied,
		"百度分享转存需要登录态（pan.baidu.com BDUSS）会话，当前百度 Open（OAuth）凭据不支持；请使用支持分享转存的网盘账号或等待百度会话凭据支持")
}

// ---- 协议函数（未来 BDUSS 会话凭据接入后启用） ----

// initBaiduShare 访问分享页获取 bdstoken（需 BDUSS cookie）。
func (d *Driver) initBaiduShare(ctx context.Context, surl string) (string, error) {
	_ = ctx
	_ = baiduShareInitPath
	_ = surl
	return "", domain.Errorf(domain.CodePermissionDenied, "百度分享查询需要 BDUSS 会话")
}

// transferBaiduShare 执行分享转存（需 BDUSS cookie + bdstoken）。
func (d *Driver) transferBaiduShare(ctx context.Context, surl, passcode, bdstoken string) error {
	_ = ctx
	_ = baiduShareTransferPath
	_ = surl
	_ = passcode
	_ = bdstoken
	return domain.Errorf(domain.CodePermissionDenied, "百度分享转存需要 BDUSS 会话")
}
