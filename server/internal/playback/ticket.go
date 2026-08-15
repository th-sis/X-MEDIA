package playback

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"time"

	"xmedia/internal/domain"
)

// TicketClaims 播放票据载荷。
type TicketClaims struct {
	TaskID     int64  `json:"t"`
	AccountID  int64  `json:"a"`
	FileID     string `json:"f"`
	Source     string `json:"s"`
	ExternalID int64  `json:"e"`
	ExpiresAt  int64  `json:"x"`
}

// TicketSigner 生成与校验播放票据。
type TicketSigner struct {
	configs domain.ConfigRepository
}

func NewTicketSigner(configs domain.ConfigRepository) *TicketSigner {
	return &TicketSigner{configs: configs}
}

// secret 返回签名密钥，缺失时生成并持久化。
func (ts *TicketSigner) secret(ctx context.Context) ([]byte, error) {
	v, ok, err := ts.configs.Get(ctx, domain.ConfigTicketSigningSecret)
	if err != nil {
		return nil, err
	}
	if ok && v != "" {
		b, err := hex.DecodeString(v)
		if err == nil && len(b) > 0 {
			return b, nil
		}
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	if err := ts.configs.Set(ctx, domain.ConfigTicketSigningSecret, hex.EncodeToString(raw)); err != nil {
		return nil, err
	}
	return raw, nil
}

// Sign 签发票据。ttl 为有效时长，<=0 时按 source 取默认值。
func (ts *TicketSigner) Sign(ctx context.Context, claims TicketClaims, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = defaultTicketTTL(claims.Source)
	}
	claims.ExpiresAt = time.Now().Add(ttl).Unix()
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	b64 := base64.RawURLEncoding.EncodeToString(payload)
	secret, err := ts.secret(ctx)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(b64))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return b64 + "." + sig, nil
}

// Verify 校验票据，过期/签名不符返回 AppError。
func (ts *TicketSigner) Verify(ctx context.Context, ticket string) (*TicketClaims, error) {
	dot := -1
	for i := len(ticket) - 1; i >= 0; i-- {
		if ticket[i] == '.' {
			dot = i
			break
		}
	}
	if dot <= 0 || dot == len(ticket)-1 {
		return nil, domain.Errf(domain.CodeValidation)
	}
	b64, sig := ticket[:dot], ticket[dot+1:]
	secret, err := ts.secret(ctx)
	if err != nil {
		return nil, err
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(b64))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return nil, domain.Errf(domain.CodeAuthExpired)
	}
	payload, err := base64.RawURLEncoding.DecodeString(b64)
	if err != nil {
		return nil, domain.Errf(domain.CodeValidation)
	}
	var claims TicketClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, domain.Errf(domain.CodeValidation)
	}
	if claims.ExpiresAt > 0 && time.Now().Unix() > claims.ExpiresAt {
		return nil, domain.Errf(domain.CodeAuthExpired)
	}
	return &claims, nil
}

func defaultTicketTTL(source string) time.Duration {
	switch source {
	case "nas":
		return 24 * time.Hour
	case "quark":
		return time.Hour
	case "demo":
		return 2 * time.Hour
	default:
		return 2 * time.Hour
	}
}
