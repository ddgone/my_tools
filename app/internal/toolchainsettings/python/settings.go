package pythonsettings

import (
	"strings"

	"fire-salamander-desktop/internal/shared"
	"fire-salamander-desktop/internal/toolchain"
)

// ------- frontend-facing types -------

type PythonToolchainConfig struct {
	SelectedBinary string   `json:"selectedBinary"`
	KnownBinaries  []string `json:"knownBinaries"`
	Disabled       bool     `json:"disabled"`
}

type PythonToolchainCandidate struct {
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

type PythonDependencyStatus struct {
	PackageName string   `json:"packageName"`
	ModuleName  string   `json:"moduleName"`
	Installed   bool     `json:"installed"`
	Version     string   `json:"version,omitempty"`
	Error       string   `json:"error,omitempty"`
	RequiredBy  []string `json:"requiredBy"`
}

type PythonToolchainState struct {
	Config               PythonToolchainConfig      `json:"config"`
	Candidates           []PythonToolchainCandidate `json:"candidates"`
	HasUsableBaseBinary  bool                       `json:"hasUsableBaseBinary"`
	ActiveBaseBinary     string                     `json:"activeBaseBinary"`
	ActiveBaseVersion    string                     `json:"activeBaseVersion"`
	ActiveBaseSource     string                     `json:"activeBaseSource"`
	HasUsableBinary      bool                       `json:"hasUsableBinary"`
	ActiveBinary         string                     `json:"activeBinary"`
	ActiveVersion        string                     `json:"activeVersion"`
	ActiveSource         string                     `json:"activeSource"`
	PipAvailable         bool                       `json:"pipAvailable"`
	DependenciesReady    bool                       `json:"dependenciesReady"`
	MissingPackages      []string                   `json:"missingPackages"`
	StatusMessage        string                     `json:"statusMessage"`
	Dependencies         []PythonDependencyStatus   `json:"dependencies"`
	DependencyToolCount  int                        `json:"dependencyToolCount"`
	DependencyTotalCount int                        `json:"dependencyTotalCount"`
	ManagedEnvDirectory  string                     `json:"managedEnvDirectory"`
	NeedsRebuild         bool                       `json:"needsRebuild"`
	ManagedBaseBinary    string                     `json:"managedBaseBinary"`
	ManagedBaseVersion   string                     `json:"managedBaseVersion"`
}

type PythonToolchainTaskState struct {
	Kind                 string  `json:"kind"`
	Status               string  `json:"status"`
	Message              string  `json:"message"`
	Detail               string  `json:"detail,omitempty"`
	CurrentItem          string  `json:"currentItem,omitempty"`
	ProgressPercent      float64 `json:"progressPercent"`
	Step                 int     `json:"step"`
	TotalSteps           int     `json:"totalSteps"`
	BaseBinary           string  `json:"baseBinary,omitempty"`
	EnvironmentDirectory string  `json:"environmentDirectory,omitempty"`
	Error                string  `json:"error,omitempty"`
	UpdatedAt            int64   `json:"updatedAt"`
}

// ------- converters -------

func convertPythonToolchainState(state toolchain.PythonState) PythonToolchainState {
	candidates := make([]PythonToolchainCandidate, 0, len(state.Candidates))
	for _, candidate := range state.Candidates {
		candidates = append(candidates, PythonToolchainCandidate{
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
	dependencies := make([]PythonDependencyStatus, 0, len(state.Dependencies))
	for _, dep := range state.Dependencies {
		dependencies = append(dependencies, PythonDependencyStatus{
			PackageName: dep.PackageName,
			ModuleName:  dep.ModuleName,
			Installed:   dep.Installed,
			Version:     dep.Version,
			Error:       dep.Error,
			RequiredBy:  cloneStringSlice(dep.RequiredBy),
		})
	}
	return PythonToolchainState{
		Config: PythonToolchainConfig{
			SelectedBinary: state.Config.SelectedBinary,
			KnownBinaries:  cloneStringSlice(state.Config.KnownBinaries),
			Disabled:       state.Config.Disabled,
		},
		Candidates:           candidates,
		HasUsableBaseBinary:  state.HasUsableBaseBinary,
		ActiveBaseBinary:     state.ActiveBaseBinary,
		ActiveBaseVersion:    state.ActiveBaseVersion,
		ActiveBaseSource:     state.ActiveBaseSource,
		HasUsableBinary:      state.HasUsableBinary,
		ActiveBinary:         state.ActiveBinary,
		ActiveVersion:        state.ActiveVersion,
		ActiveSource:         state.ActiveSource,
		PipAvailable:         state.PipAvailable,
		DependenciesReady:    state.DependenciesReady,
		MissingPackages:      cloneStringSlice(state.MissingPackages),
		StatusMessage:        state.StatusMessage,
		Dependencies:         dependencies,
		DependencyToolCount:  state.DependencyToolCount,
		DependencyTotalCount: state.DependencyTotalCount,
		ManagedEnvDirectory:  state.ManagedEnvDirectory,
		NeedsRebuild:         state.NeedsRebuild,
		ManagedBaseBinary:    state.ManagedBaseBinary,
		ManagedBaseVersion:   state.ManagedBaseVersion,
	}
}

func convertPythonToolchainTaskState(task *shared.PythonToolchainTask) *PythonToolchainTaskState {
	if task == nil {
		return nil
	}
	return &PythonToolchainTaskState{
		Kind:                 task.Kind,
		Status:               task.Status,
		Message:              task.Message,
		Detail:               task.Detail,
		CurrentItem:          task.CurrentItem,
		ProgressPercent:      task.ProgressPercent,
		Step:                 task.Step,
		TotalSteps:           task.TotalSteps,
		BaseBinary:           task.BaseBinary,
		EnvironmentDirectory: task.EnvironmentDirectory,
		Error:                task.Error,
		UpdatedAt:            task.UpdatedAt,
	}
}

func cloneStringSlice(values []string) []string {
	return append([]string{}, values...)
}

// ------- Manager -------

type Manager struct {
	state         *shared.SharedState
	ensureTooling func(*shared.SharedState) error
}

func NewManager(state *shared.SharedState, ensureTooling func(*shared.SharedState) error) *Manager {
	return &Manager{
		state:         state,
		ensureTooling: ensureTooling,
	}
}

// ------- public API -------

func (m *Manager) GetPythonToolchainState() (*PythonToolchainState, error) {
	state, err := toolchain.GetPythonState()
	if err != nil {
		return nil, err
	}
	result := convertPythonToolchainState(state)
	return &result, nil
}

func (m *Manager) SavePythonToolchainConfig(cfg PythonToolchainConfig) (*PythonToolchainState, error) {
	normalized := toolchain.PythonConfig{
		SelectedBinary: strings.TrimSpace(cfg.SelectedBinary),
		KnownBinaries:  cloneStringSlice(cfg.KnownBinaries),
		Disabled:       cfg.Disabled,
	}
	if err := toolchain.SavePythonConfig(normalized); err != nil {
		return nil, err
	}
	state, err := toolchain.GetPythonState()
	if err != nil {
		return nil, err
	}
	result := convertPythonToolchainState(state)
	return &result, nil
}

func (m *Manager) InstallPythonDependencies() (*PythonToolchainState, error) {
	state, err := toolchain.InstallPythonDependencies()
	if err != nil {
		return nil, err
	}
	result := convertPythonToolchainState(state)
	return &result, nil
}

func (m *Manager) PreparePythonToolchainEnvironment() (*PythonToolchainState, error) {
	state, err := toolchain.PrepareManagedPythonEnvironment()
	if err != nil {
		return nil, err
	}
	result := convertPythonToolchainState(state)
	return &result, nil
}

func (m *Manager) CheckPythonToolchainEnvironment() (*PythonToolchainState, error) {
	state, err := toolchain.CheckManagedPythonEnvironment()
	if err != nil {
		result := convertPythonToolchainState(state)
		return &result, err
	}
	result := convertPythonToolchainState(state)
	return &result, nil
}

func (m *Manager) DeletePythonToolchainEnvironment() (*PythonToolchainState, error) {
	state, err := toolchain.DeleteManagedPythonEnvironment()
	if err != nil {
		return nil, err
	}
	result := convertPythonToolchainState(state)
	return &result, nil
}

func (m *Manager) GetPythonToolchainTaskState() *PythonToolchainTaskState {
	return convertPythonToolchainTaskState(m.getPythonToolchainTaskState())
}

func (m *Manager) StartPreparePythonToolchainEnvironment() (*PythonToolchainTaskState, error) {
	task, err := m.startPreparePythonToolchainTask()
	if err != nil {
		return nil, err
	}
	return convertPythonToolchainTaskState(task), nil
}

func (m *Manager) StartInstallPythonDependencies() (*PythonToolchainTaskState, error) {
	task, err := m.startInstallPythonDependenciesTask()
	if err != nil {
		return nil, err
	}
	return convertPythonToolchainTaskState(task), nil
}

func (m *Manager) CancelActivePythonToolchainTask() error {
	return m.cancelPythonToolchainTask()
}
