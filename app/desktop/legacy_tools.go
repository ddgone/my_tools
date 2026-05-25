package main

import (
	"context"
	"io"
	"regexp"
	"strings"
	"sync"

	"my_tools/catalog/builtin"
	"my_tools/core/toolspec"
	"my_tools/pkg/framework"

	_ "my_tools/tools/geojson_to_shapefile"
	_ "my_tools/tools/pos_trajectory_to_gis"
	_ "my_tools/tools/python_tools"
	_ "my_tools/tools/utm_extract_to_gis"
)

type terminalRunFunc func(ctx context.Context, args string, out io.Writer) error
type pythonRunFunc func(ctx context.Context, env string, args string, out io.Writer) error

type legacyTool struct {
	tool      framework.Tool
	title     string
	usage     string
	run       terminalRunFunc
	runPython pythonRunFunc
}

type captureAppContext struct {
	tool      framework.Tool
	title     string
	usage     string
	run       terminalRunFunc
	runPython pythonRunFunc
}

func (c *captureAppContext) ShowModal(title, message string) {}
func (c *captureAppContext) PromptInput(title, prompt, defaultValue string, callback func(string)) {
}
func (c *captureAppContext) PromptChoice(title, prompt string, options []string, callback func(string)) {
}
func (c *captureAppContext) ShowTerminal(title string, usage string, run func(ctx context.Context, args string, out io.Writer) error) {
	c.title = title
	c.usage = usage
	c.run = run
}
func (c *captureAppContext) ShowPythonTerminal(title string, usage string, run func(ctx context.Context, env string, args string, out io.Writer) error) {
	c.title = title
	c.usage = usage
	c.runPython = run
}
func (c *captureAppContext) GetLastParam(key string) string       { return "" }
func (c *captureAppContext) RecordUsage(params map[string]string) {}

var markupStripper = regexp.MustCompile(`\[[^\]]*\]`)

func stripUsageMarkup(input string) string {
	cleaned := markupStripper.ReplaceAllString(input, "")
	lines := strings.Split(cleaned, "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		normalized = append(normalized, strings.TrimRight(line, " \t"))
	}
	return strings.TrimSpace(strings.Join(normalized, "\n"))
}

func deriveSummary(usage string, fallback string) string {
	for _, line := range strings.Split(usage, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasSuffix(line, "工具") || strings.Contains(line, "说明") {
			continue
		}
		return line
	}
	return fallback
}

func loadLegacyTools() map[string]*legacyTool {
	result := make(map[string]*legacyTool, len(framework.Registry))
	for _, tool := range framework.Registry {
		capture := &captureAppContext{tool: tool}
		tool.Execute(capture)
		result[tool.ID()] = &legacyTool{
			tool:      tool,
			title:     capture.title,
			usage:     stripUsageMarkup(capture.usage),
			run:       capture.run,
			runPython: capture.runPython,
		}
	}
	return result
}

func buildToolManifests(legacy map[string]*legacyTool) (map[string]toolspec.ToolManifest, error) {
	loaded, err := builtin.Load()
	if err != nil {
		return nil, err
	}

	manifests := make(map[string]toolspec.ToolManifest, len(loaded))
	for _, manifest := range loaded {
		manifests[manifest.ID] = manifest
	}

	for id, legacyTool := range legacy {
		manifest, ok := manifests[id]
		if !ok {
			kind := toolspec.ToolKindGo
			if legacyTool.runPython != nil {
				kind = toolspec.ToolKindPython
			}
			manifest = toolspec.ToolManifest{
				ID:          id,
				Name:        legacyTool.tool.Name(),
				Kind:        kind,
				Category:    legacyTool.tool.Category(),
				Icon:        string(kind),
				Description: deriveSummary(legacyTool.usage, legacyTool.tool.Name()),
				Docs: toolspec.ToolDocs{
					Summary: deriveSummary(legacyTool.usage, legacyTool.tool.Name()),
					Usage:   legacyTool.usage,
				},
			}
		}

		if manifest.Name == "" {
			manifest.Name = legacyTool.tool.Name()
		}
		if manifest.Category == "" {
			manifest.Category = legacyTool.tool.Category()
		}
		if manifest.Docs.Usage == "" {
			manifest.Docs.Usage = legacyTool.usage
		}
		if manifest.Docs.Summary == "" {
			manifest.Docs.Summary = deriveSummary(legacyTool.usage, manifest.Description)
		}
		if manifest.Description == "" {
			manifest.Description = deriveSummary(legacyTool.usage, legacyTool.tool.Name())
		}

		manifests[id] = manifest
	}

	return manifests, nil
}

var (
	toolInitOnce     sync.Once
	cachedLegacy     map[string]*legacyTool
	cachedManifests  map[string]toolspec.ToolManifest
	cachedToolingErr error
)
