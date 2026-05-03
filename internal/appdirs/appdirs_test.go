package appdirs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveUsesXDGPaths(t *testing.T) {
	t.Setenv("HOME", "/tmp/home")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/config")
	t.Setenv("XDG_STATE_HOME", "/tmp/state")
	t.Setenv("XDG_RUNTIME_DIR", "/tmp/runtime")

	paths, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if paths.ConfigDir != "/tmp/config/hpad" {
		t.Fatalf("ConfigDir = %q, want %q", paths.ConfigDir, "/tmp/config/hpad")
	}
	if paths.StateDir != "/tmp/state/hpad" {
		t.Fatalf("StateDir = %q, want %q", paths.StateDir, "/tmp/state/hpad")
	}
	if paths.RuntimeDir != "/tmp/runtime/hpad" {
		t.Fatalf("RuntimeDir = %q, want %q", paths.RuntimeDir, "/tmp/runtime/hpad")
	}
	if paths.LockFile != "/tmp/runtime/hpad/hpad.lock" {
		t.Fatalf("LockFile = %q, want %q", paths.LockFile, "/tmp/runtime/hpad/hpad.lock")
	}
}

func TestResolveFallsBackWhenXDGPathsMissing(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("XDG_RUNTIME_DIR", "")

	paths, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if paths.ConfigDir != filepath.Join(homeDir, ".config", "hpad") {
		t.Fatalf("ConfigDir = %q", paths.ConfigDir)
	}
	if paths.StateDir != filepath.Join(homeDir, ".local", "state", "hpad") {
		t.Fatalf("StateDir = %q", paths.StateDir)
	}
	if paths.RuntimeDir != filepath.Join(homeDir, ".local", "state", "hpad", "runtime") {
		t.Fatalf("RuntimeDir = %q", paths.RuntimeDir)
	}
	if paths.LockFile != filepath.Join(homeDir, ".local", "state", "hpad", "runtime", "hpad.lock") {
		t.Fatalf("LockFile = %q", paths.LockFile)
	}
}

func TestEnsureCreatesPrivateDirectories(t *testing.T) {
	baseDir := t.TempDir()
	paths := Paths{
		ConfigDir:  filepath.Join(baseDir, "config"),
		StateDir:   filepath.Join(baseDir, "state"),
		RuntimeDir: filepath.Join(baseDir, "runtime"),
	}

	if err := Ensure(paths); err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}

	for _, dir := range []string{paths.ConfigDir, paths.StateDir, paths.RuntimeDir} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("Stat(%q) error = %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("%q is not a directory", dir)
		}
		if got := info.Mode().Perm(); got != 0o700 {
			t.Fatalf("permissions for %q = %o, want 700", dir, got)
		}
	}
}
