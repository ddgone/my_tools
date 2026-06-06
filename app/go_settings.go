package main

import (
	"strings"

	"fire-salamander-desktop/internal/toolchain"
)

type GoToolchainConfig struct {
	SelectedBinary       string   `json:"selectedBinary"`
	KnownBinaries        []string `json:"knownBinaries"`
	LastInstallDirectory string   `json:"lastInstallDirectory"`
	Disabled             bool     `json:"disabled"`
}

type GoToolchainCandidate struct {
	Path     string `json:"path"`
	Version  string `json:"version"`
	Source   string `json:"source"`
	Label    string `json:"label"`
	Detail   string `json:"detail"`
	Error    string `json:"error,omitempty"`
	Valid    bool   `json:"valid"`
	Selected bool   `json:"selected"`
	Active   bool   `json:"active"`
}

type GoToolchainState struct {
	Config                    GoToolchainConfig      `json:"config"`
	Candidates                []GoToolchainCandidate `json:"candidates"`
	HasUsableBinary           bool                   `json:"hasUsableBinary"`
	ActiveBinary              string                 `json:"activeBinary"`
	ActiveVersion             string                 `json:"activeVersion"`
	ActiveSource              string                 `json:"activeSource"`
	StatusMessage             string                 `json:"statusMessage"`
	SuggestedInstallDirectory string                 `json:"suggestedInstallDirectory"`
}

type GoOfficialRelease struct {
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
}

type InstallGoToolchainRequest struct {
	Version   string `json:"version"`
	Directory string `json:"directory"`
}

func (a *App) GetGoToolchainState() (*GoToolchainState, error) {
	state, err := toolchain.GetState()
	if err != nil {
		return nil, err
	}
	result := convertGoToolchainState(state)
	return &result, nil
}

func (a *App) SaveGoToolchainConfig(cfg GoToolchainConfig) (*GoToolchainState, error) {
	normalized := toolchain.Config{
		SelectedBinary:       strings.TrimSpace(cfg.SelectedBinary),
		KnownBinaries:        cloneStringSlice(cfg.KnownBinaries),
		LastInstallDirectory: strings.TrimSpace(cfg.LastInstallDirectory),
		Disabled:             cfg.Disabled,
	}
	if err := toolchain.SaveConfig(normalized); err != nil {
		return nil, err
	}
	state, err := toolchain.GetState()
	if err != nil {
		return nil, err
	}
	result := convertGoToolchainState(state)
	return &result, nil
}

func (a *App) ListOfficialGoReleases() ([]GoOfficialRelease, error) {
	releases, err := toolchain.ListOfficialReleases()
	if err != nil {
		return nil, err
	}
	result := make([]GoOfficialRelease, 0, len(releases))
	for _, release := range releases {
		result = append(result, GoOfficialRelease{
			Version: release.Version,
			Stable:  release.Stable,
		})
	}
	return result, nil
}

func (a *App) InstallGoToolchain(req InstallGoToolchainRequest) (*GoToolchainState, error) {
	installResult, err := toolchain.InstallOfficialRelease(req.Version, req.Directory)
	if err != nil {
		return nil, err
	}
	cfg, err := toolchain.LoadConfig()
	if err != nil {
		return nil, err
	}
	cfg.SelectedBinary = installResult.BinaryPath
	cfg.KnownBinaries = append(cfg.KnownBinaries, installResult.BinaryPath)
	cfg.LastInstallDirectory = installResult.Directory
	cfg.Disabled = false
	if saveErr := toolchain.SaveConfig(cfg); saveErr != nil {
		return nil, saveErr
	}
	state, err := toolchain.GetState()
	if err != nil {
		return nil, err
	}
	converted := convertGoToolchainState(state)
	return &converted, nil
}

func convertGoToolchainState(state toolchain.State) GoToolchainState {
	candidates := make([]GoToolchainCandidate, 0, len(state.Candidates))
	for _, candidate := range state.Candidates {
		candidates = append(candidates, GoToolchainCandidate{
			Path:     candidate.Path,
			Version:  candidate.Version,
			Source:   candidate.Source,
			Label:    candidate.Label,
			Detail:   candidate.Detail,
			Error:    candidate.Error,
			Valid:    candidate.Valid,
			Selected: candidate.Selected,
			Active:   candidate.Active,
		})
	}
	return GoToolchainState{
		Config: GoToolchainConfig{
			SelectedBinary:       state.Config.SelectedBinary,
			KnownBinaries:        cloneStringSlice(state.Config.KnownBinaries),
			LastInstallDirectory: state.Config.LastInstallDirectory,
			Disabled:             state.Config.Disabled,
		},
		Candidates:                candidates,
		HasUsableBinary:           state.HasUsableBinary,
		ActiveBinary:              state.ActiveBinary,
		ActiveVersion:             state.ActiveVersion,
		ActiveSource:              state.ActiveSource,
		StatusMessage:             state.StatusMessage,
		SuggestedInstallDirectory: state.SuggestedInstallDirectory,
	}
}
