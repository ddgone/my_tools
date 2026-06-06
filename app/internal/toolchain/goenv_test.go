package toolchain

import (
	"os/exec"
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
