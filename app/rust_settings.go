package main

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
