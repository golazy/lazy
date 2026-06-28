package tailwindcommand

import "golazy.dev/lazy/services/tailwindservice"

type Command = tailwindservice.Command

var requiredPackages = tailwindservice.RequiredPackages()

func ensurePackageDevDependencies(path string, packages []string) (bool, error) {
	return tailwindservice.EnsurePackageDevDependencies(path, packages)
}
