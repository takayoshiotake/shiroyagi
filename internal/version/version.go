package version

import (
	"fmt"
	"runtime/debug"
)

var (
	Version = "dev"
	Commit  = "unknown"
)

type Info struct {
	Version string
	Commit  string
}

func Get() Info {
	info := Info{
		Version: Version,
		Commit:  Commit,
	}

	if info.Commit == "unknown" {
		if buildInfo, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range buildInfo.Settings {
				if setting.Key == "vcs.revision" && setting.Value != "" {
					info.Commit = setting.Value
					break
				}
			}
		}
	}

	return info
}

func String() string {
	info := Get()
	return fmt.Sprintf("shiroyagi %s (%s)", info.Version, info.Commit)
}
