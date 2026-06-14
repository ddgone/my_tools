package main

import (
	"strings"

	"fire-salamander-desktop/internal/toolchain"
)

type RustToolchainConfig struct {
	Mode                 string   `json:"mode"`
	SelectedRustRoot     string   `json:"selectedRustRoot"`
	KnownRustRoots       []string `json:"knownRustRoots"`
	SelectedZigBinary    string   `json:"selectedZigBinary"`
	KnownZigBinaries     []string `json:"knownZigBinaries"`
	LastInstallDirectory string   `json:"lastInstallDirectory"`
	Disabled             bool     `json:"disabled"`
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
	RustCandidates             []RustToolchainEnvironment  `json:"rustCandidates"`
	ZigCandidates              []RustToolchainCandidate    `json:"zigCandidates"`
	InstalledTargets           []string                    `json:"installedTargets"`
	TargetStatuses             []RustToolchainTargetStatus `json:"targetStatuses"`
	HasInstalledTargetInfo     bool                        `json:"hasInstalledTargetInfo"`
	HasFullTargetCoverage      bool                        `json:"hasFullTargetCoverage"`
	TargetStatusMessage        string                      `json:"targetStatusMessage"`
	CargoZigbuildStatusMessage string                      `json:"cargoZigbuildStatusMessage"`
	HasUsableEnvironment       bool                        `json:"hasUsableEnvironment"`
	HasUsableRust              bool                        `json:"hasUsableRust"`
	HasUsableCargo             bool                        `json:"hasUsableCargo"`
	HasUsableRustup            bool                        `json:"hasUsableRustup"`
	HasUsableZig               bool                        `json:"hasUsableZig"`
	HasUsableCargoZigbuild     bool                        `json:"hasUsableCargoZigbuild"`
	CanManageTargets           bool                        `json:"canManageTargets"`
	CanManageCargoZigbuild     bool                        `json:"canManageCargoZigbuild"`
	ActiveRustRoot             string                      `json:"activeRustRoot"`
	ActiveRustVersion          string                      `json:"activeRustVersion"`
	ActiveRustSource           string                      `json:"activeRustSource"`
	ActiveRustManaged          bool                        `json:"activeRustManaged"`
	ActiveCargoBinary          string                      `json:"activeCargoBinary"`
	ActiveCargoVersion         string                      `json:"activeCargoVersion"`
	ActiveCargoSource          string                      `json:"activeCargoSource"`
	ActiveRustupBinary         string                      `json:"activeRustupBinary"`
	ActiveRustupVersion        string                      `json:"activeRustupVersion"`
	ActiveRustupSource         string                      `json:"activeRustupSource"`
	ActiveRustcBinary          string                      `json:"activeRustcBinary"`
	ActiveZigBinary            string                      `json:"activeZigBinary"`
	ActiveZigVersion           string                      `json:"activeZigVersion"`
	ActiveZigSource            string                      `json:"activeZigSource"`
	ActiveCargoZigbuildBinary  string                      `json:"activeCargoZigbuildBinary"`
	ActiveCargoZigbuildVersion string                      `json:"activeCargoZigbuildVersion"`
	ActiveCargoZigbuildSource  string                      `json:"activeCargoZigbuildSource"`
	StatusMessage              string                      `json:"statusMessage"`
	SuggestedInstallDirectory  string                      `json:"suggestedInstallDirectory"`
}

type RustToolchainEnvironment struct {
	RootDir             string `json:"rootDir"`
	Version             string `json:"version"`
	Source              string `json:"source"`
	Label               string `json:"label"`
	Detail              string `json:"detail"`
	Error               string `json:"error,omitempty"`
	Valid               bool   `json:"valid"`
	Selected            bool   `json:"selected"`
	Active              bool   `json:"active"`
	CargoBinary         string `json:"cargoBinary,omitempty"`
	RustupBinary        string `json:"rustupBinary,omitempty"`
	RustcBinary         string `json:"rustcBinary,omitempty"`
	CargoZigbuildBinary string `json:"cargoZigbuildBinary,omitempty"`
	HasRustup           bool   `json:"hasRustup"`
	HasCargoZigbuild    bool   `json:"hasCargoZigbuild"`
	CanManageTargets    bool   `json:"canManageTargets"`
	Managed             bool   `json:"managed"`
}

type RustOfficialRelease struct {
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
	Channel bool   `json:"channel"`
}

type ZigOfficialRelease struct {
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
	Date    string `json:"date,omitempty"`
}

type RustToolchainTaskState struct {
	Kind             string  `json:"kind"`
	Status           string  `json:"status"`
	Message          string  `json:"message"`
	Detail           string  `json:"detail,omitempty"`
	CurrentItem      string  `json:"currentItem,omitempty"`
	CurrentSource    string  `json:"currentSource,omitempty"`
	ProgressPercent  float64 `json:"progressPercent"`
	Step             int     `json:"step"`
	TotalSteps       int     `json:"totalSteps"`
	RustVersion      string  `json:"rustVersion,omitempty"`
	ZigVersion       string  `json:"zigVersion,omitempty"`
	Directory        string  `json:"directory,omitempty"`
	TransferredBytes int64   `json:"transferredBytes,omitempty"`
	TotalBytes       int64   `json:"totalBytes,omitempty"`
	TransferSpeed    string  `json:"transferSpeed,omitempty"`
	Error            string  `json:"error,omitempty"`
	UpdatedAt        int64   `json:"updatedAt"`
}

type InstallRustToolchainRequest struct {
	RustVersion string `json:"rustVersion"`
	ZigVersion  string `json:"zigVersion"`
	Directory   string `json:"directory"`
}

func (a *App) StartInstallRustCargoZigbuild() (*RustToolchainTaskState, error) {
	task, err := a.startInstallRustCapabilityTask("cargo-zigbuild")
	if err != nil {
		return nil, err
	}
	return convertRustToolchainTaskState(task), nil
}

func (a *App) StartInstallRustTargets() (*RustToolchainTaskState, error) {
	task, err := a.startInstallRustCapabilityTask("targets")
	if err != nil {
		return nil, err
	}
	return convertRustToolchainTaskState(task), nil
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
		Mode:                 strings.TrimSpace(cfg.Mode),
		SelectedRustRoot:     strings.TrimSpace(cfg.SelectedRustRoot),
		KnownRustRoots:       cloneStringSlice(cfg.KnownRustRoots),
		SelectedZigBinary:    strings.TrimSpace(cfg.SelectedZigBinary),
		KnownZigBinaries:     cloneStringSlice(cfg.KnownZigBinaries),
		LastInstallDirectory: strings.TrimSpace(cfg.LastInstallDirectory),
		Disabled:             cfg.Disabled,
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

func (a *App) ListOfficialRustReleases() ([]RustOfficialRelease, error) {
	releases, err := toolchain.ListOfficialRustReleases()
	if err != nil {
		return nil, err
	}
	result := make([]RustOfficialRelease, 0, len(releases))
	for _, release := range releases {
		result = append(result, RustOfficialRelease{
			Version: release.Version,
			Stable:  release.Stable,
			Channel: release.Channel,
		})
	}
	return result, nil
}

func (a *App) ListOfficialZigReleases() ([]ZigOfficialRelease, error) {
	releases, err := toolchain.ListOfficialZigReleases()
	if err != nil {
		return nil, err
	}
	result := make([]ZigOfficialRelease, 0, len(releases))
	for _, release := range releases {
		result = append(result, ZigOfficialRelease{
			Version: release.Version,
			Stable:  release.Stable,
			Date:    release.Date,
		})
	}
	return result, nil
}

func (a *App) DeleteManagedRustToolchainEnvironment() (*RustToolchainState, error) {
	state, err := toolchain.DeleteManagedRustEnvironment()
	if err != nil {
		return nil, err
	}
	result := convertRustToolchainState(state)
	return &result, nil
}

func (a *App) GetRustToolchainTaskState() *RustToolchainTaskState {
	return convertRustToolchainTaskState(a.getRustToolchainTaskState())
}

func (a *App) StartInstallRustToolchain(req InstallRustToolchainRequest) (*RustToolchainTaskState, error) {
	task, err := a.startInstallRustToolchainTask(req)
	if err != nil {
		return nil, err
	}
	return convertRustToolchainTaskState(task), nil
}

func (a *App) CancelActiveRustToolchainTask() error {
	return a.cancelRustToolchainTask()
}

func convertRustToolchainState(state toolchain.RustToolchainState) RustToolchainState {
	return RustToolchainState{
		Config: RustToolchainConfig{
			Mode:                 state.Config.Mode,
			SelectedRustRoot:     state.Config.SelectedRustRoot,
			KnownRustRoots:       cloneStringSlice(state.Config.KnownRustRoots),
			SelectedZigBinary:    state.Config.SelectedZigBinary,
			KnownZigBinaries:     cloneStringSlice(state.Config.KnownZigBinaries),
			LastInstallDirectory: state.Config.LastInstallDirectory,
			Disabled:             state.Config.Disabled,
		},
		RustCandidates:             convertRustEnvironmentCandidates(state.RustCandidates),
		ZigCandidates:              convertRustCandidates(state.ZigCandidates),
		InstalledTargets:           cloneStringSlice(state.InstalledTargets),
		TargetStatuses:             convertRustTargetStatuses(state.TargetStatuses),
		HasInstalledTargetInfo:     state.HasInstalledTargetInfo,
		HasFullTargetCoverage:      state.HasFullTargetCoverage,
		TargetStatusMessage:        state.TargetStatusMessage,
		CargoZigbuildStatusMessage: state.CargoZigbuildStatusMessage,
		HasUsableEnvironment:       state.HasUsableEnvironment,
		HasUsableRust:              state.HasUsableRust,
		HasUsableCargo:             state.HasUsableCargo,
		HasUsableRustup:            state.HasUsableRustup,
		HasUsableZig:               state.HasUsableZig,
		HasUsableCargoZigbuild:     state.HasUsableCargoZigbuild,
		CanManageTargets:           state.CanManageTargets,
		CanManageCargoZigbuild:     state.CanManageCargoZigbuild,
		ActiveRustRoot:             state.ActiveRustRoot,
		ActiveRustVersion:          state.ActiveRustVersion,
		ActiveRustSource:           state.ActiveRustSource,
		ActiveRustManaged:          state.ActiveRustManaged,
		ActiveCargoBinary:          state.ActiveCargoBinary,
		ActiveCargoVersion:         state.ActiveCargoVersion,
		ActiveCargoSource:          state.ActiveCargoSource,
		ActiveRustupBinary:         state.ActiveRustupBinary,
		ActiveRustupVersion:        state.ActiveRustupVersion,
		ActiveRustupSource:         state.ActiveRustupSource,
		ActiveRustcBinary:          state.ActiveRustcBinary,
		ActiveZigBinary:            state.ActiveZigBinary,
		ActiveZigVersion:           state.ActiveZigVersion,
		ActiveZigSource:            state.ActiveZigSource,
		ActiveCargoZigbuildBinary:  state.ActiveCargoZigbuildBinary,
		ActiveCargoZigbuildVersion: state.ActiveCargoZigbuildVersion,
		ActiveCargoZigbuildSource:  state.ActiveCargoZigbuildSource,
		StatusMessage:              state.StatusMessage,
		SuggestedInstallDirectory:  state.SuggestedInstallDirectory,
	}
}

func convertRustEnvironmentCandidates(input []toolchain.RustEnvironmentCandidate) []RustToolchainEnvironment {
	result := make([]RustToolchainEnvironment, 0, len(input))
	for _, candidate := range input {
		result = append(result, RustToolchainEnvironment{
			RootDir:             candidate.RootDir,
			Version:             candidate.Version,
			Source:              candidate.Source,
			Label:               candidate.Label,
			Detail:              candidate.Detail,
			Error:               candidate.Error,
			Valid:               candidate.Valid,
			Selected:            candidate.Selected,
			Active:              candidate.Active,
			CargoBinary:         candidate.CargoBinary,
			RustupBinary:        candidate.RustupBinary,
			RustcBinary:         candidate.RustcBinary,
			CargoZigbuildBinary: candidate.CargoZigbuildBinary,
			HasRustup:           candidate.HasRustup,
			HasCargoZigbuild:    candidate.HasCargoZigbuild,
			CanManageTargets:    candidate.CanManageTargets,
			Managed:             candidate.Managed,
		})
	}
	return result
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

func convertRustToolchainTaskState(task *RustToolchainTask) *RustToolchainTaskState {
	if task == nil {
		return nil
	}
	return &RustToolchainTaskState{
		Kind:             task.Kind,
		Status:           task.Status,
		Message:          task.Message,
		Detail:           task.Detail,
		CurrentItem:      task.CurrentItem,
		CurrentSource:    task.CurrentSource,
		ProgressPercent:  task.ProgressPercent,
		Step:             task.Step,
		TotalSteps:       task.TotalSteps,
		RustVersion:      task.RustVersion,
		ZigVersion:       task.ZigVersion,
		Directory:        task.Directory,
		TransferredBytes: task.TransferredBytes,
		TotalBytes:       task.TotalBytes,
		TransferSpeed:    task.TransferSpeed,
		Error:            task.Error,
		UpdatedAt:        task.UpdatedAt,
	}
}
