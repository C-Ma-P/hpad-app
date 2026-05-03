package singleinstance

import (
	"path/filepath"
	"testing"
)

func TestAcquireBlocksSecondCaller(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "hpad.lock")

	first, acquired, err := Acquire(lockPath)
	if err != nil {
		t.Fatalf("first Acquire() error = %v", err)
	}
	if !acquired {
		t.Fatal("first Acquire() should acquire the lock")
	}
	t.Cleanup(func() {
		if err := first.Release(); err != nil {
			t.Fatalf("Release() error = %v", err)
		}
	})

	second, acquired, err := Acquire(lockPath)
	if err != nil {
		t.Fatalf("second Acquire() error = %v", err)
	}
	if second != nil {
		t.Fatal("second Acquire() returned a lock for a contested file")
	}
	if acquired {
		t.Fatal("second Acquire() should report the lock as held by another instance")
	}
}

func TestReleaseAllowsFutureAcquire(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "hpad.lock")

	first, acquired, err := Acquire(lockPath)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	if !acquired {
		t.Fatal("Acquire() should acquire the lock")
	}
	if err := first.Release(); err != nil {
		t.Fatalf("Release() error = %v", err)
	}

	second, acquired, err := Acquire(lockPath)
	if err != nil {
		t.Fatalf("Acquire() after release error = %v", err)
	}
	if !acquired {
		t.Fatal("Acquire() after release should succeed")
	}
	if second == nil {
		t.Fatal("Acquire() after release returned nil lock")
	}
	t.Cleanup(func() {
		if err := second.Release(); err != nil {
			t.Fatalf("second Release() error = %v", err)
		}
	})
}