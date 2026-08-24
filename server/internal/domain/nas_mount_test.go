package domain

import (
	"os"
	"strings"
	"testing"
)

func TestResolveNASPath(t *testing.T) {
	mounts := NASMountMap{
		"/mnt/BTORAGE": "/mnt/nas-root",
		"/mnt/extra":   "/srv/extra",
	}
	detected := []MountInfoEntry{
		{Filesystem: "cifs", MountTarget: "/mnt/nas-root", Source: "//nas/media"},
		{Filesystem: "cifs", MountTarget: "/srv/extra", Source: "//nas2/extra"},
	}

	tests := []struct {
		name       string
		raw        string
		wantPath   string
		wantSource string
	}{
		{"exact configured mount", "/mnt/BTORAGE/Asia-Movie", "/mnt/nas-root/Asia-Movie", "explicit"},
		{"prefix configured mount", "/mnt/BTORAGE/Sub/Dir", "/mnt/nas-root/Sub/Dir", "explicit"},
		{"trailing slash normalized", "/mnt/BTORAGE/Asia-Movie/", "/mnt/nas-root/Asia-Movie", "explicit"},
		// longest prefix wins: /mnt/BTORAGE is more specific than detected /mnt/nas-root
		{"longest prefix from mounts", "/mnt/BTORAGE/A/B", "/mnt/nas-root/A/B", "explicit"},
		// already container-internal under detected mount -> detected match
		{"under detected mount no rewrite", "/mnt/nas-root/Already", "/mnt/nas-root/Already", "auto_detected"},
		{"under detected mount extra", "/srv/extra/X", "/srv/extra/X", "auto_detected"},
		// not configured + not detected -> passthrough
		{"unknown prefix passthrough", "/random/path", "/random/path", "passthrough"},
		// empty -> passthrough
		{"empty", "", "", "passthrough"},
		// path exactly equals detected mount target
		{"exact detected mount target", "/mnt/nas-root", "/mnt/nas-root", "auto_detected"},
		// similar prefix to avoid false match: /mnt/a must NOT match /mnt/abc
		{"prefix no false match", "/mnt/abc/foo", "/mnt/abc/foo", "passthrough"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotPath, gotSource := ResolveNASPath(tc.raw, mounts, detected)
			if gotPath != tc.wantPath {
				t.Errorf("path: got %q, want %q", gotPath, tc.wantPath)
			}
			if gotSource != tc.wantSource {
				t.Errorf("source: got %q, want %q", gotSource, tc.wantSource)
			}
		})
	}
}

func TestLoadAndRenderNASMountMap(t *testing.T) {
	// Sample configs KV map (with some unrelated keys)
	all := map[string]string{
		nas_mountKey("/mnt/BTORAGE"):    "/mnt/nas-root",
		nas_mountKey("/mnt/extra"):      "/srv/extra",
		"tmdb_api_key":                  "secret",
		nas_mountKey("/mnt/special"):    "/data/special",
		nas_mountKey(""):                "/should/be/ignored",
		nas_mountKey("/mnt/empty"):      "",
		"nas_mount_no_prefix_wrong":    "/mnt/wrong",
	}

	mounts := LoadNASMountMap(all)
	if len(mounts) != 3 {
		t.Fatalf("expected 3 valid mounts, got %d: %v", len(mounts), mounts)
	}
	if mounts["/mnt/BTORAGE"] != "/mnt/nas-root" {
		t.Errorf("missing /mnt/BTORAGE -> /mnt/nas-root")
	}
	if mounts["/mnt/extra"] != "/srv/extra" {
		t.Errorf("missing /mnt/extra -> /srv/extra")
	}
	if mounts["/mnt/special"] != "/data/special" {
		t.Errorf("missing /mnt/special -> /data/special")
	}
	if _, ok := mounts["/mnt/empty"]; ok {
		t.Error("empty container path should be filtered")
	}
	if _, ok := mounts[""]; ok {
		t.Error("empty host path should be filtered")
	}
	if _, ok := mounts["/mnt/special/"]; ok {
		t.Error("trailing slash should be trimmed")
	}

	// Round-trip render
	rendered := RenderNASMountMap(mounts)
	if len(rendered) != 3 {
		t.Errorf("rendered count: got %d, want 3", len(rendered))
	}
	if rendered[nas_mountKey("/mnt/BTORAGE")] != "/mnt/nas-root" {
		t.Errorf("rendered mishap: %v", rendered)
	}
}

func TestProbeNASMountsParsing(t *testing.T) {
	// Construct a fake mountinfo line
	sample := strings.Join([]string{
		"36 22 0:21 / /mnt/nas-root rw,relatime master:1 - cifs //nas/media rw,vers=3.1.1",
		"37 36 0:22 / /proc rw,nosuid,nodev,noexec,relatime - proc proc rw",
		"38 22 0:23 / /mnt/extra\\040with\\040space rw - cifs //192.168.1.5/share rw",
		"", // empty
		"bad line with too few fields",
	}, "\n")
	// Use a synthetic file from string
	f := createTempMountInfo(t, sample)
	defer func() { _ = removeFile(f) }()

	// Manually replicate parseMountInfo logic with our f
	entries, err := probeFromFile(f)
	if err != nil {
		t.Fatalf("probe failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 cifs entries, got %d: %v", len(entries), entries)
	}
	if entries[0].MountTarget != "/mnt/nas-root" {
		t.Errorf("entry 0 mount_target: %s", entries[0].MountTarget)
	}
	if entries[1].MountTarget != "/mnt/extra with space" {
		t.Errorf("entry 1 mount_target (escape): %s", entries[1].MountTarget)
	}
}

// [V7 §9.4 实测回归] 用户在 UI 填主机视角路径 (/mnt/BTORAGE/Asia-Movie),
// 容器内只认得挂载点 /mnt/nas-root ← //192.168.7.154/BTORAGE.
// DeriveNASMountMap 从 SMB 源的 share 名推导主机侧别名前缀, 让
// ResolveNASPath 能完成 host→container 映射 — 这是"用户只管填路径,
// 系统自动识别内网挂载并映射"的核心一步.
func TestDeriveNASMountMap_FromSMBSource(t *testing.T) {
	detected := []MountInfoEntry{
		{Filesystem: "cifs", MountTarget: "/mnt/nas-root", Source: "//192.168.7.154/BTORAGE"},
		{Filesystem: "overlay", MountTarget: "/", Source: "overlay"}, // 非 SMB, 忽略
	}
	got := DeriveNASMountMap(detected)
	want := NASMountMap{
		"/BTORAGE":     "/mnt/nas-root",
		"/mnt/BTORAGE": "/mnt/nas-root",
	}
	if len(got) != len(want) {
		t.Fatalf("derived = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("derived[%q] = %q, want %q", k, got[k], v)
		}
	}
}

func TestResolveNASPath_ViaSMBAlias(t *testing.T) {
	detected := []MountInfoEntry{
		{Filesystem: "cifs", MountTarget: "/mnt/nas-root", Source: "//192.168.7.154/BTORAGE"},
	}
	got, src := ResolveNASPath("/mnt/BTORAGE/Asia-Movie", nil, detected)
	if got != "/mnt/nas-root/Asia-Movie" {
		t.Fatalf("resolved = %q, want /mnt/nas-root/Asia-Movie", got)
	}
	if src != "smb_alias" {
		t.Fatalf("source = %q, want smb_alias", src)
	}
}

// helpers

func nas_mountKey(host string) string {
	return ConfigKeyPrefixNASMount + host
}

// createTempMountInfo writes content to a temp file and returns its path.
func createTempMountInfo(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "mountinfo-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	return f.Name()
}

func removeFile(path string) error {
	return os.Remove(path)
}

// probeFromFile reads a path-stripped mountinfo file and returns parsed entries.
// This duplicates parseMountInfo's parsing logic but reads from a file path
// (the real parseMountInfo reads from *os.File). Used for unit test only.
func probeFromFile(path string) ([]MountInfoEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseMountInfo(f)
}
