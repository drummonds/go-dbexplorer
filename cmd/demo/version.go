// Build version reporting shared by the native and WASM demos.
package main

import (
	"runtime/debug"
	"strings"
	"time"
)

// version and commitTime are set at build time by the Taskfile:
//
//	-ldflags "-X main.version=$(git describe --tags --always --dirty)
//	          -X main.commitTime=$(git log -1 --format=%cI)"
//
// so every build after a commit reports that commit. When they are unset
// (a plain `go build`), the VCS information Go stamps into the binary is
// used instead, which gives the short revision, commit time and a dirty flag
// but not the tag.
var (
	version    string
	commitTime string
)

// buildVersion returns e.g. "v0.2.0-3-gabc1234 (committed 2026-09-02 10:40 UTC)".
func buildVersion() string {
	v, t, dirty := version, commitTime, false
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				if v == "" && len(s.Value) >= 7 {
					v = s.Value[:7]
				}
			case "vcs.time":
				if t == "" {
					t = s.Value
				}
			case "vcs.modified":
				dirty = s.Value == "true"
			}
		}
	}
	if v == "" {
		v = "dev"
	}
	if dirty && !strings.HasSuffix(v, "-dirty") {
		v += "-dirty"
	}
	if ts, err := time.Parse(time.RFC3339, t); err == nil {
		t = ts.UTC().Format("2006-01-02 15:04 UTC")
	}
	if t == "" {
		return v
	}
	return v + " (committed " + t + ")"
}
