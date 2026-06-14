package main

import (
	"strings"

	"fire-salamander-desktop/internal/toolchain"
)

type RustToolchainConfig struct {
	SelectedCargoBinary         string   `json:"selectedCargoBinary"`
	KnownCargoBinaries          []string `json:"knownCargoBinaries"`
	SelectedRustupBinary        string   `json:"selectedRustupBinary"`
	KnownRustupBinaries         []string `json:"knownRustupBinaries"`
	SelectedZigBinary           string   `json:"selectedZigBinary"`
	KnownZigBinaries            []string `json:"knownZigBinaries"`
	SelectedCargoZigbuildBinary string   `json:"selectedCargoZigbuildBinary"`
	KnownCargoZigbuildBinaries  []string `json:"knownCargoZigbuildBinaries"`
}

type RustToolchainCandidate struct {
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

type RustToolchainTargetStatus struct {
	PlatformKey   string `json:"platformKey"`
	PlatformLabel string `json:"platformLabel"`
	TargetTriple  string `json:"targetTriple"`
	Installed     bool   `json:"installed"`
	Native        bool   `json:"native"`
	Note          string `json:"note,omitempty"`
}

type RustToolchainState struct {
	Config                     RustToolchainConfig         `json:"config"`
	CargoCandidates            []RustToolchainCandidate    `json:"cargoCandidates"`
	RustupCandidates           []RustToolchainCandidate    `json:"rustupCandidates"`
	ZigCandidates              []RustToolchainCandidate    `json:"zigCandidates"`
	CargoZigbuildCandidates    []RustToolchainCandidate    `json:"cargoZigbuildCandidates"`
	InstalledTargets           []string                    `json:"installedTargets"`
	TargetStatuses             []RustToolchainTargetStatus `json:"targetStatuses"`
	HasInstalledTargetInfo     bool                        `json:"hasInstalledTargetInfo"`
	HasFullTargetCoverage      bool                        `json:"hasFullTargetCoverage"`
	TargetStatusMessage        string                      `json:"targetStatusMessage"`
	HasUsableEnvironment       bool                        `json:"hasUsableEnvironment"`
	HasUsableCargo             bool                        `json:"hasUsableCargo"`
	HasUsableRustup            bool                        `json:"hasUsableRustup"`
	HasUsableZig               bool                        `json:"hasUsableZig"`
	HasUsableCargoZigbuild     bool                        `json:"hasUsableCargoZigbuild"`
	ActiveCargoBinary          string                      `json:"activeCargoBinary"`
	ActiveCargoVersion         string                      `json:"activeCargoVersion"`
	ActiveCargoSource          string                      `json:"activeCargoSource"`
	ActiveRustupBinary         string                      `json:"activeRustupBinary"`
	ActiveRustupVersion        string                      `json:"activeRustupVersion"`
	ActiveRustupSource         string                      `json:"activeRustupSource"`
	ActiveZigBinary            string                      `json:"activeZigBinary"`
	ActiveZigVersion           string                      `json:"activeZigVersion"`
	ActiveZigSource            string                      `json:"activeZigSource"`
	ActiveCargoZigbuildBinary  string                      `json:"activeCargoZigbuildBinary"`
	ActiveCargoZigbuildVersion string                      `json:"activeCargoZigbuildVersion"`
	ActiveCargoZigbuildSource  string                      `json:"activeCargoZigbuildSource"`
	StatusMessage              string                      `json:"statusMessage"`
}

func (a *App) GetRustToolchainState() (*RustToolchainState, error) {
	state, err := toolchain.GetRustState()
	if err != nil {
		return nil, err
	}
	result := convertRustToolchainState(state)
	return &result, nil
}

func (a *App) CheckRustToolchainEnvironment() (*RustToolchainState, error) {
	state, err := toolchain.GetRustState()
	if err != nil {
		return nil, err
	}
	result := convertRustToolchainState(state)
	return &result, nil
}

func (a *App) SaveRustToolchainConfig(cfg RustToolchainConfig) (*RustToolchainState, error) {
	normalized := toolchain.RustConfig{
		SelectedCargoBinary:         strings.TrimSpace(cfg.SelectedCargoBinary),
		KnownCargoBinaries:          cloneStringSlice(cfg.KnownCargoBinaries),
		SelectedRustupBinary:        strings.TrimSpace(cfg.SelectedRustupBinary),
		KnownRustupBinaries:         cloneStringSlice(cfg.KnownRustupBinaries),
		SelectedZigBinary:           strings.TrimSpace(cfg.SelectedZigBinary),
		KnownZigBinaries:            cloneStringSlice(cfg.KnownZigBinaries),
		SelectedCargoZigbuildBinary: strings.TrimSpace(cfg.SelectedCargoZigbuildBinary),
		KnownCargoZigbuildBinaries:  cloneStringSlice(cfg.KnownCargoZigbuildBinaries),
	}
	if err := toolchain.SaveRustConfig(normalized); err != nil {
		return nil, err
	}
	state, err := toolchain.GetRustState()
	if err != nil {
		return nil, err
	}
	result := convertRustToolchainState(state)
	return &result, nil
}

func convertRustToolchainState(state toolchain.RustToolchainState) RustToolchainState {
	return RustToolchainState{
		Config: RustToolchainConfig{
			SelectedCargoBinary:         state.Config.SelectedCargoBinary,
			KnownCargoBinaries:          cloneStringSlice(state.Config.KnownCargoBinaries),
			SelectedRustupBinary:        state.Config.SelectedRustupBinary,
			KnownRustupBinaries:         cloneStringSlice(state.Config.KnownRustupBinaries),
			SelectedZigBinary:           state.Config.SelectedZigBinary,
			KnownZigBinaries:            cloneStringSlice(state.Config.KnownZigBinaries),
			SelectedCargoZigbuildBinary: state.Config.SelectedCargoZigbuildBinary,
			KnownCargoZigbuildBinaries:  cloneStringSlice(state.Config.KnownCargoZigbuildBinaries),
		},
		CargoCandidates:            convertRustCandidates(state.CargoCandidates),
		RustupCandidates:           convertRustCandidates(state.RustupCandidates),
		ZigCandidates:              convertRustCandidates(state.ZigCandidates),
		CargoZigbuildCandidates:    convertRustCandidates(state.CargoZigbuildCandidates),
		InstalledTargets:           cloneStringSlice(state.InstalledTargets),
		TargetStatuses:             convertRustTargetStatuses(state.TargetStatuses),
		HasInstalledTargetInfo:     state.HasInstalledTargetInfo,
		HasFullTargetCoverage:      state.HasFullTargetCoverage,
		TargetStatusMessage:        state.TargetStatusMessage,
		HasUsableEnvironment:       state.HasUsableEnvironment,
		HasUsableCargo:             state.HasUsableCargo,
		HasUsableRustup:            state.HasUsableRustup,
		HasUsableZig:               state.HasUsableZig,
		HasUsableCargoZigbuild:     state.HasUsableCargoZigbuild,
		ActiveCargoBinary:          state.ActiveCargoBinary,
		ActiveCargoVersion:         state.ActiveCargoVersion,
		ActiveCargoSource:          state.ActiveCargoSource,
		ActiveRustupBinary:         state.ActiveRustupBinary,
		ActiveRustupVersion:        state.ActiveRustupVersion,
		ActiveRustupSource:         state.ActiveRustupSource,
		ActiveZigBinary:            state.ActiveZigBinary,
		ActiveZigVersion:           state.ActiveZigVersion,
		ActiveZigSource:            state.ActiveZigSource,
		ActiveCargoZigbuildBinary:  state.ActiveCargoZigbuildBinary,
		ActiveCargoZigbuildVersion: state.ActiveCargoZigbuildVersion,
		ActiveCargoZigbuildSource:  state.ActiveCargoZigbuildSource,
		StatusMessage:              state.StatusMessage,
	}
}

func convertRustCandidates(input []toolchain.RustCandidate) []RustToolchainCandidate {
	result := make([]RustToolchainCandidate, 0, len(input))
	for _, candidate := range input {
		result = append(result, RustToolchainCandidate{
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
	return result
}

func convertRustTargetStatuses(input []toolchain.RustTargetStatus) []RustToolchainTargetStatus {
	result := make([]RustToolchainTargetStatus, 0, len(input))
	for _, status := range input {
		result = append(result, RustToolchainTargetStatus{
			PlatformKey:   status.PlatformKey,
			PlatformLabel: status.PlatformLabel,
			TargetTriple:  status.TargetTriple,
			Installed:     status.Installed,
			Native:        status.Native,
			Note:          status.Note,
		})
	}
	return result
}
