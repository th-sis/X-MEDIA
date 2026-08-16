package domain

import (
	"testing"
)

func TestParseNASPaths(t *testing.T) {
	tests := []struct {
		name    string
		newJSON string
		legacy  string
		want    NASPathList
	}{
		{
			name:    "新格式 JSON 数组",
			newJSON: `["/mnt/nas-root/Asia-Movie","/mnt/nas-root/Western-Movie"]`,
			want:    NASPathList{"/mnt/nas-root/Asia-Movie", "/mnt/nas-root/Western-Movie"},
		},
		{
			name:    "新格式 + 旧单字符串：新优先",
			newJSON: `["/mnt/nas-root/A"]`,
			legacy:  "/mnt/nas-root/B",
			want:    NASPathList{"/mnt/nas-root/A"},
		},
		{
			name:   "新格式空 → 回退旧单字符串",
			legacy: "/mnt/nas-root/Legacy",
			want:   NASPathList{"/mnt/nas-root/Legacy"},
		},
		{
			name:    "新格式带空白 + 重复 + 空字符串过滤",
			newJSON: `["  /mnt/a  ","/mnt/a","","/mnt/b"]`,
			want:    NASPathList{"/mnt/a", "/mnt/b"},
		},
		{
			name:    "相对路径被拒绝",
			newJSON: `["relative/path","/mnt/absolute"]`,
			want:    NASPathList{"/mnt/absolute"},
		},
		{
			name:    "新格式 JSON 损坏 → 回退旧字段",
			newJSON: `not-a-json`,
			legacy:  "/mnt/nas-root/Fallback",
			want:    NASPathList{"/mnt/nas-root/Fallback"},
		},
		{
			name: "全空 → 空列表",
			want: NASPathList{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseNASPaths(tc.newJSON, tc.legacy)
			if !equalPathList(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func equalPathList(a, b NASPathList) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
