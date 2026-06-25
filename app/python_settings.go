package main

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
