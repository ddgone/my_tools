package toolchain

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReadInstalledRustTargetsParsesUniqueValues(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("该测试依赖 sh 脚本，在 Windows 上跳过")
	}
	t.Parallel()

	script := filepath.Join(t.TempDir(), "rustup")
	content := "#!/bin/sh\nprintf 'x86_64-unknown-linux-musl\\n\\naarch64-unknown-linux-musl\\nx86_64-unknown-linux-musl\\n'\n"
	if err := os.WriteFile(script, []byte(content), 0755); err != nil {
		t.Fatalf("写入测试脚本失败: %v", err)
	}

	targets, err := readInstalledRustTargets(rustEnvironmentLayout{RustupBinary: script})
	if err != nil {
		t.Fatalf("readInstalledRustTargets 返回错误: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("期望解析出 2 个唯一 target，得到 %d: %#v", len(targets), targets)
	}
	if targets[0] != "x86_64-unknown-linux-musl" || targets[1] != "aarch64-unknown-linux-musl" {
		t.Fatalf("解析结果顺序不符合预期: %#v", targets)
	}
}

func TestInspectRustTargetsMarksInstalledAndNative(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("该测试依赖 sh 脚本，在 Windows 上跳过")
	}
	t.Parallel()

	script := filepath.Join(t.TempDir(), "rustup")
	content := "#!/bin/sh\nprintf 'x86_64-unknown-linux-musl\\n'\n"
	if err := os.WriteFile(script, []byte(content), 0755); err != nil {
		t.Fatalf("写入测试脚本失败: %v", err)
	}

	statuses, installedTargets, err := inspectRustTargets(rustEnvironmentLayout{RustupBinary: script})
	if err != nil {
		t.Fatalf("inspectRustTargets 返回错误: %v", err)
	}
	if len(installedTargets) != 1 || installedTargets[0] != "x86_64-unknown-linux-musl" {
		t.Fatalf("已安装 target 列表不符合预期: %#v", installedTargets)
	}

	foundInstalled := false
	nativeCount := 0
	for _, status := range statuses {
		if status.TargetTriple == "x86_64-unknown-linux-musl" && status.Installed {
			foundInstalled = true
		}
		if status.Native {
			nativeCount += 1
			if !status.Installed {
				t.Fatalf("原生平台应视为已可构建: %#v", status)
			}
			if status.Note == "" {
				t.Fatalf("原生平台应携带说明文案")
			}
		}
	}
	if !foundInstalled {
		t.Fatalf("期望已安装 target 被标记为 installed")
	}
	if nativeCount != 1 {
		t.Fatalf("期望恰好存在一个原生平台状态，得到 %d", nativeCount)
	}
}

func TestReadInstalledRustTargetsUsesRustEnvironmentVars(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("该测试依赖 sh 脚本，在 Windows 上跳过")
	}
	t.Parallel()

	dir := t.TempDir()
	script := filepath.Join(dir, "rustup")
	content := "#!/bin/sh\nprintf '%s\\n' \"$RUSTUP_HOME\"\n"
	if err := os.WriteFile(script, []byte(content), 0755); err != nil {
		t.Fatalf("写入测试脚本失败: %v", err)
	}

	layout := rustEnvironmentLayout{
		RootDir:      dir,
		CargoHome:    filepath.Join(dir, "cargo"),
		RustupHome:   filepath.Join(dir, "rustup"),
		RustupBinary: script,
	}
	targets, err := readInstalledRustTargets(layout)
	if err != nil {
		t.Fatalf("readInstalledRustTargets 返回错误: %v", err)
	}
	if len(targets) != 1 || targets[0] != layout.RustupHome {
		t.Fatalf("期望读取到 layout.RustupHome，得到 %#v", targets)
	}
}

func TestSummarizeRustTargetsReportsMissingTargets(t *testing.T) {
	t.Parallel()

	ok, message := summarizeRustTargets([]RustTargetStatus{
		{PlatformLabel: "Linux x64", TargetTriple: "x86_64-unknown-linux-musl", Installed: true},
		{PlatformLabel: "Windows x64", TargetTriple: "x86_64-pc-windows-gnu", Installed: false},
		{PlatformLabel: "mac Apple", TargetTriple: "aarch64-apple-darwin", Native: true, Installed: true},
	})
	if ok {
		t.Fatalf("存在缺失 target 时不应返回完整覆盖")
	}
	if message == "" {
		t.Fatalf("缺失 target 时应返回提示信息")
	}
	if !containsAll(message, []string{"1/2", "x86_64-pc-windows-gnu", "rustup target add"}) {
		t.Fatalf("提示信息未包含关键内容: %q", message)
	}
}

func containsAll(text string, fragments []string) bool {
	for _, fragment := range fragments {
		if !strings.Contains(text, fragment) {
			return false
		}
	}
	return true
}
