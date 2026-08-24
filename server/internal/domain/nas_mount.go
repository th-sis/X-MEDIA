package domain

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// [V7 整改 commit #4] NAS 主机路径 -> 容器路径 自动映射。
//
// 用户视角：管理后台输入"主机路径"（如 /mnt/BTORAGE/Asia-Movie），
// 后端自动 prefix rewrite 成容器内路径（/mnt/nas-root/Asia-Movie），
// 然后再走 filepath.WalkDir / os.Stat。
//
// 映射规则来源：
//  1. 自动探测：启动时读 /proc/self/mountinfo 找 cifs/smbfs 挂载点
//  2. 用户手动覆盖：管理后台写入 configs 表 key=nas_mount_<host_path>
//
// 容器内查找顺序（先到先得）：
//   - DB 配置的映射
//   - 自动探测的 mount
//   - 原样返回（假设用户已直接填容器内路径）

// NASMountMap 主机路径 -> 容器路径映射。
// key 标准化为绝对路径（trim 末尾 /），value 同。
type NASMountMap map[string]string

// ConfigKeyPrefixNASMount NAS 挂载映射前缀（[V7 整改 commit #4]）。
// 完整 key: nas_mount_<host_path>，value: container_path。
// ConfigKeyPrefix 必须导出供 allowedConfigKey 校验。
const ConfigKeyPrefixNASMount = "nas_mount_"

// MountInfoEntry 探测到的 SMB 挂载项。
type MountInfoEntry struct {
	// Filesystem 形如 cifs/smbfs/nfs 等
	Filesystem string `json:"filesystem"`
	// MountTarget 容器内挂载点，如 /mnt/nas-root
	MountTarget string `json:"mount_target"`
	// Source 挂载源，例如 //192.168.1.10/media
	Source string `json:"source"`
}

// ProbeNASMounts 扫描 /proc/self/mountinfo 提取 cifs/smbfs 挂载点。
// 返回的 MountTarget 是容器内路径（即 mountinfo 真实记录的 path）。
//
// 返回空切片 + nil 表示容器内没有 SMB 挂载（正常裸机部署场景）。
// 返回非 nil error 表示读取 /proc/self/mountinfo 失败（极少见）。
func ProbeNASMounts() ([]MountInfoEntry, error) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return nil, fmt.Errorf("打开 /proc/self/mountinfo 失败: %w", err)
	}
	defer f.Close()
	return parseMountInfo(f)
}

func parseMountInfo(r *os.File) ([]MountInfoEntry, error) {
	var out []MountInfoEntry
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		// /proc/self/mountinfo 行格式（空格分隔）：
		//   mount-id parent-major:parent-minor major:minor root mount-target options - fs-type source super-options
		// 例: 36 22 0:21 / /mnt/nas-root rw,relatime master:1 - cifs //nas/media rw,vers=3.1.1,...
		line := scanner.Text()
		fields := strings.Split(line, " ")
		if len(fields) < 10 {
			continue
		}
		// 找 "-" 分隔符（必须在 fs-type 之前）
		dashIdx := -1
		for i, f := range fields {
			if f == "-" {
				dashIdx = i
				break
			}
		}
		if dashIdx < 0 || dashIdx+3 > len(fields) {
			continue
		}
		fsType := fields[dashIdx+1]
		// 仅关心 cifs / smbfs（SMB 共享）
		if fsType != "cifs" && fsType != "smbfs" {
			continue
		}
		mountTarget := unescapeMountField(fields[4])
		source := unescapeMountField(fields[dashIdx+2])
		out = append(out, MountInfoEntry{
			Filesystem: fsType,
			MountTarget: mountTarget,
			Source:     source,
		})
	}
	return out, scanner.Err()
}

// unescapeMountField /proc/self/mountinfo 字段用 \040 (空格) \011 (tab) \012 (newline) \134 (\) 转义。
func unescapeMountField(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			nn := s[i+1 : i+4]
			switch nn {
			case "040":
				b.WriteByte(' ')
			case "011":
				b.WriteByte('\t')
			case "012":
				b.WriteByte('\n')
			case "134":
				b.WriteByte('\\')
			default:
				b.WriteByte(s[i])
			}
			i += 3
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// ResolveNASPath 把用户填的路径 rewrite 成容器内绝对路径。
// 输入是 NASSources.path 原始值（用户可能是主机路径或容器内路径）。
// 返回 rewrite 后的容器内路径 + 真值来源（auto_detected / explicit / passthrough）。
//
// Rewrite 规则（按优先级）：
//  1. mounts 配置中精确匹配 host_path 命中 -> 用配置的 container_path
//  2. mounts 配置中 prefix 命中（最长前缀）-> 替换前缀
//  3. detected 列表中精确匹配 host_path 命中
//  4. detected 列表中 prefix 命中（最长前缀）
//  5. 路径已"看起来像容器内路径"（在常见 mount 根下）-> 原样
//  6. 都不命中 -> 原样 + passthrough（不报错，让 os.Stat 失败时上层处理）
func ResolveNASPath(rawPath string, mounts NASMountMap, detected []MountInfoEntry) (resolved string, source string) {
	if rawPath == "" {
		return "", "passthrough"
	}
	cleaned := strings.TrimSpace(rawPath)
	cleaned = strings.TrimRight(cleaned, "/")
	// 1+2: configured mounts (exact, then longest prefix)
	if hit, src := matchMountMap(cleaned, mounts); hit != "" {
		return hit, src
	}
	// 3+4: detected mounts (exact, then longest prefix)
	if hit, src := matchDetected(cleaned, detected); hit != "" {
		return hit, src
	}
	// 4.5: [V7 §9.4 UI-first] SMB 源别名推导 — 用户填的是主机视角路径
	// (/mnt/BTORAGE/...), 容器只认挂载点 /mnt/nas-root ← //ip/BTORAGE.
	// 从 share 名推导 "/BTORAGE"、"/mnt/BTORAGE" 等主机侧别名前缀再匹配,
	// 让用户无需理解容器路径即可完成配置.
	if hit, _ := matchMountMap(cleaned, DeriveNASMountMap(detected)); hit != "" {
		return hit, "smb_alias"
	}
	// 5: passthrough (assume already container-internal)
	return cleaned, "passthrough"
}

// DeriveNASMountMap 从探测到的 SMB 挂载源推导主机侧别名映射.
// Source 形如 //192.168.7.154/BTORAGE → 取 share 名 "BTORAGE",
// 生成 {"/BTORAGE": target, "/mnt/BTORAGE": target} 两个候选别名.
// 非 SMB 挂载 (overlay/nfs/本地盘) 不参与推导.
func DeriveNASMountMap(detected []MountInfoEntry) NASMountMap {
	out := NASMountMap{}
	for _, m := range detected {
		if m.Filesystem != "cifs" && m.Filesystem != "smbfs" {
			continue
		}
		src := strings.TrimRight(strings.TrimSpace(m.Source), "/")
		if !strings.HasPrefix(src, "//") {
			continue
		}
		rest := strings.TrimPrefix(src, "//") // host/share[/sub...]
		hostEnd := strings.IndexByte(rest, '/')
		if hostEnd <= 0 {
			continue
		}
		sharePart := rest[hostEnd+1:] // share[/sub...]
		if idx := strings.IndexByte(sharePart, '/'); idx >= 0 {
			sharePart = sharePart[:idx]
		}
		share := strings.TrimSpace(sharePart)
		if share == "" || m.MountTarget == "" {
			continue
		}
		target := strings.TrimRight(m.MountTarget, "/")
		if target == "" {
			continue
		}
		for _, alias := range []string{"/" + share, "/mnt/" + share} {
			if _, exists := out[alias]; !exists {
				out[alias] = target
			}
		}
	}
	return out
}

// matchMountMap 在 mounts 里查 longest prefix 匹配（先精确后 prefix）。
func matchMountMap(p string, mounts NASMountMap) (string, string) {
	if mounts == nil {
		return "", ""
	}
	if v, ok := mounts[p]; ok {
		return v, "explicit"
	}
	best := ""
	bestSrc := ""
	bestHost := ""
	for host, container := range mounts {
		hostClean := strings.TrimRight(host, "/")
		if hostClean == "" {
			continue
		}
		if p == hostClean {
			return container, "explicit"
		}
		// prefix 匹配：必须以 host/ 开头（避免 /mnt/a 错配 /mnt/abc）
		if strings.HasPrefix(p, hostClean+"/") {
			if len(hostClean) > len(bestHost) {
				best = container + strings.TrimPrefix(p, hostClean)
				bestSrc = "explicit"
				bestHost = hostClean
			}
		}
	}
	return best, bestSrc
}

// matchDetected 跟 matchMountMap 一样，但 source 来源是 auto_detected。
func matchDetected(p string, detected []MountInfoEntry) (string, string) {	if len(detected) == 0 {
		return "", ""
	}
	for _, m := range detected {
		mt := strings.TrimRight(m.MountTarget, "/")
		if mt == "" {
			continue
		}
		if p == mt {
			return mt, "auto_detected"
		}
	}
	best := ""
	bestSrc := ""
	bestMt := ""
	for _, m := range detected {
		mt := strings.TrimRight(m.MountTarget, "/")
		if strings.HasPrefix(p, mt+"/") {
			if len(mt) > len(bestMt) {
				best = mt + strings.TrimPrefix(p, mt)
				bestSrc = "auto_detected"
				bestMt = mt
			}
		}
	}
	return best, bestSrc
}

// LoadNASMountMap 从 configs Map 读 nas_mount_* keys 装配成 NASMountMap。
// 输入应该是 configs.All() 返回的 map[string]string]。
//
// 严格检查：
//  1. key 必须以 "nas_mount_" 开头且剩余部分非空
//  2. host 必须是绝对路径（以 / 开头），避免 "nas_mount_no_prefix_wrong"
//     之类 anti-pattern 误匹配
func LoadNASMountMap(all map[string]string) NASMountMap {
	out := NASMountMap{}
	const prefix = ConfigKeyPrefixNASMount
	for k, v := range all {
		if len(k) <= len(prefix) || k[:len(prefix)] != prefix {
			continue
		}
		host := k[len(prefix):]
		if host == "" || v == "" {
			continue
		}
		// 强制 host 必须是绝对路径（避免 key 形如 "nas_mount_no_prefix_wrong" 通过）
		if !strings.HasPrefix(host, "/") {
			continue
		}
		host = strings.TrimRight(host, "/")
		v = strings.TrimRight(v, "/")
		if host == "" || v == "" {
			continue
		}
		out[host] = v
	}
	return out
}

// RenderNASMountMap 把 NASMountMap 写回 configs KV 格式。
// 返回 (setMap, deleteKeys)：
//   - setMap 要写到 DB 的 key -> value
//   - deleteKeys 要从 DB 删的旧 key（仅清理与 frontend 同步传过来的不一致项）
func RenderNASMountMap(m NASMountMap) map[string]string {
	out := make(map[string]string, len(m))
	for host, container := range m {
		if host == "" || container == "" {
			continue
		}
		out[ConfigKeyPrefixNASMount+host] = container
	}
	return out
}
