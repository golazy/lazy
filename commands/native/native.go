package native

import "golazy.dev/lazy/services/nativeservice"

type Command = nativeservice.Command

func helperName(goos string) string {
	return nativeservice.HelperName(goos)
}
