package quark

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xmedia/internal/driver"
)

// TestParseShareLink 覆盖 4 种分享 URL 格式（Phase 7a 验收项）。
func TestParseShareLink(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		pwdID    string
		passcode string
		wantErr  bool
	}{
		{"带密码参数", "https://pan.quark.cn/s/abc123?pwd=xyz9", "abc123", "xyz9", false},
		{"无密码", "https://pan.quark.cn/s/abc123", "abc123", "", false},
		{"裸短链", "https://pan.quark.cn/s/9f8e7d6c", "9f8e7d6c", "", false},
		{"带尾部斜杠", "https://pan.quark.cn/s/abc123/", "abc123", "", false},
		{"附加参数干扰", "https://pan.quark.cn/s/abc123?entry=share&pwd=xyz9&fr=pc", "abc123", "xyz9", false},
		{"空链接", "", "", "", true},
		{"无 pwd_id", "https://pan.quark.cn/", "", "", true},
		{"畸形链接", "://not-a-url", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pwdID, passcode, err := parseShareLink(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("期望报错，实际成功: pwdID=%q", pwdID)
				}
				return
			}
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			if pwdID != tc.pwdID || passcode != tc.passcode {
				t.Fatalf("解析结果 = (%q, %q), want (%q, %q)", pwdID, passcode, tc.pwdID, tc.passcode)
			}
		})
	}
}

// TestSaveShareRequestSequence 用 mock 服务器验证 token -> detail -> save -> task -> list
// 的完整请求序列与请求形状（Phase 7a 验收项）。
func TestSaveShareRequestSequence(t *testing.T) {
	var sequence []string
	var handlerErrs []string
	report := func(msg string) { handlerErrs = append(handlerErrs, msg) }

	mux := http.NewServeMux()
	mux.HandleFunc("/share/sharepage/token", func(w http.ResponseWriter, r *http.Request) {
		sequence = append(sequence, "token")
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			report("token 请求体解析失败: " + err.Error())
		}
		if body["pwd_id"] != "abc123" || body["passcode"] != "xyz9" {
			report("token 请求体错误")
		}
		writeQuarkResp(w, map[string]any{"stoken": "st-1"})
	})
	mux.HandleFunc("/share/sharepage/detail", func(w http.ResponseWriter, r *http.Request) {
		sequence = append(sequence, "detail")
		if r.URL.Query().Get("pwd_id") != "abc123" || r.URL.Query().Get("stoken") != "st-1" {
			report("detail query 错误: " + r.URL.RawQuery)
		}
		writeQuarkResp(w, map[string]any{"list": []map[string]any{
			{"fid": "f-1", "file_name": "阿凡达.mkv", "size": 1234, "fid_token": "ft-1", "dir": false},
		}})
	})
	mux.HandleFunc("/share/sharepage/save", func(w http.ResponseWriter, r *http.Request) {
		sequence = append(sequence, "save")
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			report("save 请求体解析失败: " + err.Error())
		}
		fids, _ := body["fid_list"].([]any)
		tokens, _ := body["fid_token_list"].([]any)
		if len(fids) != 1 || fids[0] != "f-1" || len(tokens) != 1 || tokens[0] != "ft-1" {
			report("save 请求体错误")
		}
		if body["to_pdir_fid"] != "target-folder" || body["scene"] != "link" {
			report("save 目标参数错误")
		}
		writeQuarkResp(w, map[string]any{"task_id": "task-9"})
	})
	mux.HandleFunc("/task", func(w http.ResponseWriter, r *http.Request) {
		sequence = append(sequence, "task")
		if r.URL.Query().Get("task_id") != "task-9" {
			report("task query 错误: " + r.URL.RawQuery)
		}
		writeQuarkResp(w, map[string]any{"status": 2})
	})
	mux.HandleFunc("/file/sort", func(w http.ResponseWriter, r *http.Request) {
		sequence = append(sequence, "list")
		writeQuarkResp(w, map[string]any{"list": []map[string]any{
			{"fid": "new-fid-1", "file_name": "阿凡达.mkv", "size": 1234, "dir": false, "file_type": 1},
		}})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	d := &Driver{client: server.Client(), baseURLOverride: server.URL}

	res, err := d.SaveShare(context.Background(), driver.ShareRequest{
		ShareURL:       "https://pan.quark.cn/s/abc123?pwd=xyz9",
		TargetParentID: "target-folder",
	})
	if len(handlerErrs) > 0 {
		t.Fatalf("mock handler 断言失败: %v", handlerErrs)
	}
	if err != nil {
		t.Fatalf("SaveShare 失败: %v", err)
	}
	if res.FileID != "new-fid-1" || res.FileName != "阿凡达.mkv" || res.FileCount != 1 {
		t.Fatalf("转存结果错误: %#v", res)
	}
	want := []string{"token", "detail", "save", "task", "list"}
	if len(sequence) != len(want) {
		t.Fatalf("请求序列 = %v, want %v", sequence, want)
	}
	for i := range want {
		if sequence[i] != want[i] {
			t.Fatalf("请求序列[%d] = %q, want %q（全序 %v）", i, sequence[i], want[i], sequence)
		}
	}
}

func writeQuarkResp(w http.ResponseWriter, data map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": 200, "code": 0, "data": data})
}
