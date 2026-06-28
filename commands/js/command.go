package jscommand

import "golazy.dev/lazy/services/jsservice"

const DefaultEntrypointGroup = jsservice.DefaultEntrypointGroup

type Manifest = jsservice.Manifest
type OutputConfig = jsservice.OutputConfig
type BundleConfig = jsservice.BundleConfig
type Entrypoint = jsservice.Entrypoint
type BuildResult = jsservice.BuildResult
type Bundler = jsservice.Bundler
type Command = jsservice.Command
type ManifestEditor = jsservice.ManifestEditor
type ManifestCloseError = jsservice.ManifestCloseError

type importMap struct {
	Imports map[string]string `json:"imports"`
}

var LoadManifest = jsservice.LoadManifest
var ParseManifest = jsservice.ParseManifest
var FormatManifest = jsservice.FormatManifest
var ValidateManifest = jsservice.ValidateManifest
var Bundle = jsservice.Bundle
var PackageDir = jsservice.PackageDir
var OpenManifest = jsservice.OpenManifest

func findAppRoot(start string) (string, error) {
	return jsservice.FindAppRoot(start)
}

func defaultManifest() Manifest {
	return jsservice.DefaultManifest()
}

func ensurePackageDependencies(path string, packages []string) (bool, error) {
	return jsservice.EnsurePackageDependencies(path, packages)
}

func requiredPackages(manifest Manifest) []string {
	return jsservice.RequiredPackages(manifest)
}

func cloneManifest(manifest Manifest) Manifest {
	return jsservice.CloneManifest(manifest)
}
