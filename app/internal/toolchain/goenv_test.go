package toolchain

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInspectConfigDisabledDisablesActiveBinary(t *testing.T) {
	t.Parallel()

	goBinary, err := exec.LookPath(goExecutableName())
	if err != nil {
		t.Skip("当前环境未找到 go，可跳过")
	}

	state, err := InspectConfig(Config{
		SelectedBinary: goBinary,
		KnownBinaries:  []string{goBinary},
		Disabled:       true,
	})
	if err != nil {
		t.Fatalf("InspectConfig 返回错误: %v", err)
	}
	if state.HasUsableBinary {
		t.Fatalf("禁用 Go 环境后不应存在可用二进制")
	}
	if state.ActiveBinary != "" {
		t.Fatalf("禁用 Go 环境后不应存在激活路径: %s", state.ActiveBinary)
	}
	if state.StatusMessage == "" {
		t.Fatalf("禁用 Go 环境后应返回状态提示")
	}
}

func TestSameFilePathResolvesSymlinkAliases(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "python-real")
	alias := filepath.Join(dir, "python-link")
	if err := os.WriteFile(target, []byte("binary"), 0755); err != nil {
		t.Fatalf("写入目标文件失败: %v", err)
	}
	if err := os.Symlink(target, alias); err != nil {
		t.Fatalf("创建软链失败: %v", err)
	}

	if !sameFilePath(target, alias) {
		t.Fatalf("期望软链与真实路径被识别为同一文件")
	}
}

func TestDedupeStringsResolvesSymlinkAliases(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "python-real")
	alias := filepath.Join(dir, "python-link")
	if err := os.WriteFile(target, []byte("binary"), 0755); err != nil {
		t.Fatalf("写入目标文件失败: %v", err)
	}
	if err := os.Symlink(target, alias); err != nil {
		t.Fatalf("创建软链失败: %v", err)
	}

	values := dedupeStrings([]string{alias, target})
	if len(values) != 1 {
		t.Fatalf("期望软链别名被去重，实际得到 %d 项: %#v", len(values), values)
	}
	if values[0] != alias {
		t.Fatalf("期望保留首次出现的路径，得到 %q", values[0])
	}
}
