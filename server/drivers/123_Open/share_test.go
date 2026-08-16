package pan123open

import (
	"context"
	"testing"

	"xmedia/internal/domain"
	"xmedia/internal/driver"
)

// TestParse123ShareLink 覆盖 123 分享 URL 形态（Phase 7b 验收）。
func TestParse123ShareLink(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		code     string
		passcode string
		wantErr  bool
	}{
		{"html 形态", "https://www.123pan.com/s/AbCdEf.html", "AbCdEf", "", false},
		{"html 带密码", "https://www.123pan.com/s/AbCdEf.html?pwd=1234", "AbCdEf", "1234", false},
		{"无后缀", "https://www.123pan.com/s/AbCdEf", "AbCdEf", "", false},
		{"尾部斜杠", "https://www.123pan.com/s/AbCdEf/", "AbCdEf", "", false},
		{"空链接", "", "", "", true},
		{"无分享码", "https://www.123pan.com/", "", "", true},
		{"畸形", "://bad", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, passcode, err := parse123ShareLink(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("期望报错: code=%q", code)
				}
				return
			}
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			if code != tc.code || passcode != tc.passcode {
				t.Fatalf("解析 = (%q, %q), want (%q, %q)", code, passcode, tc.code, tc.passcode)
			}
		})
	}
}

// TestSaveShareTokenReturnsCapabilityError Token 凭据诚实失败（非假成功）。
func TestSaveShareTokenReturnsCapabilityError(t *testing.T) {
	d := &Driver{}
	_, err := d.SaveShare(context.Background(), driver.ShareRequest{
		ShareURL: "https://www.123pan.com/s/AbCdEf.html",
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
