package main

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"fire-salamander-desktop/internal/runtime"
	"my_tools/libs/core/toolspec"
	"my_tools/libs/framework"
)

type remoteResultHint struct {
	Path string
	Kind string
}

func resolveRemoteResultHint(params []toolspec.ParameterSpec, rawArgs string, remoteWorkDir string) (remoteResultHint, error) {
	outputParam, ok := findLikelyOutputParam(params)
	if !ok {
		return remoteResultHint{}, nil
	}

	parsedArgs, err := framework.ParseArgs(rawArgs)
	if err != nil {
		return remoteResultHint{}, err
	}
	value, ok := extractParamValue(parsedArgs, outputParam)
	if !ok || strings.TrimSpace(value) == "" {
		value, ok = inferDefaultOutputValue(params, parsedArgs, outputParam, remoteWorkDir)
		if !ok || strings.TrimSpace(value) == "" {
			return remoteResultHint{}, nil
		}
	}

	kind := "directory"
	if strings.TrimSpace(outputParam.PathMode) == "file" {
		kind = "file"
	}
	return remoteResultHint{
		Path: resolveRemotePath(value, remoteWorkDir),
		Kind: kind,
	}, nil
}

func inferDefaultOutputValue(params []toolspec.ParameterSpec, parsedArgs []string, outputParam toolspec.ParameterSpec, remoteWorkDir string) (string, bool) {
	if !supportsDefaultOutputInference(outputParam) {
		return "", false
	}

	inputValue, inputParam, ok := findLikelyInputParamValue(params, parsedArgs, outputParam)
	if !ok {
		return "", false
	}
	resolvedInputPath := resolveRemotePath(inputValue, remoteWorkDir)
	if strings.TrimSpace(resolvedInputPath) == "" {
		return "", false
	}

	switch strings.TrimSpace(inputParam.PathMode) {
	case "file":
		return path.Join(path.Dir(resolvedInputPath), "output"), true
	case "directory":
		return path.Join(resolvedInputPath, "output"), true
	default:
		if looksLikeFilePath(resolvedInputPath) {
			return path.Join(path.Dir(resolvedInputPath), "output"), true
		}
		return path.Join(resolvedInputPath, "output"), true
	}
}

func supportsDefaultOutputInference(outputParam toolspec.ParameterSpec) bool {
	if outputParam.Type != toolspec.FieldTypePath {
		return false
	}
	if strings.TrimSpace(outputParam.PathMode) == "file" {
		return false
	}
	if !isLikelyOutputParam(outputParam) {
		return false
	}

	text := strings.ToLower(strings.Join([]string{
		strings.TrimSpace(outputParam.Placeholder),
		strings.TrimSpace(outputParam.Help),
	}, " "))
	return (strings.Contains(text, "默认") || strings.Contains(text, "留空")) && strings.Contains(text, "output")
}

func findLikelyInputParamValue(params []toolspec.ParameterSpec, parsedArgs []string, outputParam toolspec.ParameterSpec) (string, toolspec.ParameterSpec, bool) {
	for _, param := range params {
		if param.Type != toolspec.FieldTypePath {
			continue
		}
		if isSameParam(param, outputParam) || isLikelyOutputParam(param) {
			continue
		}
		value, ok := extractParamValue(parsedArgs, param)
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		return value, param, true
	}
	return "", toolspec.ParameterSpec{}, false
}

func findLikelyOutputParam(params []toolspec.ParameterSpec) (toolspec.ParameterSpec, bool) {
	for _, param := range params {
		if isLikelyOutputParam(param) {
			return param, true
		}
	}
	return toolspec.ParameterSpec{}, false
}

func isLikelyOutputParam(param toolspec.ParameterSpec) bool {
	if param.Type != toolspec.FieldTypePath {
		return false
	}
	argKey := strings.TrimSpace(param.ArgKey)
	key := strings.TrimSpace(param.Key)
	switch {
	case argKey == "output":
		return true
	case key == "output", key == "outputDir":
		return true
	default:
		return false
	}
}

func isSameParam(left toolspec.ParameterSpec, right toolspec.ParameterSpec) bool {
	return strings.TrimSpace(left.Key) == strings.TrimSpace(right.Key) &&
		strings.TrimSpace(left.ArgKey) == strings.TrimSpace(right.ArgKey)
}

func extractParamValue(parsedArgs []string, param toolspec.ParameterSpec) (string, bool) {
	candidates := make([]string, 0, 4)
	if key := strings.TrimSpace(param.ArgKey); key != "" {
		candidates = append(candidates, "-"+key, "--"+key)
	}
	if key := strings.TrimSpace(param.Key); key != "" && key != param.ArgKey {
		candidates = append(candidates, "-"+key, "--"+key)
	}

	for i := 0; i < len(parsedArgs); i++ {
		token := parsedArgs[i]
		if !containsString(candidates, token) {
			continue
		}
		if i+1 >= len(parsedArgs) {
			return "", false
		}
		next := parsedArgs[i+1]
		if strings.HasPrefix(next, "-") {
			return "", false
		}
		return next, true
	}
	return "", false
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func resolveRemotePath(value string, remoteWorkDir string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if path.IsAbs(trimmed) {
		return path.Clean(trimmed)
	}
	return path.Clean(path.Join(strings.TrimSpace(remoteWorkDir), trimmed))
}

func looksLikeFilePath(value string) bool {
	base := path.Base(strings.TrimSpace(value))
	ext := path.Ext(base)
	return base != "" && ext != "" && ext != "."
}

func probeRemoteResult(ctx context.Context, executor *runtime.RemoteExecutor, remotePath string) (remoteResultProbe, error) {
	kind, exists, err := executor.DetectPathKind(ctx, remotePath)
	if err != nil {
		return remoteResultProbe{}, err
	}
	if !exists {
		return remoteResultProbe{
			Status:  "missing",
			Path:    remotePath,
			Message: "未发现可下载结果",
		}, nil
	}

	message := "已探测到可下载结果"
	if kind == "directory" {
		message = "已探测到可下载的输出目录"
	} else if kind == "file" {
		message = "已探测到可下载的输出文件"
	}
	return remoteResultProbe{
		Status:  "available",
		Path:    remotePath,
		Kind:    kind,
		Message: message,
	}, nil
}

func pathWithinRemoteBase(base string, target string) bool {
	cleanBase := path.Clean(strings.TrimSpace(base))
	cleanTarget := path.Clean(strings.TrimSpace(target))
	if cleanBase == "" || cleanTarget == "" {
		return false
	}
	return cleanTarget == cleanBase || strings.HasPrefix(cleanTarget, cleanBase+"/")
}

func buildRemoteArchiveFileName(taskID string, toolID string, remoteResultPath string) string {
	base := sanitizeRemoteName(strings.TrimSpace(toolID))
	if base == "" {
		base = sanitizeRemoteName(path.Base(strings.TrimSpace(remoteResultPath)))
	}
	if base == "" {
		base = "result"
	}
	suffix := sanitizeRemoteName(strings.TrimSpace(taskID))
	if suffix == "" {
		suffix = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("fire-salamander-%s-%s.tar.gz", base, suffix)
}

func sanitizeRemoteName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	sanitized := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-', r == '_', r == '.':
			return r
		default:
			return '-'
		}
	}, value)
	return strings.Trim(sanitized, "-.")
}
