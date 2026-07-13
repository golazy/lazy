package addonservice

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/mod/semver"
)

// Catalog combines local, compiled first-party, and future remote sources.
// Earlier sources win when two sources expose the same exact version.
type Catalog struct {
	Sources []Source
}

type provenancePreservingSource interface {
	preserveResolvedProvenance()
}

func (catalog Catalog) Resolve(ctx context.Context, query Query) (ResolvedAddon, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	query.ID = strings.TrimSpace(query.ID)
	query.Version = strings.TrimSpace(query.Version)
	if query.ID == "" {
		return ResolvedAddon{}, fmt.Errorf("add-on ID is required")
	}
	if query.Version != "" && !exactVersion(query.Version) {
		return ResolvedAddon{}, fmt.Errorf("add-on %q requested non-exact version %q", query.ID, query.Version)
	}
	var candidates []ResolvedAddon
	var sourceErrors []error
	for _, source := range catalog.Sources {
		if source == nil {
			continue
		}
		sourceID := strings.TrimSpace(source.ID())
		if sourceID == "" {
			return ResolvedAddon{}, fmt.Errorf("resolve %s: source ID is required", query.ID)
		}
		if err := ctx.Err(); err != nil {
			return ResolvedAddon{}, err
		}
		resolved, err := source.Resolve(ctx, query)
		if err != nil {
			sourceErrors = append(sourceErrors, fmt.Errorf("resolve %s from %s: %w", query.ID, sourceID, err))
			continue
		}
		for _, addon := range resolved {
			_, lockedSnapshot := source.(lockedSnapshotSource)
			if err := validateSourceArtifacts(addon, lockedSnapshot); err != nil {
				return ResolvedAddon{}, fmt.Errorf("resolve %s from %s: %w", query.ID, sourceID, err)
			}
			if _, preserve := source.(provenancePreservingSource); !preserve {
				addon.Source = sourceID
			}
			normalizeResolved(&addon)
			if err := validateResolved(addon); err != nil {
				return ResolvedAddon{}, fmt.Errorf("resolve %s from %s: %w", query.ID, sourceID, err)
			}
			if addon.ID != query.ID {
				return ResolvedAddon{}, fmt.Errorf("resolve %s from %s: source returned add-on %q", query.ID, sourceID, addon.ID)
			}
			if query.Version != "" && addon.Version != query.Version {
				return ResolvedAddon{}, fmt.Errorf("resolve %s@%s from %s: source returned version %q", query.ID, query.Version, sourceID, addon.Version)
			}
			candidates = append(candidates, addon)
		}
	}
	// Exact pinned queries may fall back to the immutable addons.toml snapshot
	// when a network or local package source is unavailable. Unpinned queries
	// are updates/discovery and must report source failures instead of silently
	// pretending the locked release is still the newest.
	if len(sourceErrors) != 0 && query.Version == "" {
		return ResolvedAddon{}, errors.Join(sourceErrors...)
	}
	if len(candidates) == 0 {
		if len(sourceErrors) != 0 {
			return ResolvedAddon{}, errors.Join(sourceErrors...)
		}
		if query.Version != "" {
			return ResolvedAddon{}, fmt.Errorf("add-on %q version %q was not found", query.ID, query.Version)
		}
		return ResolvedAddon{}, fmt.Errorf("add-on %q was not found", query.ID)
	}
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if compareVersions(candidate.Version, best.Version) > 0 {
			best = candidate
		}
	}
	return best, nil
}

// StateSource lets add/remove remain reproducible from addons.toml even when a
// local manifest is not supplied again. Update adds the recorded local or
// remote source ahead of this locked snapshot when one is available.
type StateSource struct{ State State }

func (StateSource) ID() string { return "locked:addons.toml" }

func (StateSource) preserveResolvedProvenance() {}

func (StateSource) allowMissingLockedArtifacts() {}

func (source StateSource) Resolve(ctx context.Context, query Query) ([]ResolvedAddon, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var matches []ResolvedAddon
	for _, installed := range source.State.Addons {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if installed.ID != query.ID || query.Version != "" && installed.Version != query.Version {
			continue
		}
		matches = append(matches, installed.ResolvedAddon)
	}
	return matches, nil
}

func resolveSelection(ctx context.Context, catalog Catalog, direct []Query, pinned map[string]string, configs map[string]map[string]string) ([]InstalledAddon, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	directIDs := make(map[string]bool, len(direct))
	queries := make(map[string]Query, len(direct))
	for _, query := range direct {
		query.ID = strings.TrimSpace(query.ID)
		query.Version = strings.TrimSpace(query.Version)
		if query.ID == "" {
			return nil, fmt.Errorf("direct add-on ID is required")
		}
		if directIDs[query.ID] {
			return nil, fmt.Errorf("add-on %q is selected more than once", query.ID)
		}
		directIDs[query.ID] = true
		queries[query.ID] = query
	}

	state := map[string]uint8{}
	resolved := map[string]ResolvedAddon{}
	var order []string
	var visit func(Query, []string) error
	visit = func(query Query, trail []string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if existing, ok := resolved[query.ID]; ok && query.Version != "" && existing.Version != query.Version {
			owner := "selection"
			if len(trail) != 0 {
				owner = trail[len(trail)-1]
			}
			return fmt.Errorf("%s requires add-on %q at version %q, but version %q is already selected", owner, query.ID, query.Version, existing.Version)
		}
		if state[query.ID] == 1 {
			return fmt.Errorf("add-on dependency cycle: %s", strings.Join(append(trail, query.ID), " -> "))
		}
		if state[query.ID] == 2 {
			return nil
		}
		if query.Version == "" {
			query.Version = pinned[query.ID]
		}
		addon, err := catalog.Resolve(ctx, query)
		if err != nil {
			return err
		}
		state[query.ID] = 1
		resolved[query.ID] = addon
		for _, requirement := range addon.Requires {
			required, version, err := ParseSpec(requirement)
			if err != nil {
				return fmt.Errorf("add-on %q requirement %q: %w", addon.ID, requirement, err)
			}
			if err := visit(Query{ID: required, Version: version}, append(trail, query.ID)); err != nil {
				return err
			}
		}
		state[query.ID] = 2
		order = append(order, query.ID)
		return nil
	}
	ids := make([]string, 0, len(queries))
	for id := range queries {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := visit(queries[id], nil); err != nil {
			return nil, err
		}
	}
	for id, addon := range resolved {
		for _, conflict := range addon.Conflicts {
			if _, exists := resolved[conflict]; exists {
				return nil, fmt.Errorf("add-on %q conflicts with %q", id, conflict)
			}
		}
	}
	installed := make([]InstalledAddon, 0, len(order))
	for _, id := range order {
		installed = append(installed, InstalledAddon{ResolvedAddon: resolved[id], Direct: directIDs[id], Config: cloneConfig(configs[id])})
	}
	return installed, nil
}

func compareVersions(left, right string) int {
	if semver.IsValid(left) && semver.IsValid(right) {
		return semver.Compare(left, right)
	}
	return strings.Compare(left, right)
}
