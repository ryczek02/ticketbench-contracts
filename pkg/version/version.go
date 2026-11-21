// Package version carries build metadata set through -ldflags:
//
//	-X github.com/ryczek02/ticketbench-contracts/pkg/version.Version=v0.1.0
//	-X github.com/ryczek02/ticketbench-contracts/pkg/version.Commit=abc1234
//	-X github.com/ryczek02/ticketbench-contracts/pkg/version.Date=2026-11-21
package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

var (
	Version = "dev"
	Commit  = ""
	Date    = ""
)

// String returns "v0.1.0 (abc1234, 2026-11-21, go1.26 darwin/arm64)".
func String() string {
	commit, date := Commit, Date
	if commit == "" {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, s := range info.Settings {
				switch s.Key {
				case "vcs.revision":
					commit = s.Value
				case "vcs.time":
					date = s.Value
				}
			}
		}
	}
	if len(commit) > 7 {
		commit = commit[:7]
	}
	return fmt.Sprintf("%s (%s, %s, %s %s/%s)", Version, orNone(commit), orNone(date), runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

func orNone(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}
