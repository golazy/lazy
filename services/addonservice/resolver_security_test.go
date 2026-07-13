package addonservice_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"golazy.dev/lazy/services/addonservice"
)

type mismatchedSource struct {
	addon addonservice.ResolvedAddon
}

type failingSource struct{}

func (failingSource) ID() string { return "marketplace:offline" }

func (failingSource) Resolve(context.Context, addonservice.Query) ([]addonservice.ResolvedAddon, error) {
	return nil, errors.New("registry unavailable")
}

func (mismatchedSource) ID() string { return "memory:mismatched" }

func (source mismatchedSource) Resolve(context.Context, addonservice.Query) ([]addonservice.ResolvedAddon, error) {
	return []addonservice.ResolvedAddon{source.addon}, nil
}

func TestSourceCannotSubstituteRequestedAddonIdentity(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	addon := memoryAddon("attacker", "v1.2.3")
	_, err := (addonservice.Manager{
		Dir:     dir,
		Sources: []addonservice.Source{mismatchedSource{addon: addon}},
	}).Add("seo@v1.2.3")
	if err == nil || !strings.Contains(err.Error(), `source returned add-on "attacker"`) {
		t.Fatalf("identity substitution error = %v", err)
	}
}

func TestSourceCannotSubstituteRequestedAddonVersion(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	addon := memoryAddon("seo", "v1.9.9")
	_, err := (addonservice.Manager{
		Dir:     dir,
		Sources: []addonservice.Source{mismatchedSource{addon: addon}},
	}).Add("seo@v1.2.3")
	if err == nil || !strings.Contains(err.Error(), `source returned version "v1.9.9"`) {
		t.Fatalf("version substitution error = %v", err)
	}
}

func TestCatalogBindsCandidateToActualSourceAndValidatesDigest(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	addon := memoryAddon("seo", "v1.2.3")
	addon.Source = "marketplace:spoofed"
	plan, err := (addonservice.Manager{
		Dir:     dir,
		Sources: []addonservice.Source{mismatchedSource{addon: addon}},
	}).Add("seo@v1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if got := plan.After.Addons[0].Source; got != "memory:mismatched" {
		t.Fatalf("locked source = %q", got)
	}

	addon.ManifestDigest = "sha256:not-a-digest"
	if _, err := (addonservice.Manager{
		Dir:     fixtureApp(t, "v0.1.21"),
		Sources: []addonservice.Source{mismatchedSource{addon: addon}},
	}).Add("seo@v1.2.3"); err == nil || !strings.Contains(err.Error(), "SHA-256 manifest digest") {
		t.Fatalf("invalid digest error = %v", err)
	}
}

func TestPinnedMutationFallsBackToLockedStateWhenSourceIsOffline(t *testing.T) {
	dir := fixtureApp(t, "v0.1.21")
	catalog := addonservice.MemorySource{Name: "memory:online", Addons: []addonservice.ResolvedAddon{
		memoryAddon("alpha", "v1.0.0"),
		memoryAddon("beta", "v1.0.0"),
	}}
	manager := addonservice.Manager{Dir: dir, Sources: []addonservice.Source{catalog}}
	for _, id := range []string{"alpha", "beta"} {
		plan, err := manager.Add(id)
		if err != nil {
			t.Fatal(err)
		}
		if err := plan.Apply(); err != nil {
			t.Fatal(err)
		}
	}

	remove, err := (addonservice.Manager{Dir: dir, Sources: []addonservice.Source{failingSource{}}}).Remove("alpha")
	if err != nil {
		t.Fatalf("remove with offline source: %v", err)
	}
	if got := installedVersion(remove.After, "beta"); got != "v1.0.0" {
		t.Fatalf("locked beta version = %q", got)
	}
	if err := remove.Apply(); err != nil {
		t.Fatal(err)
	}

	if _, err := (addonservice.Manager{Dir: dir, Sources: []addonservice.Source{failingSource{}}}).Update("beta"); err == nil || !strings.Contains(err.Error(), "registry unavailable") {
		t.Fatalf("unpinned update source error = %v", err)
	}
}
