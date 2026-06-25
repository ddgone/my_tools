package main

import (
	"fmt"
	"strings"
)

type artifactBatchResolvedRequest struct {
	Mode            string
	ExportRootDir   string
	Concurrency     int
	SkipUnchanged   bool
	PreferCache     bool
	ForceRebuild    bool
	ContinueOnError bool
	Items           []ArtifactBatchSelection
}

func normalizeArtifactBatchMode(mode string) string {
	if strings.TrimSpace(mode) == artifactBatchModeBuildCache {
		return artifactBatchModeBuildCache
	}
	return artifactBatchModeExport
}

func artifactItemKey(toolID string, targetOS string, targetArch string) string {
	return fmt.Sprintf("%s:%s/%s", toolID, targetOS, targetArch)
}
