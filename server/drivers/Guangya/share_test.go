package guangya

import (
	"context"
	"testing"

	"xmedia/internal/domain"
	"xmedia/internal/driver"
)

// TestParseGuangyaShareLink 覆盖光鸭分享 URL 形态（Phase 7b 验收）。
func TestParseGuangyaShareLink(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		shareID string
		wantErr bool
	}{
		{"标准形态", "https://guangya.com/s/gy12345", "gy12345", false},
		{"带密码参数", "https://guangya.com/s/gy12345?pwd=666", "gy12345", false},
		{"尾部斜杠", "https://guangya.com/s/gy12345/", "gy12345", false},
		{"空链接", "", "", true},
		{"无分享标识", "https://guangya.com/", "", true},
		{"畸形", "://bad", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shareID, err := parseGuangyaShareLink(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("期望报错: shareID=%q", shareID)
				}
				return
			}
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			if shareID != tc.shareID {
				t.Fatalf("解析 = %q, want %q", shareID, tc.shareID)
			}
		})
	}
}

// TestSaveShareTokenReturnsCapabilityError Token 凭据诚实失败（非假成功）。
func TestSaveShareTokenReturnsCapabilityError(t *testing.T) {
	d := &Driver{}
	_, err := d.SaveShare(context.Background(), driver.ShareRequest{
		ShareURL: "https://guangya.com/s/gy12345",
	})
	if err == nil {
		t.Fatalf("Token 凭据下应返回能力错误")
	}
	ae, ok := domain.AsAppError(err)
	if !ok || ae.Code != domain.CodePermissionDenied {
		t.Fatalf("应为 PermissionDenied AppError: %v", err)
	}
}

func TestSaveShareInvalidLinkFailsFast(t *testing.T) {
	d := &Driver{}
	_, err := d.SaveShare(context.Background(), driver.ShareRequest{ShareURL: ""})
	if err == nil {
		t.Fatalf("空链接应报错")
	}
	ae, ok := domain.AsAppError(err)
	if !ok || ae.Code != domain.CodeValidation {
		t.Fatalf("应为 Validation AppError: %v", err)
	}
}
