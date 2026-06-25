package artifact

import (
	"fmt"
	"strings"

	"fire-salamander-desktop/internal/shared"
)

type artifactBatchResolvedRequest struct {
	Mode            string
	ExportRootDir   string
	Concurrency     int
	SkipUnchanged   bool
	PreferCache     bool
	ForceRebuild    bool
	ContinueOnError bool
	Items           []shared.ArtifactBatchSelection
}

func normalizeArtifactBatchMode(mode string) string {
	if strings.TrimSpace(mode) == shared.ArtifactBatchModeBuildCache {
		return shared.ArtifactBatchModeBuildCache
	}
	return shared.ArtifactBatchModeExport
}

func artifactItemKey(toolID string, targetOS string, targetArch string) string {
	return fmt.Sprintf("%s:%s/%s", toolID, targetOS, targetArch)
}
