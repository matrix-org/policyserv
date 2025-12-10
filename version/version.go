package version

import "runtime/debug"

var Revision string

func init() {
	if build, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range build.Settings {
			if setting.Key == "vcs.revision" {
				Revision = setting.Value
				return
			}
		}
	}

	Revision = "<unknown>"
}
