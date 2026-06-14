package toolchain

import (
	"path/filepath"
	"testing"
)

func TestRustupInitExecutableName(t *testing.T) {
	name := rustupInitExecutableName()
	if name != "rustup-init"+rustupInitFileExtension() {
		t.Fatalf("unexpected rustup-init executable name: %s", name)
	}
	if name != rustToolExecutableName("rustup-init") {
		t.Fatalf("rustup-init executable name should match tool executable name, got %s", name)
	}
}

func TestManagedRustEnvironmentVarsUsesManagedHomesBeforeInstall(t *testing.T) {
	root := filepath.Join("/tmp", "fire-salamander", "rust", "stable")
	env := managedRustEnvironmentVars(root, false)
	envMap := make(map[string]string, len(env))
	for _, item := range env {
		for i := 0; i < len(item); i++ {
			if item[i] == '=' {
				envMap[item[:i]] = item[i+1:]
				break
			}
		}
	}

	wantCargoHome := filepath.Join(root, "cargo")
	wantRustupHome := filepath.Join(root, "rustup")
	if envMap["CARGO_HOME"] != wantCargoHome {
		t.Fatalf("expected CARGO_HOME=%s, got %s", wantCargoHome, envMap["CARGO_HOME"])
	}
	if envMap["RUSTUP_HOME"] != wantRustupHome {
		t.Fatalf("expected RUSTUP_HOME=%s, got %s", wantRustupHome, envMap["RUSTUP_HOME"])
	}
	if envMap["RUSTUP_INIT_SKIP_PATH_CHECK"] != "yes" {
		t.Fatalf("expected RUSTUP_INIT_SKIP_PATH_CHECK=yes, got %s", envMap["RUSTUP_INIT_SKIP_PATH_CHECK"])
	}
}
