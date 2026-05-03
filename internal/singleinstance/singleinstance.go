package singleinstance

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

type Lock struct {
	file *os.File
	path string
}

func Acquire(lockPath string) (*Lock, bool, error) {
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, fmt.Errorf("open lock file %s: %w", lockPath, err)
	}

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("lock %s: %w", lockPath, err)
	}

	if err := file.Truncate(0); err != nil {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
		return nil, false, fmt.Errorf("truncate lock file %s: %w", lockPath, err)
	}
	if _, err := file.WriteString(fmt.Sprintf("%d\n", os.Getpid())); err != nil {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
		return nil, false, fmt.Errorf("write lock file %s: %w", lockPath, err)
	}

	return &Lock{file: file, path: lockPath}, true, nil
}

func (l *Lock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}

	file := l.file
	l.file = nil

	errUnlock := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	errClose := file.Close()
	if errUnlock != nil {
		return fmt.Errorf("unlock %s: %w", l.path, errUnlock)
	}
	if errClose != nil {
		return fmt.Errorf("close %s: %w", l.path, errClose)
	}
	return nil
}