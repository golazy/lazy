package addonservice_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golazy.dev/lazy/services/addonservice"
)

func TestDeclarativeSourceEditsAreIdempotentSharedAndReversible(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	writeFile(t, dir, "plugin/lazyaddon.toml", sourceEditManifest(true))
	writeFile(t, dir, "plugin/edits/shared.json", `{
  "schema": 1,
  "operations": [
    {"kind":"toml.set_strings.v1","path":"settings.toml","table":"seo","key":"features","values":["robots","canonical"]},
    {"kind":"go.ensure_import.v1","path":"app/services/source_edit_target.go","import":"example.com/sourceedit/runtime","name":"_"},
    {"kind":"toml.set_string.v1","path":"settings.toml","table":"seo","key":"title","value":"Add-on title"}
  ]
}`)
	writeFile(t, dir, "settings.toml", "# application settings\n[seo]\ntitle = \"Application title\" # keep this comment\nfeatures = [\"application\"]\nunrelated = true\n")
	writeFile(t, dir, "app/services/source_edit_target.go", "package services\n\nfunc SourceEditTarget() {}\n")

	manager := addonservice.Manager{Dir: dir, ManifestPaths: []string{"plugin/lazyaddon.toml"}}
	installAlpha, err := manager.Add("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if err := installAlpha.Apply(); err != nil {
		t.Fatal(err)
	}
	settings := readFile(t, dir, "settings.toml")
	for _, expected := range []string{`title = "Add-on title" # keep this comment`, `features = ["robots", "canonical"]`, "unrelated = true"} {
		if !strings.Contains(settings, expected) {
			t.Fatalf("settings.toml does not contain %q:\n%s", expected, settings)
		}
	}
	if source := readFile(t, dir, "app/services/source_edit_target.go"); !strings.Contains(source, `_ "example.com/sourceedit/runtime"`) {
		t.Fatalf("Go import edit =\n%s", source)
	}
	state, err := addonservice.ReadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := state.Addons[0].SourceEdits; len(got) != 1 || got[0] != "edits/shared.json" {
		t.Fatalf("locked source edits = %#v", got)
	}
	if file := managedFile(state, "settings.toml"); file.Kind != "source-edit" || len(file.Properties) != 2 || strings.Join(file.Owners, ",") != "alpha" {
		t.Fatalf("settings ownership = %#v", file)
	}

	idempotent, err := (addonservice.Manager{Dir: dir}).Add("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if idempotent.Changed() {
		t.Fatalf("idempotent source edits = %#v", idempotent.Result.Files)
	}

	installBeta, err := manager.Add("beta")
	if err != nil {
		t.Fatal(err)
	}
	if err := installBeta.Apply(); err != nil {
		t.Fatal(err)
	}
	state, err = addonservice.ReadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if owners := strings.Join(managedFile(state, "settings.toml").Owners, ","); owners != "alpha,beta" {
		t.Fatalf("shared source edit owners = %q", owners)
	}
	removeAlpha, err := manager.Remove("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if err := removeAlpha.Apply(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readFile(t, dir, "settings.toml"), `title = "Add-on title"`) {
		t.Fatal("removing one owner restored a still-shared property")
	}

	drifted := strings.Replace(readFile(t, dir, "settings.toml"), `title = "Add-on title"`, `title = "local edit"`, 1)
	writeFile(t, dir, "settings.toml", drifted)
	if _, err := manager.Remove("beta"); err == nil || !strings.Contains(err.Error(), "drifted") {
		t.Fatalf("source property drift error = %v", err)
	}
	writeFile(t, dir, "settings.toml", strings.Replace(drifted, `title = "local edit"`, `title = "Add-on title"`, 1))
	removeBeta, err := manager.Remove("beta")
	if err != nil {
		t.Fatal(err)
	}
	if err := removeBeta.Apply(); err != nil {
		t.Fatal(err)
	}
	settings = readFile(t, dir, "settings.toml")
	for _, expected := range []string{`title = "Application title" # keep this comment`, `features = ["application"]`, "unrelated = true"} {
		if !strings.Contains(settings, expected) {
			t.Fatalf("restored settings do not contain %q:\n%s", expected, settings)
		}
	}
	if source := readFile(t, dir, "app/services/source_edit_target.go"); strings.Contains(source, "example.com/sourceedit/runtime") {
		t.Fatalf("Go import was not removed:\n%s", source)
	}
}

func TestDeclarativeSourceEditCreatedFileAndUnrelatedChanges(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	writeFile(t, dir, "plugin/lazyaddon.toml", singleSourceEditManifest("created", "edits/created.json"))
	writeFile(t, dir, "plugin/edits/created.json", `{"schema":1,"operations":[{"kind":"toml.set_string.v1","path":"config/created.toml","table":"addon","key":"enabled","value":"yes"}]}`)
	manager := addonservice.Manager{Dir: dir, ManifestPaths: []string{"plugin/lazyaddon.toml"}}
	install, err := manager.Add("created")
	if err != nil {
		t.Fatal(err)
	}
	if err := install.Apply(); err != nil {
		t.Fatal(err)
	}
	state, err := addonservice.ReadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if kind := managedFile(state, "config/created.toml").Kind; kind != "source-edit-created" {
		t.Fatalf("created file kind = %q", kind)
	}
	remove, err := manager.Remove("created")
	if err != nil {
		t.Fatal(err)
	}
	if err := remove.Apply(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "config/created.toml")); !os.IsNotExist(err) {
		t.Fatalf("installer-created source edit file remains: %v", err)
	}

	install, err = manager.Add("created")
	if err != nil {
		t.Fatal(err)
	}
	if err := install.Apply(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "config/created.toml", readFile(t, dir, "config/created.toml")+"\n[application]\nkept = true\n")
	remove, err = manager.Remove("created")
	if err != nil {
		t.Fatal(err)
	}
	if err := remove.Apply(); err != nil {
		t.Fatal(err)
	}
	remaining := readFile(t, dir, "config/created.toml")
	if strings.Contains(remaining, "enabled") || !strings.Contains(remaining, "kept = true") {
		t.Fatalf("unrelated change was not preserved:\n%s", remaining)
	}
}

func TestSourceEditDoesNotRemoveNamedImportStillUsedByApplication(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	writeFile(t, dir, "plugin/lazyaddon.toml", singleSourceEditManifest("named", "edits/named.json"))
	writeFile(t, dir, "plugin/edits/named.json", `{"schema":1,"operations":[{"kind":"go.ensure_import.v1","path":"app/services/named.go","import":"example.com/sourceedit/runtime","name":"runtimepkg"}]}`)
	writeFile(t, dir, "app/services/named.go", "package services\n\nfunc Named() {}\n")
	manager := addonservice.Manager{Dir: dir, ManifestPaths: []string{"plugin/lazyaddon.toml"}}
	install, err := manager.Add("named")
	if err != nil {
		t.Fatal(err)
	}
	if err := install.Apply(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "app/services/named.go", strings.Replace(readFile(t, dir, "app/services/named.go"), "func Named() {}", "func Named() { _ = runtimepkg.Value }", 1))
	if _, err := manager.Remove("named"); err == nil || !strings.Contains(err.Error(), "still uses import name") {
		t.Fatalf("used import removal error = %v", err)
	}
	writeFile(t, dir, "app/services/named.go", strings.Replace(readFile(t, dir, "app/services/named.go"), "func Named() { _ = runtimepkg.Value }", "func Named() {}", 1))
	remove, err := manager.Remove("named")
	if err != nil {
		t.Fatal(err)
	}
	if err := remove.Apply(); err != nil {
		t.Fatal(err)
	}
}

func TestSourceEditDescriptorBytesArePartOfImmutableReleaseDigest(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	writeFile(t, dir, "plugin/lazyaddon.toml", singleSourceEditManifest("immutable", "edits/immutable.json"))
	descriptor := `{"schema":1,"operations":[{"kind":"toml.set_string.v1","path":"settings.toml","key":"value","value":"one"}]}`
	writeFile(t, dir, "plugin/edits/immutable.json", descriptor)
	manager := addonservice.Manager{Dir: dir, ManifestPaths: []string{"plugin/lazyaddon.toml"}}
	install, err := manager.Add("immutable")
	if err != nil {
		t.Fatal(err)
	}
	if err := install.Apply(); err != nil {
		t.Fatal(err)
	}
	state, err := addonservice.ReadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	digest := state.Addons[0].ManifestDigest
	writeFile(t, dir, "plugin/edits/immutable.json", strings.Replace(descriptor, `"one"`, `"two"`, 1))
	if _, err := (addonservice.Manager{Dir: dir}).Update(); err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("changed descriptor immutability error = %v", err)
	}
	state, err = addonservice.ReadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if state.Addons[0].ManifestDigest != digest {
		t.Fatal("failed update changed locked descriptor digest")
	}
}

func TestSourceEditsComposeWithPostgresTypedTOMLContributions(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	writeFile(t, dir, "plugin/lazyaddon.toml", singleSourceEditManifest("settings", "edits/settings.json"))
	writeFile(t, dir, "plugin/edits/settings.json", `{
  "schema": 1,
  "operations": [
    {"kind":"toml.set_string.v1","path":"lazy.toml","table":"tmux","key":"session","value":"addon-kept"},
    {"kind":"toml.set_string.v1","path":"mise.toml","table":"env","key":"SEO_MODE","value":"strict"}
  ]
}`)
	manager := addonservice.Manager{Dir: dir, ManifestPaths: []string{"lazyaddon.toml", "plugin/lazyaddon.toml"}}
	installPostgres, err := manager.Add("postgres")
	if err != nil {
		t.Fatal(err)
	}
	if err := installPostgres.Apply(); err != nil {
		t.Fatal(err)
	}
	installSettings, err := manager.Add("settings")
	if err != nil {
		t.Fatal(err)
	}
	if err := installSettings.Apply(); err != nil {
		t.Fatal(err)
	}
	state, err := addonservice.ReadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"lazy.toml", "mise.toml"} {
		managed := managedFile(state, name)
		if managed.Kind != "composite" || strings.Join(managed.Owners, ",") != "postgres,settings" {
			t.Fatalf("%s managed state = %#v", name, managed)
		}
	}
	if lazyConfig := readFile(t, dir, "lazy.toml"); !strings.Contains(lazyConfig, `services = ["postgres"]`) || !strings.Contains(lazyConfig, `session = "addon-kept"`) {
		t.Fatalf("composed lazy.toml =\n%s", lazyConfig)
	}
	if miseConfig := readFile(t, dir, "mise.toml"); !strings.Contains(miseConfig, `DATABASE_URL = `) || !strings.Contains(miseConfig, `SEO_MODE = "strict"`) {
		t.Fatalf("composed mise.toml =\n%s", miseConfig)
	}

	removePostgres, err := manager.Remove("postgres")
	if err != nil {
		t.Fatal(err)
	}
	if err := removePostgres.Apply(); err != nil {
		t.Fatal(err)
	}
	if lazyConfig := readFile(t, dir, "lazy.toml"); strings.Contains(lazyConfig, "postgres") || !strings.Contains(lazyConfig, `session = "addon-kept"`) {
		t.Fatalf("lazy.toml after removing postgres =\n%s", lazyConfig)
	}
	if miseConfig := readFile(t, dir, "mise.toml"); strings.Contains(miseConfig, "DB_NAME") || !strings.Contains(miseConfig, `SEO_MODE = "strict"`) {
		t.Fatalf("mise.toml after removing postgres =\n%s", miseConfig)
	}

	removeSettings, err := manager.Remove("settings")
	if err != nil {
		t.Fatal(err)
	}
	if err := removeSettings.Apply(); err != nil {
		t.Fatal(err)
	}
	if lazyConfig := readFile(t, dir, "lazy.toml"); strings.Contains(lazyConfig, "addon-kept") {
		t.Fatalf("lazy.toml after removing settings =\n%s", lazyConfig)
	}
	if miseConfig := readFile(t, dir, "mise.toml"); strings.Contains(miseConfig, "SEO_MODE") {
		t.Fatalf("mise.toml after removing settings =\n%s", miseConfig)
	}
}

func TestDeclarativeSourceEditsRejectConflictsMalformedDescriptorsAndEscapes(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	writeFile(t, dir, "plugin/lazyaddon.toml", `manifest = 1

[package]
id = "example.com/sourceedit"
version = "v1.0.0"

[[addons]]
id = "alpha"
go_import = "example.com/sourceedit/alpha"
source_edits = ["edits/alpha.json"]

[[addons]]
id = "beta"
go_import = "example.com/sourceedit/beta"
source_edits = ["edits/beta.json"]
`)
	writeFile(t, dir, "plugin/edits/alpha.json", `{"schema":1,"operations":[{"kind":"toml.set_string.v1","path":"settings.toml","key":"owner","value":"alpha"}]}`)
	writeFile(t, dir, "plugin/edits/beta.json", `{"schema":1,"operations":[{"kind":"toml.set_string.v1","path":"settings.toml","key":"owner","value":"beta"}]}`)
	writeFile(t, dir, "settings.toml", "owner = \"application\"\n")
	manager := addonservice.Manager{Dir: dir, ManifestPaths: []string{"plugin/lazyaddon.toml"}}
	install, err := manager.Add("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if err := install.Apply(); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Add("beta"); err == nil || !strings.Contains(err.Error(), "incompatible source edits") {
		t.Fatalf("source edit conflict error = %v", err)
	}

	malformed := map[string]string{
		"unknown schema":      `{"schema":2,"operations":[{"kind":"toml.set_string.v1","path":"settings.toml","key":"x","value":"y"}]}`,
		"unknown operation":   `{"schema":1,"operations":[{"kind":"shell.run.v1","path":"settings.toml"}]}`,
		"unknown field":       `{"schema":1,"operations":[{"kind":"toml.set_string.v1","path":"settings.toml","key":"x","value":"y","command":"rm"}]}`,
		"duplicate field":     `{"schema":1,"operations":[{"kind":"toml.set_string.v1","path":"settings.toml","key":"x","key":"other","value":"y"}]}`,
		"missing values":      `{"schema":1,"operations":[{"kind":"toml.set_strings.v1","path":"settings.toml","key":"x"}]}`,
		"target traversal":    `{"schema":1,"operations":[{"kind":"toml.set_string.v1","path":"../outside.toml","key":"x","value":"y"}]}`,
		"reserved target":     `{"schema":1,"operations":[{"kind":"toml.set_string.v1","path":"addons.toml","key":"x","value":"y"}]}`,
		"wrong extension":     `{"schema":1,"operations":[{"kind":"go.ensure_import.v1","path":"settings.toml","import":"example.com/sourceedit/runtime","name":"_"}]}`,
		"missing import name": `{"schema":1,"operations":[{"kind":"go.ensure_import.v1","path":"app/services/service.go","import":"example.com/sourceedit/runtime"}]}`,
		"dot import":          `{"schema":1,"operations":[{"kind":"go.ensure_import.v1","path":"app/services/service.go","import":"example.com/sourceedit/runtime","name":"."}]}`,
		"trailing JSON":       `{"schema":1,"operations":[]} {}`,
	}
	for name, descriptor := range malformed {
		t.Run(name, func(t *testing.T) {
			app := fixtureApp(t, "v0.1.21")
			writeFile(t, app, "plugin/lazyaddon.toml", singleSourceEditManifest("broken", "edits/broken.json"))
			writeFile(t, app, "plugin/edits/broken.json", descriptor)
			if _, err := (addonservice.Manager{Dir: app, ManifestPaths: []string{"plugin/lazyaddon.toml"}}).Add("broken"); err == nil {
				t.Fatal("malformed descriptor error = nil")
			}
		})
	}

	unsafeManifest := strings.Replace(singleSourceEditManifest("broken", "edits/broken.json"), "edits/broken.json", "../broken.json", 1)
	if _, err := addonservice.ParsePackageManifest([]byte(unsafeManifest), "local:test"); err == nil {
		t.Fatal("descriptor traversal error = nil")
	}

	app := fixtureApp(t, "v0.1.21")
	writeFile(t, app, "plugin/lazyaddon.toml", singleSourceEditManifest("broken", "edits/link.json"))
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte(`{"schema":1,"operations":[{"kind":"toml.set_string.v1","path":"settings.toml","key":"x","value":"y"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(app, "plugin/edits"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(app, "plugin/edits/link.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := (addonservice.Manager{Dir: app, ManifestPaths: []string{"plugin/lazyaddon.toml"}}).Add("broken"); err == nil || !strings.Contains(err.Error(), "outside the manifest directory") {
		t.Fatalf("escaping descriptor symlink error = %v", err)
	}
}

func sourceEditManifest(shared bool) string {
	manifest := `manifest = 1

[package]
id = "example.com/sourceedit"
version = "v1.0.0"

[[addons]]
id = "alpha"
go_import = "example.com/sourceedit/alpha"
source_edits = ["edits/shared.json"]
`
	if shared {
		manifest += `
[[addons]]
id = "beta"
go_import = "example.com/sourceedit/beta"
source_edits = ["edits/shared.json"]
`
	}
	return manifest
}

func singleSourceEditManifest(id, descriptor string) string {
	return `manifest = 1

[package]
id = "example.com/sourceedit"
version = "v1.0.0"

[[addons]]
id = "` + id + `"
go_import = "example.com/sourceedit/addon"
source_edits = ["` + descriptor + `"]
`
}
