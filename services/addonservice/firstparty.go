package addonservice

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
)

// firstPartyPGManifest is the CLI's offline distribution snapshot of
// golazy/pg/lazyaddon.toml. The root version sync/check flow owns exact parity.
//
//go:embed firstparty_pg.lazyaddon.toml
var firstPartyPGManifest string

type firstPartySource struct{ version string }

// FirstPartySource returns add-ons distributed with the running lazy CLI. Its
// exact version is the CLI version so a zero-configuration install remains
// reproducible and upgrades are explicit through lazy update.
func FirstPartySource(version string) Source { return firstPartySource{version: version} }

func (source firstPartySource) ID() string { return "first-party:" + source.version }

func (source firstPartySource) Resolve(ctx context.Context, query Query) ([]ResolvedAddon, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data := []byte(strings.ReplaceAll(firstPartyPGManifest, "${VERSION}", source.version))
	manifest, err := ParsePackageManifest(data, source.ID())
	if err != nil {
		return nil, fmt.Errorf("parse compiled first-party manifest: %w", err)
	}
	var matches []ResolvedAddon
	for _, addon := range manifest.Addons {
		if addon.ID == query.ID && (query.Version == "" || addon.Version == query.Version) {
			matches = append(matches, addon)
		}
	}
	return matches, nil
}
