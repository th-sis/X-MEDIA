package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xmedia/internal/domain"
)

// configStub 测试用 ConfigRepository。
type configStub struct {
	values map[string]string
}

func (c *configStub) Get(_ context.Context, key string) (string, bool, error) {
	v, ok := c.values[key]
	return v, ok, nil
}
func (c *configStub) Set(_ context.Context, key, value string) error {
	if c.values == nil {
		c.values = map[string]string{}
	}
	c.values[key] = value
	return nil
}
func (c *configStub) All(context.Context) (map[string]string, error) { return c.values, nil }

// TestMaskSecret 脱敏边界。
func TestMaskSecret(t *testing.T) {
	if got := maskSecret("short"); got != "********" {
		t.Fatalf("短密钥脱敏 = %q", got)
	}
	if got := maskSecret("abcdefgh12345678"); got != "abcd****5678" {
		t.Fatalf("长密钥脱敏 = %q", got)
	}
}

// TestConfigsGetMasksToken GET 返回白名单键且 token 脱敏。
func TestConfigsGetMasksToken(t *testing.T) {
	h := &configAdminHandlers{configs: &configStub{values: map[string]string{
		domain.ConfigTMDBAPIKey:     "tmdb-key-123",
		domain.ConfigPansearchToken: "tok-abcdef123456",
		"internal_secret":           "should-not-appear",
	}}}
	req := httptest.NewRequest(http.MethodGet, "/api/admin/configs/", nil)
	rec := httptest.NewRecorder()
	h.handleConfigsGet(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 = %d", rec.Code)
	}
	var body struct {
		Data struct {
			Items map[string]string `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应解析失败: %v", err)
	}
	if body.Data.Items[domain.ConfigTMDBAPIKey] != "tmdb-key-123" {
		t.Fatalf("tmdb key 应原样返回: %#v", body.Data.Items)
	}
	if tok := body.Data.Items[domain.ConfigPansearchToken]; tok != "tok-****3456" {
		t.Fatalf("token 应脱敏: %q", tok)
	}
	if _, ok := body.Data.Items["internal_secret"]; ok {
		t.Fatalf("非白名单键不应返回")
	}
}

// TestConfigsPutWhitelist PUT 白名单键成功、非白名单键拒绝。
func TestConfigsPutWhitelist(t *testing.T) {
	h := &configAdminHandlers{configs: &configStub{values: map[string]string{}}}
	send := func(key, value string) *httptest.ResponseRecorder {
		body := map[string]string{"key": key, "value": value}
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPut, "/api/admin/configs/", bytes.NewReader(raw))
		rec := httptest.NewRecorder()
		h.handleConfigsPut(rec, req)
		return rec
	}
	ok := send(domain.ConfigTMDBAPIKey, "k-1")
	if ok.Code != http.StatusOK {
		t.Fatalf("白名单键写入应成功: %d", ok.Code)
	}
	rejected := send("admin_password", "hacked")
	if rejected.Code == http.StatusOK {
		t.Fatalf("内部键应被拒绝写入")
	}
	unknown := send("not_a_real_key", "x")
	if unknown.Code == http.StatusOK {
		t.Fatalf("未知键应被拒绝写入")
	}
}
