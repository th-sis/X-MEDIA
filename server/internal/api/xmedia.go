package api

import (
	"encoding/json"
	"net/http"
)

// X-MEDIA 播放器 API 响应契约（§18.1），与 LitePan 管理后台的 {success,data} 格式分离。

func writeJSONBody(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeList 列表响应：{items,page,has_more,total}。
func writeList(w http.ResponseWriter, items any, page int, hasMore bool, total int) {
	writeJSONBody(w, http.StatusOK, map[string]any{
		"items":    items,
		"page":     page,
		"has_more": hasMore,
		"total":    total,
	})
}

// writeAPIError 错误响应：{error,code,action}。
func writeAPIError(w http.ResponseWriter, status int, code, msg, action string) {
	writeJSONBody(w, status, map[string]any{
		"error":  msg,
		"code":   code,
		"action": action,
	})
}

// decodeBody 宽松解析 JSON 请求体（忽略未知字段）。
func decodeBody(r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return err
	}
	return nil
}
