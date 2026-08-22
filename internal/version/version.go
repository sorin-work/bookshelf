// Package version reports which build is running.
package version

import (
	"runtime/debug"
	"sync"
)

// Version is set at build time with
// -ldflags "-X github.com/sorin-work/bookshelf/internal/version.Version=1.2.3".
var Version = ""

var (
	once   sync.Once
	cached string
)

// String returns the release version if one was set, otherwise the short
// git commit the binary was built from, otherwise "dev".
func String() string {
	once.Do(func() {
		cached = compute()
	})
	return cached
}

func compute() string {
	if Version != "" {
		return Version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	rev, dirty := "", false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if len(rev) < 7 {
		return "dev"
	}
	rev = rev[:7]
	if dirty {
		rev += "-dirty"
	}
	return rev
}
