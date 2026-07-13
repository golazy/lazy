package addonservice_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"golazy.dev/lazy/services/addonservice"
)

type artifactBoundarySource struct {
	addon addonservice.ResolvedAddon
}

func (artifactBoundarySource) ID() string { return "marketplace:adversarial" }

func (source artifactBoundarySource) Resolve(context.Context, addonservice.Query) ([]addonservice.ResolvedAddon, error) {
	return []addonservice.ResolvedAddon{source.addon}, nil
}

type offlineArtifactSource struct{}

func (offlineArtifactSource) ID() string { return "marketplace:offline-artifacts" }

func (offlineArtifactSource) Resolve(context.Context, addonservice.Query) ([]addonservice.ResolvedAddon, error) {
	return nil, errors.New("artifact registry unavailable")
}

func TestMarketplaceSourceRequiresExactDeclaredFileArtifacts(t *testing.T) {
	tests := []struct {
		name   string
		change func(*addonservice.ResolvedAddon)
		want   string
	}{
		{
			name: "missing artifact",
			change: func(addon *addonservice.ResolvedAddon) {
				addon.Artifacts = nil
			},
			want: "did not provide declared file artifact",
		},
		{
			name: "undeclared artifact",
			change: func(addon *addonservice.ResolvedAddon) {
				addon.Files = nil
			},
			want: "undeclared or mismatched file artifact",
		},
		{
			name: "plain metadata cannot authorize bytes",
			change: func(addon *addonservice.ResolvedAddon) {
				addon.Files = []string{"app/services/seo.txt"}
			},
			want: "undeclared or mismatched file artifact",
		},
		{
			name: "duplicate declaration",
			change: func(addon *addonservice.ResolvedAddon) {
				addon.Files = append(addon.Files, addon.Files[0])
			},
			want: "declares file",
		},
		{
			name: "duplicate target",
			change: func(addon *addonservice.ResolvedAddon) {
				addon.Files = append(addon.Files, "other.txt=>app/services/seo.txt")
				addon.Artifacts = append(addon.Artifacts, addonservice.FileArtifact{Source: "other.txt", Target: "app/services/seo.txt", Data: []byte("seo\n"), Mode: 0o644})
			},
			want: "declares target",
		},
		{
			name: "duplicate artifact",
			change: func(addon *addonservice.ResolvedAddon) {
				addon.Artifacts = append(addon.Artifacts, addon.Artifacts[0])
			},
			want: "provides file artifact",
		},
		{
			name: "mismatched source",
			change: func(addon *addonservice.ResolvedAddon) {
				addon.Artifacts[0].Source = "other.txt"
			},
			want: "undeclared or mismatched file artifact",
		},
		{
			name: "mismatched target",
			change: func(addon *addonservice.ResolvedAddon) {
				addon.Artifacts[0].Target = "app/services/other.txt"
			},
			want: "undeclared or mismatched file artifact",
		},
		{
			name: "non canonical source metadata",
			change: func(addon *addonservice.ResolvedAddon) {
				addon.Artifacts[0].Source = " payload.txt"
			},
			want: "non-canonical metadata",
		},
		{
			name: "escaping artifact metadata",
			change: func(addon *addonservice.ResolvedAddon) {
				addon.Artifacts[0].Source = "../payload.txt"
			},
			want: "invalid metadata",
		},
		{
			name: "invalid mapping",
			change: func(addon *addonservice.ResolvedAddon) {
				addon.Files[0] = "payload.txt=>../outside.txt"
			},
			want: "invalid file mapping",
		},
		{
			name: "reserved target",
			change: func(addon *addonservice.ResolvedAddon) {
				addon.Files[0] = "payload.txt=>addons.toml"
			},
			want: "installer-owned",
		},
		{
			name: "ignored Go target",
			change: func(addon *addonservice.ResolvedAddon) {
				addon.Files[0] = "payload.txt=>app/controllers/_seo.go"
			},
			want: "ignored by the Go toolchain",
		},
	}
	for _, mode := range []uint32{0, 0o600, 0o777, 0o100644} {
		mode := mode
		tests = append(tests, struct {
			name   string
			change func(*addonservice.ResolvedAddon)
			want   string
		}{
			name: "invalid mode " + formatMode(mode),
			change: func(addon *addonservice.ResolvedAddon) {
				addon.Artifacts[0].Mode = mode
			},
			want: "invalid mode",
		})
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			addon := exactFileArtifactAddon()
			test.change(&addon)
			err := resolveBoundaryAddon(t, addon)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("boundary error = %v, want error containing %q", err, test.want)
			}
		})
	}
}

func TestMarketplaceSourceRequiresExactDeclaredSourceEditArtifacts(t *testing.T) {
	tests := []struct {
		name   string
		change func(*addonservice.ResolvedAddon)
		want   string
	}{
		{
			name: "missing artifact",
			change: func(addon *addonservice.ResolvedAddon) {
				addon.EditArtifacts = nil
			},
			want: "did not provide declared source edit artifact",
		},
		{
			name: "undeclared artifact",
			change: func(addon *addonservice.ResolvedAddon) {
				addon.SourceEdits = nil
			},
			want: "undeclared or mismatched source edit artifact",
		},
		{
			name: "duplicate declaration",
			change: func(addon *addonservice.ResolvedAddon) {
				addon.SourceEdits = append(addon.SourceEdits, addon.SourceEdits[0])
			},
			want: "declares source edit",
		},
		{
			name: "duplicate artifact",
			change: func(addon *addonservice.ResolvedAddon) {
				addon.EditArtifacts = append(addon.EditArtifacts, addon.EditArtifacts[0])
			},
			want: "provides source edit artifact",
		},
		{
			name: "mismatched source",
			change: func(addon *addonservice.ResolvedAddon) {
				addon.EditArtifacts[0].Source = "edits/other.json"
			},
			want: "undeclared or mismatched source edit artifact",
		},
		{
			name: "non canonical metadata",
			change: func(addon *addonservice.ResolvedAddon) {
				addon.EditArtifacts[0].Source = " edits/seo.json"
			},
			want: "non-canonical metadata",
		},
		{
			name: "invalid artifact reference",
			change: func(addon *addonservice.ResolvedAddon) {
				addon.EditArtifacts[0].Source = "../seo.json"
			},
			want: "is invalid",
		},
		{
			name: "invalid declaration reference",
			change: func(addon *addonservice.ResolvedAddon) {
				addon.SourceEdits[0] = "../seo.json"
			},
			want: "declares invalid source edit",
		},
		{
			name: "malformed descriptor",
			change: func(addon *addonservice.ResolvedAddon) {
				addon.EditArtifacts[0].Data = []byte(`{"schema":2,"operations":[]}`)
			},
			want: "unsupported source edit schema",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			addon := exactSourceEditArtifactAddon()
			test.change(&addon)
			err := resolveBoundaryAddon(t, addon)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("boundary error = %v, want error containing %q", err, test.want)
			}
		})
	}
}

func TestMarketplaceSourceAcceptsExactArtifactMetadata(t *testing.T) {
	addon := exactFileArtifactAddon()
	edit := exactSourceEditArtifactAddon()
	addon.SourceEdits = edit.SourceEdits
	addon.EditArtifacts = edit.EditArtifacts
	dir := fixtureApp(t, "v0.1.21")
	if err := os.Remove(dir + "/lazyaddon.toml"); err != nil {
		t.Fatal(err)
	}
	plan, err := (addonservice.Manager{Dir: dir, Sources: []addonservice.Source{artifactBoundarySource{addon: addon}}}).Add("seo@v1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Changed() {
		t.Fatal("exact source artifacts produced no plan")
	}
}

func TestStateSourceExplicitlyAllowsMissingTransientArtifactsOffline(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	if err := os.Remove(dir + "/lazyaddon.toml"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "plugin/lazyaddon.toml", `manifest = 1

[package]
id = "example.com/offline"
version = "v1.0.0"

[[addons]]
id = "alpha"
go_import = "example.com/offline/alpha"
files = ["files/alpha.txt=>app/services/alpha.txt"]
source_edits = ["edits/alpha.json"]

[[addons]]
id = "beta"
go_import = "example.com/offline/beta"
`)
	writeFile(t, dir, "plugin/files/alpha.txt", "alpha\n")
	writeFile(t, dir, "plugin/edits/alpha.json", `{"schema":1,"operations":[{"kind":"toml.set_string.v1","path":"settings.toml","key":"enabled","value":"true"}]}`)
	writeFile(t, dir, "settings.toml", "name = \"app\"\n")
	manager := addonservice.Manager{Dir: dir, ManifestPaths: []string{"plugin/lazyaddon.toml"}}
	for _, id := range []string{"alpha", "beta"} {
		plan, err := manager.Add(id)
		if err != nil {
			t.Fatal(err)
		}
		if err := plan.Apply(); err != nil {
			t.Fatal(err)
		}
	}
	alphaFile := readFile(t, dir, "app/services/alpha.txt")
	settings := readFile(t, dir, "settings.toml")
	if err := os.RemoveAll(dir + "/plugin"); err != nil {
		t.Fatal(err)
	}

	remove, err := (addonservice.Manager{Dir: dir, Sources: []addonservice.Source{offlineArtifactSource{}}}).Remove("beta")
	if err != nil {
		t.Fatalf("offline locked-snapshot removal: %v", err)
	}
	if len(remove.After.Addons) != 1 || len(remove.After.Addons[0].Artifacts) != 0 || len(remove.After.Addons[0].EditArtifacts) != 0 {
		t.Fatalf("offline resolved add-on retained transient artifacts: %#v", remove.After.Addons)
	}
	if err := remove.Apply(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, dir, "app/services/alpha.txt"); got != alphaFile {
		t.Fatalf("offline file = %q, want %q", got, alphaFile)
	}
	if got := readFile(t, dir, "settings.toml"); got != settings {
		t.Fatalf("offline source edit = %q, want %q", got, settings)
	}
}

func exactFileArtifactAddon() addonservice.ResolvedAddon {
	addon := memoryAddon("seo", "v1.2.3")
	addon.Files = []string{"payload.txt=>app/services/seo.txt"}
	addon.Artifacts = []addonservice.FileArtifact{{
		Source: "payload.txt",
		Target: "app/services/seo.txt",
		Data:   []byte("seo\n"),
		Mode:   0o644,
	}}
	return addon
}

func exactSourceEditArtifactAddon() addonservice.ResolvedAddon {
	addon := memoryAddon("seo", "v1.2.3")
	addon.SourceEdits = []string{"edits/seo.json"}
	addon.EditArtifacts = []addonservice.SourceEditArtifact{{
		Source: "edits/seo.json",
		Data:   []byte(`{"schema":1,"operations":[{"kind":"toml.set_string.v1","path":"settings.toml","key":"enabled","value":"true"}]}`),
	}}
	return addon
}

func resolveBoundaryAddon(t *testing.T, addon addonservice.ResolvedAddon) error {
	t.Helper()
	dir := fixtureApp(t, "v0.1.21")
	if err := os.Remove(dir + "/lazyaddon.toml"); err != nil {
		t.Fatal(err)
	}
	_, err := (addonservice.Manager{Dir: dir, Sources: []addonservice.Source{artifactBoundarySource{addon: addon}}}).Add("seo@v1.2.3")
	return err
}

func formatMode(mode uint32) string {
	const digits = "01234567"
	if mode == 0 {
		return "0"
	}
	var value [12]byte
	index := len(value)
	for mode > 0 {
		index--
		value[index] = digits[mode&7]
		mode >>= 3
	}
	return string(value[index:])
}
