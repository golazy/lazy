package addonservice_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
	"golazy.dev/lazy/services/addonservice"
	"golazy.dev/lazycode"
)

func TestManifestCarriesRuntimeJavaScriptAndBuildContributions(t *testing.T) {
	manifest, err := addonservice.ParsePackageManifest([]byte(testManifest("v0.1.21")), "memory:first-party")
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Addons) != 2 {
		t.Fatalf("add-ons = %d, want 2", len(manifest.Addons))
	}
	postgres := manifest.Addons[0]
	if postgres.GoImport != "golazy.dev/pg/addon/postgres" || postgres.GoModule != "golazy.dev/pg" || postgres.GoModuleVersion != "v0.1.21" {
		t.Fatalf("postgres Go contribution = %#v", postgres)
	}
	if got := strings.Join(postgres.Hooks, ","); got != "dependencies,migrations" {
		t.Fatalf("hooks = %q", got)
	}
	if got := strings.Join(postgres.JavaScript.Libraries, ","); got != "postgres-inspector@1.2.3" {
		t.Fatalf("JavaScript libraries = %q", got)
	}
	if got := strings.Join(postgres.Build.Tasks, ","); got != "postgres:check,postgres:create,postgres:reset,postgres:start" {
		t.Fatalf("build tasks = %q", got)
	}
}

func TestManifestRejectsUnsafeIdentityImportAndFileMetadata(t *testing.T) {
	base := `manifest = 1

[package]
id = "example.com/addons"
version = "v1.2.3"

[[addons]]
id = "safe"
go_import = "example.com/addons/safe"
go_module = "example.com/addons"
go_version = "v1.2.3"
`
	for name, source := range map[string]string{
		"identity": strings.Replace(base, `id = "safe"`, `id = "../escape"`, 1),
		"import":   strings.Replace(base, `go_import = "example.com/addons/safe"`, `go_import = "example.com/addons/safe bad"`, 1),
		"module":   strings.Replace(base, `go_module = "example.com/addons"`, `go_module = "example.com/other"`, 1),
		"file":     strings.Replace(base, `go_version = "v1.2.3"`, "go_version = \"v1.2.3\"\nfiles = [\"../escape\"]", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := addonservice.ParsePackageManifest([]byte(source), "memory:unsafe"); err == nil {
				t.Fatal("unsafe manifest error = nil")
			}
		})
	}
}

func TestAddPlansAndAppliesPinnedStateGeneratedImportsAndGoMod(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	manager := addonservice.Manager{Dir: dir, ManifestPaths: []string{"lazyaddon.toml"}}

	plan, err := manager.Add("postgres/jobs")
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Changed() {
		t.Fatal("plan has no changes")
	}
	if _, err := os.Stat(filepath.Join(dir, addonservice.StateFilename)); !os.IsNotExist(err) {
		t.Fatalf("dry plan wrote state: %v", err)
	}
	if err := plan.Apply(); err != nil {
		t.Fatal(err)
	}

	state, err := addonservice.ReadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Addons) != 2 || len(state.Modules) != 2 {
		t.Fatalf("state = %#v", state)
	}
	if state.Addons[0].ID != "postgres" || state.Addons[0].Direct || state.Addons[0].Version != "v0.1.21" {
		t.Fatalf("required postgres = %#v", state.Addons[0])
	}
	if state.Addons[1].ID != "postgres/jobs" || !state.Addons[1].Direct || state.Addons[1].PackageVersion != "v0.1.21" {
		t.Fatalf("direct jobs = %#v", state.Addons[1])
	}
	if module := managedModule(state, "golazy.dev/pg"); module.Path != "golazy.dev/pg" || module.Version != "v0.1.21" || !module.Managed || len(module.Owners) != 2 {
		t.Fatalf("module lock = %#v", module)
	}
	if module := managedModule(state, "golazy.dev"); module.Version != "v0.1.21" || module.Previous != "v0.1.20" || !module.Managed {
		t.Fatalf("framework module lock = %#v", module)
	}

	generated := readFile(t, dir, addonservice.GeneratedFilename)
	for _, expected := range []string{
		`_ "golazy.dev/pg/addon/postgres/jobs"`,
		`_ "golazy.dev/pg/addon/postgres"`,
		`"postgres/jobs",`,
		`event.Catalog.Mount("postgres", "application", appPostgres.Files())`,
	} {
		if !strings.Contains(generated, expected) {
			t.Fatalf("generated file does not contain %q:\n%s", expected, generated)
		}
	}
	application := readFile(t, dir, addonservice.ApplicationFilename)
	if !strings.Contains(application, "Addons: installedAddons()") {
		t.Fatalf("application wiring =\n%s", application)
	}
	goMod := readFile(t, dir, addonservice.GoModFilename)
	parsed, err := modfile.Parse("go.mod", []byte(goMod), nil)
	if err != nil {
		t.Fatal(err)
	}
	if version := requirementVersion(parsed, "golazy.dev/pg"); version != "v0.1.21" {
		t.Fatalf("golazy.dev/pg version = %q", version)
	}
	for _, generatedPath := range []string{
		"db/postgres/migrations.go",
		"db/postgres/migrations/migrations.toml",
		".mise/tasks/postgres/start",
		".mise/tasks/postgres/check",
		".mise/tasks/postgres/create",
		".mise/tasks/postgres/reset",
	} {
		info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(generatedPath)))
		if err != nil {
			t.Fatalf("generated %s: %v", generatedPath, err)
		}
		if strings.HasPrefix(generatedPath, ".mise/tasks/") && info.Mode().Perm() != 0o755 {
			t.Fatalf("task %s mode = %o, want 755", generatedPath, info.Mode().Perm())
		}
	}
	if lazyConfig := readFile(t, dir, "lazy.toml"); !strings.Contains(lazyConfig, `services = ["postgres"]`) {
		t.Fatalf("lazy.toml =\n%s", lazyConfig)
	}
	miseConfig := readFile(t, dir, "mise.toml")
	for _, expected := range []string{
		`postgres = "17"`,
		`DB_DATA = ".tmp/postgres"`,
		`DB_NAME = "app_development"`,
		`DB_PORT = "5432"`,
		`DATABASE_URL = "postgres://{{env.DB_USER}}@127.0.0.1:{{env.DB_PORT}}/{{env.DB_NAME}}?sslmode=disable"`,
	} {
		if !strings.Contains(miseConfig, expected) {
			t.Fatalf("mise.toml does not contain %q:\n%s", expected, miseConfig)
		}
	}

	idempotent, err := manager.Add("postgres/jobs")
	if err != nil {
		t.Fatal(err)
	}
	if idempotent.Changed() {
		t.Fatalf("idempotent plan = %#v", idempotent.Result.Files)
	}
}

func TestAddAcceptsLocalApplicationModulePath(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	writeFile(t, dir, "go.mod", "module sample_app\n\ngo 1.26.2\n\nrequire golazy.dev v0.1.20\n")
	plan, err := (addonservice.Manager{Dir: dir, ManifestPaths: []string{"lazyaddon.toml"}}).Add("postgres")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readPlannedFile(plan, addonservice.GeneratedFilename), `sample_app/db/postgres`) {
		t.Fatalf("generated local module wiring missing from plan: %#v", plan.Result.Files)
	}
}

func TestAddConfiguredLocksAndGeneratesRuntimeConfiguration(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	manager := addonservice.Manager{Dir: dir, ManifestPaths: []string{"lazyaddon.toml"}}
	install, err := manager.AddConfigured("postgres", map[string]string{
		"database_url_variable": "APP_DATABASE_URL",
		"region":                "eu=central",
	})
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
	if got := state.Addons[0].Config["database_url_variable"]; got != "APP_DATABASE_URL" {
		t.Fatalf("database URL variable = %q", got)
	}
	if got := state.Addons[0].Config["region"]; got != "eu=central" {
		t.Fatalf("region = %q", got)
	}
	generated := readFile(t, dir, addonservice.GeneratedFilename)
	for _, expected := range []string{
		`"database_url_variable": "APP_DATABASE_URL"`,
		`"region": "eu=central"`,
	} {
		if !strings.Contains(generated, expected) {
			t.Fatalf("generated runtime configuration missing %q:\n%s", expected, generated)
		}
	}

	reconfigure, err := (addonservice.Manager{Dir: dir}).AddConfigured("postgres", map[string]string{"region": "us-east"})
	if err != nil {
		t.Fatal(err)
	}
	if err := reconfigure.Apply(); err != nil {
		t.Fatal(err)
	}
	state, err = addonservice.ReadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := state.Addons[0].Config["database_url_variable"]; got != "APP_DATABASE_URL" {
		t.Fatalf("unmentioned configuration was not preserved: %q", got)
	}
	if got := state.Addons[0].Config["region"]; got != "us-east" {
		t.Fatalf("updated region = %q", got)
	}
	unset, err := (addonservice.Manager{Dir: dir}).AddWithOptions("postgres", addonservice.AddOptions{UnsetConfig: []string{"region"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := unset.Apply(); err != nil {
		t.Fatal(err)
	}
	state, err = addonservice.ReadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := state.Addons[0].Config["region"]; exists {
		t.Fatalf("unset configuration remains: %#v", state.Addons[0].Config)
	}
	update, err := (addonservice.Manager{Dir: dir}).Update()
	if err != nil {
		t.Fatal(err)
	}
	if got := update.After.Addons[0].Config["database_url_variable"]; got != "APP_DATABASE_URL" {
		t.Fatalf("update lost configuration: %q", got)
	}
}

func TestAddConfiguredRejectsInvalidConfigurationKey(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	_, err := (addonservice.Manager{Dir: dir}).AddConfigured("postgres", map[string]string{"bad key": "value"})
	if err == nil || !strings.Contains(err.Error(), "configuration key") {
		t.Fatalf("invalid configuration error = %v", err)
	}
}

func TestUpdateUsesRecordedLocalSourceAndAdvancesExactPins(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	manager := addonservice.Manager{Dir: dir, ManifestPaths: []string{"lazyaddon.toml"}}
	install, err := manager.Add("postgres")
	if err != nil {
		t.Fatal(err)
	}
	if err := install.Apply(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "lazyaddon.toml", testManifest("v0.1.22"))

	update, err := (addonservice.Manager{Dir: dir}).Update()
	if err != nil {
		t.Fatal(err)
	}
	if update.After.Addons[0].Version != "v0.1.22" || update.After.Addons[0].GoModuleVersion != "v0.1.22" {
		t.Fatalf("updated add-on = %#v", update.After.Addons[0])
	}
	if err := update.Apply(); err != nil {
		t.Fatal(err)
	}
	if got := requirementVersion(mustParseGoMod(t, readFile(t, dir, "go.mod")), "golazy.dev/pg"); got != "v0.1.22" {
		t.Fatalf("updated module version = %q", got)
	}
}

func TestAddPreservesNewerApplicationFrameworkRequirement(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	writeFile(t, dir, "go.mod", "module example.com/app\n\ngo 1.26.2\n\nrequire golazy.dev v0.1.30\n")
	manager := addonservice.Manager{
		Dir:              dir,
		ManifestPaths:    []string{"lazyaddon.toml"},
		FrameworkVersion: "v0.1.21",
	}

	install, err := manager.Add("postgres")
	if err != nil {
		t.Fatal(err)
	}
	framework := managedModule(install.After, "golazy.dev")
	if framework.Version != "v0.1.30" || framework.Managed {
		t.Fatalf("framework lock = %#v, want preserved app-owned v0.1.30", framework)
	}
	if err := install.Apply(); err != nil {
		t.Fatal(err)
	}
	if got := requirementVersion(mustParseGoMod(t, readFile(t, dir, "go.mod")), "golazy.dev"); got != "v0.1.30" {
		t.Fatalf("framework version = %q, want v0.1.30", got)
	}
}

func TestUpdatedAppOwnedModuleBecomesManagedAndRestoresOnRemove(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	if err := os.Remove(filepath.Join(dir, "lazyaddon.toml")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "go.mod", `module example.com/app

go 1.26.2

require (
	example.com/addons/seo v1.0.0
	golazy.dev v0.1.20
)
`)
	oldAddon := memoryAddon("seo", "v1.0.0")
	manager := addonservice.Manager{
		Dir:     dir,
		Sources: []addonservice.Source{addonservice.MemorySource{Name: "memory:module-ownership", Addons: []addonservice.ResolvedAddon{oldAddon}}},
	}
	install, err := manager.Add("seo@v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if module := managedModule(install.After, oldAddon.GoModule); module.Managed {
		t.Fatalf("initial matching app requirement became installer-managed: %#v", module)
	}
	if err := install.Apply(); err != nil {
		t.Fatal(err)
	}

	newAddon := memoryAddon("seo", "v1.1.0")
	manager.Sources = []addonservice.Source{addonservice.MemorySource{
		Name:   "memory:module-ownership",
		Addons: []addonservice.ResolvedAddon{oldAddon, newAddon},
	}}
	update, err := manager.Update("seo")
	if err != nil {
		t.Fatal(err)
	}
	module := managedModule(update.After, oldAddon.GoModule)
	if !module.Managed || module.Previous != "v1.0.0" || module.Version != "v1.1.0" {
		t.Fatalf("updated module ownership = %#v", module)
	}
	if err := update.Apply(); err != nil {
		t.Fatal(err)
	}

	remove, err := manager.Remove("seo")
	if err != nil {
		t.Fatal(err)
	}
	if err := remove.Apply(); err != nil {
		t.Fatal(err)
	}
	if got := requirementVersion(mustParseGoMod(t, readFile(t, dir, "go.mod")), oldAddon.GoModule); got != "v1.0.0" {
		t.Fatalf("restored app-owned module version = %q, want v1.0.0", got)
	}
}

func TestUpdateRejectsChangedManifestForLockedReleaseVersion(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	manager := addonservice.Manager{Dir: dir, ManifestPaths: []string{"lazyaddon.toml"}}
	install, err := manager.Add("postgres")
	if err != nil {
		t.Fatal(err)
	}
	if err := install.Apply(); err != nil {
		t.Fatal(err)
	}

	manifest := strings.Replace(testManifest("v0.1.21"), "PostgreSQL services", "Changed after publishing", 1)
	writeFile(t, dir, "lazyaddon.toml", manifest)
	if _, err := (addonservice.Manager{Dir: dir}).Update(); err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("immutable release error = %v", err)
	}
}

func TestSelectiveUpdateOnlyUnpinsRequestedDependencyClosure(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	if err := os.Remove(filepath.Join(dir, "lazyaddon.toml")); err != nil {
		t.Fatal(err)
	}
	old := []addonservice.ResolvedAddon{
		memoryAddon("alpha", "v1.0.0", "shared-a"),
		memoryAddon("shared-a", "v1.0.0"),
		memoryAddon("beta", "v1.0.0", "shared-b"),
		memoryAddon("shared-b", "v1.0.0"),
	}
	manager := addonservice.Manager{Dir: dir, Sources: []addonservice.Source{addonservice.MemorySource{Name: "memory:updates", Addons: old}}}
	install, err := manager.Add("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if err := install.Apply(); err != nil {
		t.Fatal(err)
	}
	install, err = manager.Add("beta")
	if err != nil {
		t.Fatal(err)
	}
	if err := install.Apply(); err != nil {
		t.Fatal(err)
	}

	newCatalog := append([]addonservice.ResolvedAddon(nil), old...)
	newCatalog = append(newCatalog,
		memoryAddon("alpha", "v1.1.0", "shared-a"),
		memoryAddon("shared-a", "v1.1.0"),
		memoryAddon("beta", "v1.1.0", "shared-b"),
		memoryAddon("shared-b", "v1.1.0"),
	)
	update, err := (addonservice.Manager{
		Dir:     dir,
		Sources: []addonservice.Source{addonservice.MemorySource{Name: "memory:updates", Addons: newCatalog}},
	}).Update("alpha")
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{
		"alpha": "v1.1.0", "shared-a": "v1.1.0",
		"beta": "v1.0.0", "shared-b": "v1.0.0",
	} {
		if got := installedVersion(update.After, id); got != want {
			t.Fatalf("%s version = %q, want %q", id, got, want)
		}
	}
}

func TestDependencyRequirementPinsCompatibleVersion(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	if err := os.Remove(filepath.Join(dir, "lazyaddon.toml")); err != nil {
		t.Fatal(err)
	}
	feature := memoryAddon("feature", "v1.0.0", "base@v1.0.0")
	catalog := []addonservice.ResolvedAddon{
		feature,
		memoryAddon("base", "v1.0.0"),
		memoryAddon("base", "v2.0.0"),
	}
	plan, err := (addonservice.Manager{
		Dir:     dir,
		Sources: []addonservice.Source{addonservice.MemorySource{Name: "memory:constraints", Addons: catalog}},
	}).Add("feature@v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if got := installedVersion(plan.After, "base"); got != "v1.0.0" {
		t.Fatalf("resolved base version = %q, want v1.0.0", got)
	}
}

func TestAddRejectsUnmanagedGeneratedFile(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	original := "package appinit\n\nfunc userOwned() {}\n"
	writeFile(t, dir, addonservice.GeneratedFilename, original)

	_, err := (addonservice.Manager{Dir: dir, ManifestPaths: []string{"lazyaddon.toml"}}).Add("postgres")
	if err == nil || !strings.Contains(err.Error(), "not recognized") {
		t.Fatalf("unmanaged generated-file error = %v", err)
	}
	if got := readFile(t, dir, addonservice.GeneratedFilename); got != original {
		t.Fatalf("unmanaged generated file changed:\n%s", got)
	}
}

func TestApplicationRewriteTargetsConstructorConfigWithAlias(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	writeFile(t, dir, addonservice.ApplicationFilename, `package appinit

import frameworkapp "golazy.dev/lazyapp"

var unrelated = frameworkapp.Config{Name: "unrelated"}

func App() *frameworkapp.App {
	config := frameworkapp.Config{Name: "app"}
	return frameworkapp.New(config)
}
`)
	plan, err := (addonservice.Manager{Dir: dir, ManifestPaths: []string{"lazyaddon.toml"}}).Add("postgres")
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	application := readFile(t, dir, addonservice.ApplicationFilename)
	if strings.Count(application, "Addons: installedAddons()") != 1 {
		t.Fatalf("application wiring =\n%s", application)
	}
	unrelatedAt := strings.Index(application, `Name: "unrelated"`)
	appAt := strings.Index(application, `Name: "app"`)
	addonsAt := strings.Index(application, "Addons: installedAddons()")
	if unrelatedAt < 0 || appAt < 0 || addonsAt < appAt || addonsAt < unrelatedAt {
		t.Fatalf("constructor-bound config was not edited:\n%s", application)
	}
}

func TestApplicationRewriteRejectsAmbiguousConstructors(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	writeFile(t, dir, addonservice.ApplicationFilename, `package appinit

import "golazy.dev/lazyapp"

func App() *lazyapp.App {
	if useFirst() {
		return lazyapp.New(lazyapp.Config{Name: "first"})
	}
	return lazyapp.New(lazyapp.Config{Name: "second"})
}
`)
	_, err := (addonservice.Manager{Dir: dir, ManifestPaths: []string{"lazyaddon.toml"}}).Add("postgres")
	if err == nil || !strings.Contains(err.Error(), "exactly one lazyapp.New call") {
		t.Fatalf("ambiguous constructor error = %v", err)
	}
}

func TestRemoveUsesLockedSnapshotRestoresGoModAndProtectsGeneratedDrift(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	manager := addonservice.Manager{Dir: dir, ManifestPaths: []string{"lazyaddon.toml"}}
	install, err := manager.Add("postgres/jobs")
	if err != nil {
		t.Fatal(err)
	}
	if err := install.Apply(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "lazyaddon.toml")); err != nil {
		t.Fatal(err)
	}
	startTask := readFile(t, dir, ".mise/tasks/postgres/start")
	writeFile(t, dir, ".mise/tasks/postgres/start", startTask+"# local change\n")
	if _, err := (addonservice.Manager{Dir: dir}).Remove("postgres/jobs"); err == nil || !strings.Contains(err.Error(), "local changes") {
		t.Fatalf("task drift error = %v", err)
	}
	writeFile(t, dir, ".mise/tasks/postgres/start", startTask)
	if err := os.Chmod(filepath.Join(dir, ".mise/tasks/postgres/start"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "lazy.toml", readFile(t, dir, "lazy.toml")+"\n[tmux]\nsession = \"kept\"\n")
	writeFile(t, dir, "mise.toml", strings.Replace(readFile(t, dir, "mise.toml"), `DB_PORT = "5432"`, `DB_PORT = "5544"`, 1))

	remove, err := (addonservice.Manager{Dir: dir}).Remove("postgres/jobs")
	if err != nil {
		t.Fatal(err)
	}
	if err := remove.Apply(); err != nil {
		t.Fatal(err)
	}
	state, err := addonservice.ReadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Addons) != 0 || len(state.Modules) != 0 {
		t.Fatalf("state after remove = %#v", state)
	}
	for _, removedPath := range []string{addonservice.StateFilename, addonservice.GeneratedFilename} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(removedPath))); !os.IsNotExist(err) {
			t.Fatalf("removed path %s still exists: %v", removedPath, err)
		}
	}
	application := readFile(t, dir, addonservice.ApplicationFilename)
	if strings.Contains(application, "Addons:") || !strings.Contains(application, `Name: "app"`) {
		t.Fatalf("application wiring after remove =\n%s", application)
	}
	if got := requirementVersion(mustParseGoMod(t, readFile(t, dir, "go.mod")), "golazy.dev/pg"); got != "" {
		t.Fatalf("restored module version = %q, want absent", got)
	}
	for _, removedPath := range []string{
		"db/postgres/migrations.go",
		"db/postgres/migrations/migrations.toml",
		".mise/tasks/postgres/start",
		".mise/tasks/postgres/check",
		".mise/tasks/postgres/create",
		".mise/tasks/postgres/reset",
	} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(removedPath))); !os.IsNotExist(err) {
			t.Fatalf("removed path %s still exists: %v", removedPath, err)
		}
	}
	if lazyConfig := readFile(t, dir, "lazy.toml"); strings.Contains(lazyConfig, "postgres") || !strings.Contains(lazyConfig, `session = "kept"`) {
		t.Fatalf("lazy.toml after remove =\n%s", lazyConfig)
	}
	if miseConfig := readFile(t, dir, "mise.toml"); !strings.Contains(miseConfig, `DB_PORT = "5544"`) || strings.Contains(miseConfig, "DATABASE_URL") || strings.Contains(miseConfig, "DB_NAME") {
		t.Fatalf("mise.toml after remove =\n%s", miseConfig)
	}

	writeFile(t, dir, "lazyaddon.toml", testManifest("v0.1.21"))
	reinstall, err := (addonservice.Manager{Dir: dir}).Add("postgres")
	if err != nil {
		t.Fatal(err)
	}
	if err := reinstall.Apply(); err != nil {
		t.Fatal(err)
	}
	generatedPath := filepath.Join(dir, filepath.FromSlash(addonservice.GeneratedFilename))
	if err := os.WriteFile(generatedPath, []byte("package appinit\n// local edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (addonservice.Manager{Dir: dir}).Remove("postgres"); err == nil || !strings.Contains(err.Error(), "local changes") {
		t.Fatalf("drift error = %v", err)
	}
}

func TestPostgresMiseContributionPreservesPreexistingValuesAndOwnership(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	writeFile(t, dir, "mise.toml", "[tools]\npostgres = \"16\"\n\n[env]\nDB_USER = \"developer\"\nDB_PORT = \"5544\"\n")
	manager := addonservice.Manager{Dir: dir, ManifestPaths: []string{"lazyaddon.toml"}}
	install, err := manager.Add("postgres")
	if err != nil {
		t.Fatal(err)
	}
	if err := install.Apply(); err != nil {
		t.Fatal(err)
	}
	mise := readFile(t, dir, "mise.toml")
	if !strings.Contains(mise, `DB_USER = "developer"`) || !strings.Contains(mise, `DB_PORT = "5544"`) {
		t.Fatalf("preexisting mise values changed:\n%s", mise)
	}
	if !strings.Contains(mise, `postgres = "16"`) {
		t.Fatalf("preexisting postgres tool changed:\n%s", mise)
	}
	if !strings.Contains(mise, `DATABASE_URL = "postgres://{{env.DB_USER}}@127.0.0.1:{{env.DB_PORT}}/{{env.DB_NAME}}?sslmode=disable"`) {
		t.Fatalf("DATABASE_URL did not use preserved values:\n%s", mise)
	}
	remove, err := manager.Remove("postgres")
	if err != nil {
		t.Fatal(err)
	}
	if err := remove.Apply(); err != nil {
		t.Fatal(err)
	}
	mise = readFile(t, dir, "mise.toml")
	if !strings.Contains(mise, `postgres = "16"`) || !strings.Contains(mise, `DB_USER = "developer"`) || !strings.Contains(mise, `DB_PORT = "5544"`) || strings.Contains(mise, "DATABASE_URL") {
		t.Fatalf("mise ownership removal =\n%s", mise)
	}
}

func TestStateRoundTripIsDeterministicAndOmitsEmptyContributionFields(t *testing.T) {
	state := addonservice.State{
		Manifest: 1,
		Addons: []addonservice.InstalledAddon{{
			ResolvedAddon: addonservice.ResolvedAddon{
				ID: "seo", Version: "v1.2.3", Package: "example.com/seo", PackageVersion: "v1.2.3",
				ManifestDigest: lazycode.Hash([]byte("seo manifest")), Source: "market:official", GoImport: "example.com/seo/addon",
				GoModule: "example.com/seo", GoModuleVersion: "v1.2.3",
			},
			Direct: true,
			Config: map[string]string{"zeta": "last", "alpha": "contains=equals"},
		}},
		Modules: []addonservice.ManagedModule{{Path: "example.com/seo", Version: "v1.2.3", Owners: []string{"seo"}, Managed: true}},
		Managed: []addonservice.ManagedFile{{Path: "init/addons_gen.go", Kind: "generated", Owners: []string{"seo"}, Baseline: "sha256:test"}},
	}
	first, err := addonservice.MarshalState(state)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := addonservice.ParseState(first)
	if err != nil {
		t.Fatal(err)
	}
	second, err := addonservice.MarshalState(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("state is not deterministic:\n%s\n---\n%s", first, second)
	}
	if strings.Contains(string(first), "js_libraries") || strings.Contains(string(first), "build_hooks") || strings.Contains(string(first), "description") {
		t.Fatalf("empty contribution fields were rendered:\n%s", first)
	}
	if !strings.Contains(string(first), `config = ["alpha=contains=equals", "zeta=last"]`) {
		t.Fatalf("configuration was not rendered in stable order:\n%s", first)
	}
}

func fixtureApp(t *testing.T, version string) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module example.com/app\n\ngo 1.26.2\n\nrequire golazy.dev v0.1.20\n")
	writeFile(t, dir, "init/app.go", `package appinit

import "golazy.dev/lazyapp"

func App() *lazyapp.App {
	return lazyapp.New(lazyapp.Config{Name: "app"})
}
`)
	writeFile(t, dir, "lazyaddon.toml", testManifest(version))
	return dir
}

func testManifest(version string) string {
	return `manifest = 1

[package]
id = "golazy.dev/pg"
version = "` + version + `"

[[addons]]
id = "postgres"
description = "PostgreSQL services"
go_import = "golazy.dev/pg/addon/postgres"
go_module = "golazy.dev/pg"
go_version = "` + version + `"
hooks = ["dependencies", "migrations"]
js_libraries = ["postgres-inspector@1.2.3"]
js_entrypoints = ["postgres_inspector=postgres-inspector"]
tasks = ["postgres:start", "postgres:check", "postgres:create", "postgres:reset"]
files = [".mise/tasks/postgres/start", ".mise/tasks/postgres/check", ".mise/tasks/postgres/create", ".mise/tasks/postgres/reset", "db/postgres/migrations.go", "db/postgres/migrations/migrations.toml", "lazy.toml", "mise.toml"]

[[addons]]
id = "postgres/jobs"
requires = ["postgres"]
go_import = "golazy.dev/pg/addon/postgres/jobs"
go_module = "golazy.dev/pg"
go_version = "` + version + `"
hooks = ["jobs", "migrations"]
`
}

func memoryAddon(id, version string, requires ...string) addonservice.ResolvedAddon {
	modulePath := "example.com/addons/" + id
	for index, requirement := range requires {
		if !strings.Contains(requirement, "@") {
			requires[index] = requirement + "@" + version
		}
	}
	return addonservice.ResolvedAddon{
		ID:              id,
		Version:         version,
		Requires:        requires,
		Package:         modulePath,
		PackageVersion:  version,
		ManifestDigest:  lazycode.Hash([]byte(id + "@" + version)),
		GoImport:        modulePath + "/addon",
		GoModule:        modulePath,
		GoModuleVersion: version,
	}
}

func installedVersion(state addonservice.State, id string) string {
	for _, addon := range state.Addons {
		if addon.ID == id {
			return addon.Version
		}
	}
	return ""
}

func managedModule(state addonservice.State, modulePath string) addonservice.ManagedModule {
	for _, module := range state.Modules {
		if module.Path == modulePath {
			return module
		}
	}
	return addonservice.ManagedModule{}
}

func writeFile(t *testing.T, root, name, data string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, root, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func readPlannedFile(plan addonservice.Plan, name string) string {
	for _, edit := range plan.Result.Files {
		if edit.Path == name {
			return string(edit.After)
		}
	}
	return ""
}

func mustParseGoMod(t *testing.T, data string) *modfile.File {
	t.Helper()
	file, err := modfile.Parse("go.mod", []byte(data), nil)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func requirementVersion(file *modfile.File, path string) string {
	for _, requirement := range file.Require {
		if requirement.Mod.Path == path {
			return requirement.Mod.Version
		}
	}
	return ""
}
