package main

import (
	"testing"

	"fire-salamander-desktop/internal/toolchain"
)

func TestConvertPythonToolchainStateKeepsEmptySlicesNonNil(t *testing.T) {
	state := convertPythonToolchainState(toolchain.PythonState{
		Config: toolchain.PythonConfig{},
	})

	if state.Config.KnownBinaries == nil {
		t.Fatalf("expected knownBinaries to be an empty slice, got nil")
	}
	if state.MissingPackages == nil {
		t.Fatalf("expected missingPackages to be an empty slice, got nil")
	}
}

func TestConvertGoToolchainStateKeepsEmptySlicesNonNil(t *testing.T) {
	state := convertGoToolchainState(toolchain.State{
		Config: toolchain.Config{},
	})

	if state.Config.KnownBinaries == nil {
		t.Fatalf("expected knownBinaries to be an empty slice, got nil")
	}
}
