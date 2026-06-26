package main

// Go toolchain

func (a *App) GetGoToolchainState() (*GoToolchainState, error) {
	return a.goSettings.GetGoToolchainState()
}

func (a *App) CheckGoToolchainEnvironment() (*GoToolchainState, error) {
	return a.goSettings.CheckGoToolchainEnvironment()
}

func (a *App) SaveGoToolchainConfig(cfg GoToolchainConfig) (*GoToolchainState, error) {
	return a.goSettings.SaveGoToolchainConfig(cfg)
}

func (a *App) ListOfficialGoReleases() ([]GoOfficialRelease, error) {
	return a.goSettings.ListOfficialGoReleases()
}

func (a *App) InstallGoToolchain(req InstallGoToolchainRequest) (*GoToolchainState, error) {
	return a.goSettings.InstallGoToolchain(req)
}

func (a *App) DeleteGoToolchainEnvironment() (*GoToolchainState, error) {
	return a.goSettings.DeleteGoToolchainEnvironment()
}

func (a *App) GetGoToolchainTaskState() *GoToolchainTaskState {
	return a.goSettings.GetGoToolchainTaskState()
}

func (a *App) StartInstallGoToolchain(req InstallGoToolchainRequest) (*GoToolchainTaskState, error) {
	return a.goSettings.StartInstallGoToolchain(req)
}

func (a *App) CancelActiveGoToolchainTask() error {
	return a.goSettings.CancelActiveGoToolchainTask()
}

// Python toolchain

func (a *App) GetPythonToolchainState() (*PythonToolchainState, error) {
	return a.pythonSettings.GetPythonToolchainState()
}

func (a *App) SavePythonToolchainConfig(cfg PythonToolchainConfig) (*PythonToolchainState, error) {
	return a.pythonSettings.SavePythonToolchainConfig(cfg)
}

func (a *App) InstallPythonDependencies() (*PythonToolchainState, error) {
	return a.pythonSettings.InstallPythonDependencies()
}

func (a *App) PreparePythonToolchainEnvironment() (*PythonToolchainState, error) {
	return a.pythonSettings.PreparePythonToolchainEnvironment()
}

func (a *App) CheckPythonToolchainEnvironment() (*PythonToolchainState, error) {
	return a.pythonSettings.CheckPythonToolchainEnvironment()
}

func (a *App) DeletePythonToolchainEnvironment() (*PythonToolchainState, error) {
	return a.pythonSettings.DeletePythonToolchainEnvironment()
}

func (a *App) GetPythonToolchainTaskState() *PythonToolchainTaskState {
	return a.pythonSettings.GetPythonToolchainTaskState()
}

func (a *App) StartPreparePythonToolchainEnvironment() (*PythonToolchainTaskState, error) {
	return a.pythonSettings.StartPreparePythonToolchainEnvironment()
}

func (a *App) StartInstallPythonDependencies() (*PythonToolchainTaskState, error) {
	return a.pythonSettings.StartInstallPythonDependencies()
}

func (a *App) CancelActivePythonToolchainTask() error {
	return a.pythonSettings.CancelActivePythonToolchainTask()
}

// Rust toolchain

func (a *App) StartInstallRustCargoZigbuild() (*RustToolchainTaskState, error) {
	return a.rustSettings.StartInstallRustCargoZigbuild()
}

func (a *App) StartInstallRustTargets() (*RustToolchainTaskState, error) {
	return a.rustSettings.StartInstallRustTargets()
}

func (a *App) GetRustToolchainState() (*RustToolchainState, error) {
	return a.rustSettings.GetRustToolchainState()
}

func (a *App) CheckRustToolchainEnvironment() (*RustToolchainState, error) {
	return a.rustSettings.CheckRustToolchainEnvironment()
}

func (a *App) SaveRustToolchainConfig(cfg RustToolchainConfig) (*RustToolchainState, error) {
	return a.rustSettings.SaveRustToolchainConfig(cfg)
}

func (a *App) ListOfficialRustReleases() ([]RustOfficialRelease, error) {
	return a.rustSettings.ListOfficialRustReleases()
}

func (a *App) ListOfficialZigReleases() ([]ZigOfficialRelease, error) {
	return a.rustSettings.ListOfficialZigReleases()
}

func (a *App) DeleteManagedRustToolchainEnvironment() (*RustToolchainState, error) {
	return a.rustSettings.DeleteManagedRustToolchainEnvironment()
}

func (a *App) GetRustToolchainTaskState() *RustToolchainTaskState {
	return a.rustSettings.GetRustToolchainTaskState()
}

func (a *App) StartInstallRustToolchain(req InstallRustToolchainRequest) (*RustToolchainTaskState, error) {
	return a.rustSettings.StartInstallRustToolchain(req)
}

func (a *App) CancelActiveRustToolchainTask() error {
	return a.rustSettings.CancelActiveRustToolchainTask()
}
