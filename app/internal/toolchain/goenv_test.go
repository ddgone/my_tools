package toolchain

import (
	"context"
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

func TestResolveInstallTargetDirectoryUsesVersionSubdirectory(t *testing.T) {
	t.Parallel()

	target := resolveInstallTargetDirectory("go1.26.3", "/tmp/fire-salamander/toolchains")
	expected := filepath.Join("/tmp/fire-salamander/toolchains", "go1.26.3")
	if target != expected {
		t.Fatalf("期望安装目录为 %q，得到 %q", expected, target)
	}
}

func TestNormalizeInstallBaseDirectoryStripsVersionTail(t *testing.T) {
	t.Parallel()

	base := NormalizeInstallBaseDirectory("/tmp/fire-salamander/toolchains/go1.26.3")
	expected := filepath.Join("/tmp/fire-salamander/toolchains")
	if base != expected {
		t.Fatalf("期望基目录为 %q，得到 %q", expected, base)
	}
}

func TestPreferCandidateSourceKeepsMeaningfulOrigin(t *testing.T) {
	t.Parallel()

	if got := preferCandidateSource(sourceConfigured, sourceManaged); got != sourceManaged {
		t.Fatalf("期望托管 SDK 来源覆盖自定义路径标签，得到 %q", got)
	}
	if got := preferCandidateSource(sourceRemembered, sourcePath); got != sourcePath {
		t.Fatalf("期望 PATH 来源覆盖历史路径标签，得到 %q", got)
	}
	if got := preferCandidateSource(sourceManaged, sourceConfigured); got != sourceManaged {
		t.Fatalf("不应让自定义路径标签覆盖更具体的托管 SDK 来源，得到 %q", got)
	}
}

func TestResolveManagedGoDirectoryRequiresManagedRoot(t *testing.T) {
	t.Parallel()

	managedRoot := filepath.Join("/tmp", "fire-salamander", "toolchains")
	managedBinary := filepath.Join(managedRoot, "go1.26.3", "bin", goExecutableName())
	if dir, ok := resolveManagedGoDirectory(managedBinary, managedRoot); !ok || dir != filepath.Join(managedRoot, "go1.26.3") {
		t.Fatalf("期望识别托管 Go 目录，得到 dir=%q ok=%v", dir, ok)
	}
	if _, ok := resolveManagedGoDirectory("/usr/local/go/bin/go", managedRoot); ok {
		t.Fatalf("系统 Go 不应被识别为托管目录")
	}
}

func TestDeleteManagedGoEnvironmentRemovesSelectedSDK(t *testing.T) {
	goBinary, err := exec.LookPath(goExecutableName())
	if err != nil {
		t.Skip("当前环境未找到 go，可跳过")
	}

	runtimeDir := t.TempDir()
	t.Setenv("FIRE_SALAMANDER_RUNTIME_DIR", runtimeDir)

	managedDir := filepath.Join(runtimeDir, "toolchains", "go1.26.3")
	managedBinary := filepath.Join(managedDir, "bin", goExecutableName())
	if err := os.MkdirAll(filepath.Dir(managedBinary), 0755); err != nil {
		t.Fatalf("创建托管目录失败: %v", err)
	}
	if err := os.Symlink(goBinary, managedBinary); err != nil {
		t.Fatalf("创建 Go 软链失败: %v", err)
	}

	if err := SaveConfig(Config{
		SelectedBinary:       managedBinary,
		KnownBinaries:        []string{managedBinary},
		LastInstallDirectory: filepath.Join(runtimeDir, "toolchains"),
	}); err != nil {
		t.Fatalf("保存配置失败: %v", err)
	}

	state, err := DeleteManagedGoEnvironment()
	if err != nil {
		t.Fatalf("DeleteManagedGoEnvironment 返回错误: %v", err)
	}
	if _, statErr := os.Stat(managedDir); !os.IsNotExist(statErr) {
		t.Fatalf("期望托管 Go 目录被删除，得到 err=%v", statErr)
	}
	if state.Config.SelectedBinary != "" {
		t.Fatalf("删除后不应保留已删除的选中路径，得到 %q", state.Config.SelectedBinary)
	}
}

func TestDescribeGoInstallErrorReturnsClassifiedMessage(t *testing.T) {
	t.Parallel()

	err := wrapGoInstallError("安装目录不可用", os.ErrPermission)
	message, detail := DescribeGoInstallError(err)
	if message != "安装目录不可用" {
		t.Fatalf("期望错误消息为 %q，得到 %q", "安装目录不可用", message)
	}
	if detail == "" {
		t.Fatalf("期望返回底层错误详情")
	}
}

func TestDescribeGoInstallErrorHandlesCancellation(t *testing.T) {
	t.Parallel()

	message, detail := DescribeGoInstallError(context.Canceled)
	if message != "Go SDK 下载任务已停止" {
		t.Fatalf("期望取消文案为 %q，得到 %q", "Go SDK 下载任务已停止", message)
	}
	if detail != "" {
		t.Fatalf("取消场景不应返回额外错误详情，得到 %q", detail)
	}
}
