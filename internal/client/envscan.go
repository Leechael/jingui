package client

import (
	"os"
	"strings"

	"github.com/aspect-build/jingui/internal/refparser"
)

// ScanResult holds the results of scanning environment variables for jingui:// references.
type ScanResult struct {
	// PlainEnv contains all env vars that are NOT jingui:// refs (ready to pass to child process).
	PlainEnv []string
	// Refs maps env var name → jingui:// reference string (to be resolved).
	Refs map[string]string
}

// MergeEnvFileWithProcess merges .env file entries with the current process environment.
// .env entries take precedence over process env.
// Returns separated plain values and jingui:// references.
func MergeEnvFileWithProcess(entries []EnvEntry) ScanResult {
	// Start with process env as a map
	envMap := make(map[string]string)
	// Track insertion order
	var keys []string
	seen := make(map[string]bool)

	for _, e := range os.Environ() {
		k, v, _ := strings.Cut(e, "=")
		envMap[k] = v
		if !seen[k] {
			keys = append(keys, k)
			seen[k] = true
		}
	}

	// .env entries override process env
	for _, entry := range entries {
		if _, exists := seen[entry.Key]; !exists {
			keys = append(keys, entry.Key)
			seen[entry.Key] = true
		}
		envMap[entry.Key] = entry.Value
	}

	result := ScanResult{
		Refs: make(map[string]string),
	}

	for _, k := range keys {
		v := envMap[k]
		if refparser.IsRef(v) {
			result.Refs[k] = v
		} else {
			result.PlainEnv = append(result.PlainEnv, k+"="+v)
		}
	}

	return result
}
