package toolchain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"fire-salamander-desktop/internal/runtimeenv"
	"my_tools/libs/core/procutil"
)

type PythonOperationKind string

const (
	PythonOperationPrepare PythonOperationKind = "prepare"
	PythonOperationInstall PythonOperationKind = "install"
)

type PythonOperationProgress struct {
	Kind                 PythonOperationKind `json:"kind"`
	Step                 int                 `json:"step"`
	TotalSteps           int                 `json:"totalSteps"`
	ProgressPercent      float64             `json:"progressPercent"`
	Message              string              `json:"message"`
	Detail               string              `json:"detail,omitempty"`
	CurrentItem          string              `json:"currentItem,omitempty"`
	BaseBinary           string              `json:"baseBinary,omitempty"`
	EnvironmentDirectory string              `json:"environmentDirectory,omitempty"`
}

type PythonOperationHooks struct {
	OnProgress func(PythonOperationProgress)
}

func PrepareManagedPythonEnvironment() (PythonState, error) {
	return PrepareManagedPythonEnvironmentWithOptions(context.Background(), nil)
}

func CheckManagedPythonEnvironment() (PythonState, error) {
	state, err := GetPythonState()
	if err != nil {
		return PythonState{}, err
	}
	if strings.TrimSpace(state.ActiveBaseBinary) == "" {
		return state, fmt.Errorf("未检测到可用的基础 Python，请先在系统设置 > Python 中选择 Python 3")
	}
	if strings.TrimSpace(state.ActiveBinary) == "" || state.NeedsRebuild {
		return state, fmt.Errorf("当前 Python 工具环境尚未准备好，请先创建或重建工具环境")
	}
	if !state.PipAvailable {
		return state, fmt.Errorf("当前 Python 工具环境缺少 pip，请先重建工具环境")
	}
	if !state.DependenciesReady {
		return state, fmt.Errorf("当前 Python 工具环境仍缺少依赖：%s", strings.Join(state.MissingPackages, "、"))
	}
	return state, nil
}

func DeleteManagedPythonEnvironment() (PythonState, error) {
	state, err := GetPythonState()
	if err != nil {
		return PythonState{}, err
	}
	if strings.TrimSpace(state.ActiveBaseBinary) == "" {
		return state, fmt.Errorf("当前没有可删除的 Python 工具环境，请先选择基础 Python")
	}
	if strings.TrimSpace(state.ManagedEnvDirectory) == "" {
		return state, fmt.Errorf("当前基础 Python 尚未对应任何托管工具环境")
	}
	if err := os.RemoveAll(state.ManagedEnvDirectory); err != nil {
		return PythonState{}, fmt.Errorf("删除当前 Python 工具环境失败: %w", err)
	}
	return GetPythonState()
}

func PrepareManagedPythonEnvironmentWithOptions(ctx context.Context, hooks *PythonOperationHooks) (PythonState, error) {
	state, err := GetPythonState()
	if err != nil {
		return PythonState{}, err
	}
	if strings.TrimSpace(state.ActiveBaseBinary) == "" {
		return PythonState{}, fmt.Errorf("未检测到可用的基础 Python，请先在系统设置 > Python 中选择 Python 3")
	}
	layout, err := runtimeenv.ResolveLayout()
	if err != nil {
		return PythonState{}, fmt.Errorf("解析运行时目录失败: %w", err)
	}
	envDir := pythonManagedEnvDirectoryForBase(layout, state.ActiveBaseBinary)
	totalSteps := 5
	emitPythonOperationProgress(hooks, PythonOperationProgress{
		Kind:                 PythonOperationPrepare,
		Step:                 1,
		TotalSteps:           totalSteps,
		ProgressPercent:      percentForStep(0, totalSteps),
		Message:              "准备基础 Python 信息",
		Detail:               state.ActiveBaseVersion,
		BaseBinary:           state.ActiveBaseBinary,
		EnvironmentDirectory: envDir,
	})
	if err := ctx.Err(); err != nil {
		return PythonState{}, err
	}
	emitPythonOperationProgress(hooks, PythonOperationProgress{
		Kind:                 PythonOperationPrepare,
		Step:                 2,
		TotalSteps:           totalSteps,
		ProgressPercent:      percentForStep(2, totalSteps),
		Message:              "准备工具环境目录",
		BaseBinary:           state.ActiveBaseBinary,
		EnvironmentDirectory: envDir,
	})
	if err := os.MkdirAll(pythonManagedEnvsDirectory(layout), 0755); err != nil {
		return PythonState{}, fmt.Errorf("创建 Python 工具环境目录失败: %w", err)
	}
	if err := os.RemoveAll(envDir); err != nil {
		return PythonState{}, fmt.Errorf("清理当前 Python 工具环境失败: %w", err)
	}
	emitPythonOperationProgress(hooks, PythonOperationProgress{
		Kind:                 PythonOperationPrepare,
		Step:                 3,
		TotalSteps:           totalSteps,
		ProgressPercent:      percentForStep(3, totalSteps),
		Message:              "创建托管虚拟环境",
		BaseBinary:           state.ActiveBaseBinary,
		EnvironmentDirectory: envDir,
	})
	if err := runPythonCommand(ctx, state.ActiveBaseBinary, "-m", "venv", envDir); err != nil {
		return PythonState{}, fmt.Errorf("创建 Python 工具环境失败: %w", err)
	}
	managedBinary := managedPythonBinaryPath(envDir)
	if !isExistingFile(managedBinary) {
		return PythonState{}, fmt.Errorf("创建 Python 工具环境失败：未找到虚拟环境解释器")
	}
	emitPythonOperationProgress(hooks, PythonOperationProgress{
		Kind:                 PythonOperationPrepare,
		Step:                 4,
		TotalSteps:           totalSteps,
		ProgressPercent:      percentForStep(4, totalSteps),
		Message:              "检查并初始化 pip",
		BaseBinary:           state.ActiveBaseBinary,
		EnvironmentDirectory: envDir,
	})
	if !hasUsablePip(managedBinary) {
		if err := runPythonCommand(ctx, managedBinary, "-m", "ensurepip", "--upgrade"); err != nil && !hasUsablePip(managedBinary) {
			return PythonState{}, fmt.Errorf("创建 Python 工具环境成功，但初始化 pip 失败: %w", err)
		}
	}
	emitPythonOperationProgress(hooks, PythonOperationProgress{
		Kind:                 PythonOperationPrepare,
		Step:                 5,
		TotalSteps:           totalSteps,
		ProgressPercent:      percentForStep(5, totalSteps),
		Message:              "写入工具环境元数据",
		BaseBinary:           state.ActiveBaseBinary,
		EnvironmentDirectory: envDir,
	})
	if err := writeManagedPythonMetadata(envDir, managedPythonMetadata{
		BaseBinary:   state.ActiveBaseBinary,
		BaseIdentity: fileIdentity(state.ActiveBaseBinary),
		BaseVersion:  state.ActiveBaseVersion,
	}); err != nil {
		return PythonState{}, err
	}
	return GetPythonState()
}

func InstallPythonDependencies() (PythonState, error) {
	return InstallPythonDependenciesWithOptions(context.Background(), nil)
}

func InstallPythonDependenciesWithOptions(ctx context.Context, hooks *PythonOperationHooks) (PythonState, error) {
	state, err := GetPythonState()
	if err != nil {
		return PythonState{}, err
	}
	if strings.TrimSpace(state.ActiveBaseBinary) == "" {
		return PythonState{}, fmt.Errorf("未检测到可用的基础 Python，请先在系统设置 > Python 中选择 Python 3")
	}
	emitPythonOperationProgress(hooks, PythonOperationProgress{
		Kind:                 PythonOperationInstall,
		Step:                 1,
		TotalSteps:           1,
		ProgressPercent:      0,
		Message:              "检查当前工具环境",
		Detail:               state.ActiveBaseVersion,
		BaseBinary:           state.ActiveBaseBinary,
		EnvironmentDirectory: state.ManagedEnvDirectory,
	})
	preparedDuringInstall := false
	if strings.TrimSpace(state.ActiveBinary) == "" || state.NeedsRebuild {
		preparedDuringInstall = true
		state, err = PrepareManagedPythonEnvironmentWithOptions(ctx, &PythonOperationHooks{
			OnProgress: func(progress PythonOperationProgress) {
				emitPythonOperationProgress(hooks, PythonOperationProgress{
					Kind:                 PythonOperationInstall,
					Step:                 progress.Step,
					TotalSteps:           progress.TotalSteps,
					ProgressPercent:      remapProgress(progress.ProgressPercent, 5, 40),
					Message:              progress.Message,
					Detail:               progress.Detail,
					CurrentItem:          progress.CurrentItem,
					BaseBinary:           progress.BaseBinary,
					EnvironmentDirectory: progress.EnvironmentDirectory,
				})
			},
		})
		if err != nil {
			return PythonState{}, err
		}
	}
	if !state.PipAvailable {
		return PythonState{}, fmt.Errorf("当前 Python 工具环境缺少 pip，无法自动安装依赖")
	}
	missingPackages := collectMissingPythonPackages(state.Dependencies)
	if len(missingPackages) == 0 {
		emitPythonOperationProgress(hooks, PythonOperationProgress{
			Kind:                 PythonOperationInstall,
			Step:                 1,
			TotalSteps:           1,
			ProgressPercent:      100,
			Message:              "当前工具环境依赖已全部就绪",
			BaseBinary:           state.ActiveBaseBinary,
			EnvironmentDirectory: state.ManagedEnvDirectory,
		})
		return state, nil
	}
	totalSteps := len(missingPackages)
	emitPythonOperationProgress(hooks, PythonOperationProgress{
		Kind:                 PythonOperationInstall,
		Step:                 0,
		TotalSteps:           totalSteps,
		ProgressPercent:      installAnalyzeProgress(preparedDuringInstall),
		Message:              "解析缺失依赖",
		Detail:               strings.Join(missingPackages, "、"),
		BaseBinary:           state.ActiveBaseBinary,
		EnvironmentDirectory: state.ManagedEnvDirectory,
	})
	for index, pkg := range missingPackages {
		step := index + 1
		emitPythonOperationProgress(hooks, PythonOperationProgress{
			Kind:                 PythonOperationInstall,
			Step:                 step,
			TotalSteps:           totalSteps,
			ProgressPercent:      installProgressForPackage(index, len(missingPackages), false, preparedDuringInstall),
			Message:              fmt.Sprintf("安装依赖 %s", pkg),
			CurrentItem:          pkg,
			BaseBinary:           state.ActiveBaseBinary,
			EnvironmentDirectory: state.ManagedEnvDirectory,
		})
		if err := runPythonCommand(ctx, state.ActiveBinary, "-m", "pip", "install", pkg); err != nil {
			return PythonState{}, fmt.Errorf("安装 Python 依赖 %s 失败: %w", pkg, err)
		}
		emitPythonOperationProgress(hooks, PythonOperationProgress{
			Kind:                 PythonOperationInstall,
			Step:                 step,
			TotalSteps:           totalSteps,
			ProgressPercent:      installProgressForPackage(index, len(missingPackages), true, preparedDuringInstall),
			Message:              fmt.Sprintf("已安装依赖 %s", pkg),
			CurrentItem:          pkg,
			BaseBinary:           state.ActiveBaseBinary,
			EnvironmentDirectory: state.ManagedEnvDirectory,
		})
	}
	return GetPythonState()
}

func emitPythonOperationProgress(hooks *PythonOperationHooks, progress PythonOperationProgress) {
	if hooks == nil || hooks.OnProgress == nil {
		return
	}
	hooks.OnProgress(progress)
}

func percentForStep(step int, totalSteps int) float64 {
	if totalSteps <= 0 {
		return 0
	}
	if step < 0 {
		step = 0
	}
	if step > totalSteps {
		step = totalSteps
	}
	return roundProgressPercent(float64(step) * 100 / float64(totalSteps))
}

func runPythonCommand(ctx context.Context, binary string, args ...string) error {
	cmd := procutil.CommandContext(ctx, binary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return context.Canceled
		}
		return fmt.Errorf("%s", commandErrorDetail(err, output))
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return context.Canceled
	}
	return nil
}

func pythonManagedEnvsDirectory(layout runtimeenv.Layout) string {
	return filepath.Join(pythonManagedEnvDirectory(layout), "envs")
}

func pythonManagedEnvDirectoryForBase(layout runtimeenv.Layout, baseBinary string) string {
	return filepath.Join(pythonManagedEnvsDirectory(layout), pythonManagedEnvID(baseBinary))
}

func pythonManagedEnvID(baseBinary string) string {
	identity := fileIdentity(baseBinary)
	sum := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(sum[:])[:12]
}

func roundProgressPercent(value float64) float64 {
	return math.Round(value)
}

func remapProgress(value float64, min float64, max float64) float64 {
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	ratio := value / 100
	return roundProgressPercent(min + (max-min)*ratio)
}

func installAnalyzeProgress(preparedDuringInstall bool) float64 {
	if preparedDuringInstall {
		return 45
	}
	return 10
}

func installProgressForPackage(index int, total int, completed bool, preparedDuringInstall bool) float64 {
	start := 20.0
	if preparedDuringInstall {
		start = 55
	}
	end := 100.0
	if total <= 0 {
		if completed {
			return end
		}
		return start
	}
	currentStart := start + ((end-start)*float64(index))/float64(total)
	if !completed {
		return roundProgressPercent(currentStart)
	}
	currentEnd := start + ((end-start)*float64(index+1))/float64(total)
	if index == total-1 {
		currentEnd = end
	}
	return roundProgressPercent(currentEnd)
}
