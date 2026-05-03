package appdirs

import (
	"fmt"
	"os"
	"path/filepath"
)

type Paths struct {
	ConfigDir  string
	StateDir   string
	RuntimeDir string
	LockFile   string
}

func Resolve() (Paths, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve home directory: %w", err)
	}

	configHome := envOrFallback("XDG_CONFIG_HOME", filepath.Join(homeDir, ".config"))
	stateHome := envOrFallback("XDG_STATE_HOME", filepath.Join(homeDir, ".local", "state"))

	configDir := filepath.Join(configHome, "hpad")
	stateDir := filepath.Join(stateHome, "hpad")
	runtimeDir := filepath.Join(stateDir, "runtime")
	if runtimeHome := os.Getenv("XDG_RUNTIME_DIR"); filepath.IsAbs(runtimeHome) {
		runtimeDir = filepath.Join(runtimeHome, "hpad")
	}

	return Paths{
		ConfigDir:  configDir,
		StateDir:   stateDir,
		RuntimeDir: runtimeDir,
		LockFile:   filepath.Join(runtimeDir, "hpad.lock"),
	}, nil
}

func Ensure(paths Paths) error {
	for _, dir := range []string{paths.ConfigDir, paths.StateDir, paths.RuntimeDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	return nil
}

func envOrFallback(name string, fallback string) string {
	value := os.Getenv(name)
	if !filepath.IsAbs(value) {
		return fallback
	}
	return value
}
