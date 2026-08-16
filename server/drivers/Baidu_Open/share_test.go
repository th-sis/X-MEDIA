package baiduopen

import (
	"context"
	"testing"

	"xmedia/internal/domain"
	"xmedia/internal/driver"
)

// TestParseBaiduShareLink 覆盖百度分享 URL 形态（Phase 7b 验收）。
func TestParseBaiduShareLink(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		surl     string
		passcode string
		wantErr  bool
	}{
		{"带密码", "https://pan.baidu.com/s/1AbCdEf?pwd=8x3k", "1AbCdEf", "8x3k", false},
		{"无密码", "https://pan.baidu.com/s/1AbCdEf", "1AbCdEf", "", false},
		{"尾部斜杠", "https://pan.baidu.com/s/1AbCdEf/", "1AbCdEf", "", false},
		{"附加参数", "https://pan.baidu.com/s/1AbCdEf?pwd=8x3k&from=share", "1AbCdEf", "8x3k", false},
		{"空链接", "", "", "", true},
		{"无 surl", "https://pan.baidu.com/", "", "", true},
		{"畸形", "://bad", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			surl, passcode, err := parseBaiduShareLink(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("期望报错: surl=%q", surl)
				}
				return
			}
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			if surl != tc.surl || passcode != tc.passcode {
				t.Fatalf("解析 = (%q, %q), want (%q, %q)", surl, passcode, tc.surl, tc.passcode)
			}
		})
	}
}

// TestSaveShareOAuthReturnsCapabilityError OAuth 凭据诚实失败（非假成功）。
func TestSaveShareOAuthReturnsCapabilityError(t *testing.T) {
	d := &Driver{}
	_, err := d.SaveShare(context.Background(), driver.ShareRequest{
		ShareURL: "https://pan.baidu.com/s/1AbCdEf?pwd=8x3k",
	})
	if err == nil {
		t.Fatalf("OAuth 凭据下应返回能力错误")
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
