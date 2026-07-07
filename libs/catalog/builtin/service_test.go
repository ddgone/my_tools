package builtin

import "testing"

func TestLoadShouldParseAllBuiltinManifests(t *testing.T) {
	manifests, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(manifests) == 0 {
		t.Fatalf("expected builtin manifests to be non-empty")
	}
}

func TestLoadShouldProvideUniqueNonEmptyIDs(t *testing.T) {
	manifests, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	seen := make(map[string]string, len(manifests))
	for _, manifest := range manifests {
		if manifest.ID == "" {
			t.Fatalf("found builtin manifest with empty id: %+v", manifest)
		}
		if manifest.Name == "" {
			t.Fatalf("found builtin manifest with empty name: id=%s", manifest.ID)
		}
		if manifest.Source.Entry == "" {
			t.Fatalf("found builtin manifest with empty source.entry: id=%s", manifest.ID)
		}
		if other, exists := seen[manifest.ID]; exists {
			t.Fatalf("duplicate builtin manifest id %q: %s and %s", manifest.ID, other, manifest.Name)
		}
		seen[manifest.ID] = manifest.Name
	}
}
