package main

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
