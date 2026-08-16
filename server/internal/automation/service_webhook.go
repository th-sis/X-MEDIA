package automation

import (
	"context"
	"crypto/subtle"
	"strings"

	"xmedia/internal/domain"
)

// ConfigAutomationWebhookToken 是 webhook 回调的共享密钥配置键（§13.1 裁剪：
// API Key 表随 apikey 模块移除，webhook 鉴权简化为单共享 token）。值为空时拒绝所有回调。
const ConfigAutomationWebhookToken = "automation_webhook_token"

func (s *Service) TriggerWebhook(ctx context.Context, authHeader string, event WebhookEvent) (map[string]any, error) {
	raw, err := bearerToken(authHeader)
	if err != nil {
		return nil, err
	}
	expected := ""
	if s.configs != nil {
		if v, ok, err := s.configs.Get(ctx, ConfigAutomationWebhookToken); err == nil && ok {
			expected = strings.TrimSpace(v)
		}
	}
	if expected == "" || subtle.ConstantTimeCompare([]byte(raw), []byte(expected)) != 1 {
		return nil, domain.Errorf(domain.CodePermissionDenied, "Webhook Token 无效")
	}
	eventName := strings.TrimSpace(event.Event)
	if eventName == "" {
		return nil, domain.Errorf(domain.CodeValidation, "event 不能为空")
	}
	rows, err := s.rules.List(ctx, false)
	if err != nil {
		return nil, err
	}
	matched := 0
	triggered := make([]map[string]any, 0)
	for _, row := range rows {
		if row.TriggerType != domain.AutomationTriggerWebhook {
			continue
		}
		cfg := decodeMap(row.TriggerConfig)
		if !matchWebhook(cfg, event) {
			continue
		}
		matched++
		s.submitRun(row.ID, "webhook", false)
		triggered = append(triggered, map[string]any{"id": row.ID, "name": row.Name})
	}
	return map[string]any{
		"event":     event.Event,
		"source":    event.Source,
		"path":      event.Path,
		"matched":   matched,
		"triggered": triggered,
	}, nil
}

func matchWebhook(cfg map[string]any, event WebhookEvent) bool {
	if strings.TrimSpace(anyString(cfg["event"])) != strings.TrimSpace(event.Event) {
		return false
	}
	source := strings.TrimSpace(anyString(cfg["source"]))
	if source != "" && source != strings.TrimSpace(event.Source) {
		return false
	}
	pathPrefix := normalizePath(anyString(cfg["path_prefix"]))
	if pathPrefix != "/" && pathPrefix != "" {
		eventPath := normalizePath(event.Path)
		if eventPath != pathPrefix && !strings.HasPrefix(eventPath, strings.TrimRight(pathPrefix, "/")+"/") {
			return false
		}
	}
	return true
}

func bearerToken(header string) (string, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", domain.Errorf(domain.CodeAdminAuthRequired, "缺少 Authorization")
	}
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return "", domain.Errorf(domain.CodePermissionDenied, "Authorization 格式错误")
	}
	token := strings.TrimSpace(header[7:])
	if token == "" {
		return "", domain.Errorf(domain.CodePermissionDenied, "缺少 Webhook Token")
	}
	return token, nil
}
