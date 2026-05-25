package builtin

import (
	"embed"
	"io/fs"
	"sort"

	"my_tools/libs/core/toolspec"

	"gopkg.in/yaml.v3"
)

//go:embed manifests/*.yaml
var manifestFS embed.FS

func Load() ([]toolspec.ToolManifest, error) {
	entries, err := fs.ReadDir(manifestFS, "manifests")
	if err != nil {
		return nil, err
	}

	manifests := make([]toolspec.ToolManifest, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		payload, err := manifestFS.ReadFile("manifests/" + entry.Name())
		if err != nil {
			return nil, err
		}

		var manifest toolspec.ToolManifest
		if err := yaml.Unmarshal(payload, &manifest); err != nil {
			return nil, err
		}

		manifests = append(manifests, manifest)
	}

	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].Name < manifests[j].Name
	})

	return manifests, nil
}
