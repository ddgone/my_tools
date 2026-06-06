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
	RuntimeDetails            GoRuntimeDetails       `json:"runtimeDetails"`
	StatusMessage             string                 `json:"statusMessage"`
	SuggestedInstallDirectory string                 `json:"suggestedInstallDirectory"`
}

type GoRuntimeDetails struct {
	GOROOT    string `json:"goroot"`
	GOPATH    string `json:"gopath"`
	GOOS      string `json:"goos"`
	GOARCH    string `json:"goarch"`
	GOVERSION string `json:"goversion"`
}

type GoOfficialRelease struct {
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
}

type GoToolchainTaskState struct {
	Kind             string  `json:"kind"`
	Status           string  `json:"status"`
	Message          string  `json:"message"`
	Detail           string  `json:"detail,omitempty"`
	CurrentItem      string  `json:"currentItem,omitempty"`
	ProgressPercent  float64 `json:"progressPercent"`
	Step             int     `json:"step"`
	TotalSteps       int     `json:"totalSteps"`
	Version          string  `json:"version,omitempty"`
	Directory        string  `json:"directory,omitempty"`
	TransferredBytes int64   `json:"transferredBytes,omitempty"`
	TotalBytes       int64   `json:"totalBytes,omitempty"`
	TransferSpeed    string  `json:"transferSpeed,omitempty"`
	Error            string  `json:"error,omitempty"`
	UpdatedAt        int64   `json:"updatedAt"`
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

func (a *App) CheckGoToolchainEnvironment() (*GoToolchainState, error) {
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
	cfg.LastInstallDirectory = toolchain.NormalizeInstallBaseDirectory(req.Directory)
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

func (a *App) DeleteGoToolchainEnvironment() (*GoToolchainState, error) {
	state, err := toolchain.DeleteManagedGoEnvironment()
	if err != nil {
		return nil, err
	}
	result := convertGoToolchainState(state)
	return &result, nil
}

func (a *App) GetGoToolchainTaskState() *GoToolchainTaskState {
	return convertGoToolchainTaskState(a.getGoToolchainTaskState())
}

func (a *App) StartInstallGoToolchain(req InstallGoToolchainRequest) (*GoToolchainTaskState, error) {
	task, err := a.startInstallGoToolchainTask(req)
	if err != nil {
		return nil, err
	}
	return convertGoToolchainTaskState(task), nil
}

func (a *App) CancelActiveGoToolchainTask() error {
	return a.cancelGoToolchainTask()
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
		Candidates:      candidates,
		HasUsableBinary: state.HasUsableBinary,
		ActiveBinary:    state.ActiveBinary,
		ActiveVersion:   state.ActiveVersion,
		ActiveSource:    state.ActiveSource,
		RuntimeDetails: GoRuntimeDetails{
			GOROOT:    state.RuntimeDetails.GOROOT,
			GOPATH:    state.RuntimeDetails.GOPATH,
			GOOS:      state.RuntimeDetails.GOOS,
			GOARCH:    state.RuntimeDetails.GOARCH,
			GOVERSION: state.RuntimeDetails.GOVERSION,
		},
		StatusMessage:             state.StatusMessage,
		SuggestedInstallDirectory: state.SuggestedInstallDirectory,
	}
}

func convertGoToolchainTaskState(task *GoToolchainTask) *GoToolchainTaskState {
	if task == nil {
		return nil
	}
	return &GoToolchainTaskState{
		Kind:             task.Kind,
		Status:           task.Status,
		Message:          task.Message,
		Detail:           task.Detail,
		CurrentItem:      task.CurrentItem,
		ProgressPercent:  task.ProgressPercent,
		Step:             task.Step,
		TotalSteps:       task.TotalSteps,
		Version:          task.Version,
		Directory:        task.Directory,
		TransferredBytes: task.TransferredBytes,
		TotalBytes:       task.TotalBytes,
		TransferSpeed:    task.TransferSpeed,
		Error:            task.Error,
		UpdatedAt:        task.UpdatedAt,
	}
}
