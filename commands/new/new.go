package newcommand

import "golazy.dev/lazy/services/scaffoldservice"

const sampleRepository = scaffoldservice.SampleRepository
const defaultLatestVersionURL = scaffoldservice.DefaultLatestVersionURL

type LatestVersionFetcher = scaffoldservice.LatestVersionFetcher
type Command = scaffoldservice.Command

func executableName(name string) string {
	return scaffoldservice.ExecutableName(name)
}

func resolveMiseCommand() (string, []string) {
	return scaffoldservice.ResolveMiseCommand()
}

func replaceSecureCookieKey(root string) error {
	return scaffoldservice.ReplaceSecureCookieKey(root)
}
