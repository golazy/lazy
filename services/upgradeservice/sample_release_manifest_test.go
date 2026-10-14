package upgradeservice

import (
	"strings"
	"testing"
)

func TestSampleReleaseManifestRejectsCorruptedAndUnownedChanges(t *testing.T) {
	content := upgradeManifestContent("template\n")
	base := sampleReleaseManifest{From: "v0.1.22", To: "v0.1.23", PreviousCommit: strings.Repeat("a", 40), TargetCommit: strings.Repeat("b", 40), Files: []sampleReleaseFile{{Path: "README.md", Action: "update", Handling: "manual-documentation", Previous: &content, Target: &content}}}
	if err := validateSampleReleaseManifest(base); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		edit func(*sampleReleaseManifest)
	}{
		{"hash", func(m *sampleReleaseManifest) {
			bad := *m.Files[0].Target
			bad.Content += "corrupted"
			m.Files[0].Target = &bad
		}},
		{"path", func(m *sampleReleaseManifest) { m.Files[0].Path = "../README.md" }},
		{"owner", func(m *sampleReleaseManifest) { m.Files[0].Handling = "unknown" }},
		{"incomplete add", func(m *sampleReleaseManifest) { m.Files[0].Action = "add" }},
		{"duplicate", func(m *sampleReleaseManifest) { m.Files = append(m.Files, m.Files[0]) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			manifest := base
			manifest.Files = append([]sampleReleaseFile(nil), base.Files...)
			test.edit(&manifest)
			if err := validateSampleReleaseManifest(manifest); err == nil {
				t.Fatal("invalid artifact accepted")
			}
		})
	}
}

func TestBundledSampleReleaseManifest(t *testing.T) {
	manifest, found, err := readSampleReleaseManifest("v0.1.23")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Skip("sample file manifest is finalized after framework and sample publication")
	}
	if len(manifest.Files) == 0 {
		t.Fatal("release manifest has no changed sample files")
	}
	t.Logf("verified %d sample changes at %s", len(manifest.Files), manifest.TargetCommit)
}
