package version

import (
	"fmt"
	"runtime"
)

// Set via -ldflags at build time:
//
//	go build -ldflags "-X github.com/aspect-build/jingui/internal/version.Version=0.1.0
//	  -X github.com/aspect-build/jingui/internal/version.GitCommit=abc1234"
var (
	Version   = "dev"
	GitCommit = "unknown"
)

// String returns a human-readable version string.
func String(binaryName string) string {
	return fmt.Sprintf("%s %s (commit=%s, go=%s, %s/%s)",
		binaryName, Version, GitCommit, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
