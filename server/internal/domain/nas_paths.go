package domain

import (
	"encoding/json"
	"strings"
)

// NASPathList NAS 媒体源路径列表（容器内绝对路径）。
// 每条路径对应 NAS 父目录下的一个子目录（如 SMB 共享挂载后的子目录），
// 索引引擎会逐条遍历并合并结果。
type NASPathList []string

// ParseNASPaths 从配置值解析 NAS 路径列表（兼容旧单字符串 nas_local_path）。
//
// 优先级：
//  1. 解析 nas_local_paths（JSON 数组）；空数组回退到步骤 2
//  2. 回退到旧单字符串 nas_local_path；非空回退到此值包成单元素数组
//  3. 都为空返回空列表
//
// 同时：
//   - 自动 trim 前后空白
//   - 过滤空字符串
//   - 去重
//   - 拒绝明显非绝对路径（不以 "/" 开头且不匹配 Windows 盘符 "X:\")
//
// 返回的列表可直接用于 filepath.WalkDir；调用方负责确认目录存在性。
func ParseNASPaths(localPathsJSON string, legacyLocalPath string) NASPathList {
	out := make(NASPathList, 0)

	// 步骤 1：新格式 JSON 数组
	if s := strings.TrimSpace(localPathsJSON); s != "" {
		var arr []string
		if err := json.Unmarshal([]byte(s), &arr); err == nil {
			for _, p := range arr {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				if !isAbsolutePath(p) {
					continue // 拒绝相对路径（含非绝对 Windows 路径）
				}
				if !contains(out, p) {
					out = append(out, p)
				}
			}
		}
		// JSON 解析失败时静默回退到旧字段（不抛错，避免阻塞扫描）
	}

	// 步骤 2：旧单字符串（向后兼容迁移期）
	if len(out) == 0 {
		if legacy := strings.TrimSpace(legacyLocalPath); legacy != "" {
			if isAbsolutePath(legacy) {
				out = append(out, legacy)
			}
		}
	}

	return out
}

// isAbsolutePath 判断是否绝对路径：
//   - Unix 风格：以 "/" 开头
//   - Windows 风格：以 "<盘符>:/" 或 "<盘符>:\" 开头（单测本地友好）
func isAbsolutePath(p string) bool {
	if strings.HasPrefix(p, "/") {
		return true
	}
	// Windows: "C:\..." or "C:/..." or "C:"（少见但兼容）
	if len(p) >= 3 && p[1] == ':' && (p[2] == '/' || p[2] == '\\') {
		c := p[0]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			return true
		}
	}
	return false
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
