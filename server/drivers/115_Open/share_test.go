package pan115open

import (
	"context"
	"testing"

	"xmedia/internal/domain"
	"xmedia/internal/driver"
)

// TestParse115ShareLink 覆盖 4 种 115 分享 URL 格式（Phase 7a 验收项）。
func TestParse115ShareLink(t *testing.T) {
	cases := []struct {
		name        string
		raw         string
		shareCode   string
		receiveCode string
		wantErr     bool
	}{
		{"带密码", "https://115.com/s/swzabc123?password=9999", "swzabc123", "9999", false},
		{"无密码", "https://115.com/s/swzabc123", "swzabc123", "", false},
		{"cdn 域名", "https://115cdn.com/s/swzabc123?password=9999", "swzabc123", "9999", false},
		{"尾部斜杠", "https://115.com/s/swzabc123/", "swzabc123", "", false},
		{"空链接", "", "", "", true},
		{"无 share_code", "https://115.com/", "", "", true},
		{"畸形链接", "://bad", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shareCode, receiveCode, err := parse115ShareLink(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("期望报错，实际成功: %q", shareCode)
				}
				return
			}
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			if shareCode != tc.shareCode || receiveCode != tc.receiveCode {
				t.Fatalf("解析结果 = (%q, %q), want (%q, %q)", shareCode, receiveCode, tc.shareCode, tc.receiveCode)
			}
		})
	}
}

// TestSaveShareOAuthReturnsCapabilityError 验证 OAuth 凭据下诚实失败（非假成功）。
func TestSaveShareOAuthReturnsCapabilityError(t *testing.T) {
	d := &Driver{}
	_, err := d.SaveShare(context.Background(), driver.ShareRequest{
		ShareURL: "https://115.com/s/swzabc123?password=9999",
	})
	if err == nil {
		t.Fatalf("OAuth 凭据下 SaveShare 应返回能力错误")
	}
	ae, ok := domain.AsAppError(err)
	if !ok || ae.Code != domain.CodePermissionDenied {
		t.Fatalf("错误应为 PermissionDenied AppError: %v", err)
	}
}

// TestSaveShareInvalidLinkFailsFast 验证非法链接在能力检查前即失败。
func TestSaveShareInvalidLinkFailsFast(t *testing.T) {
	d := &Driver{}
	_, err := d.SaveShare(context.Background(), driver.ShareRequest{ShareURL: ""})
	if err == nil {
		t.Fatalf("空链接应报错")
	}
	ae, ok := domain.AsAppError(err)
	if !ok || ae.Code != domain.CodeValidation {
		t.Fatalf("错误应为 Validation AppError: %v", err)
	}
}
