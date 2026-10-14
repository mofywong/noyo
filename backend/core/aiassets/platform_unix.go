//go:build !windows

package aiassets

import (
	"errors"
	"os"
	"syscall"
)

func availableSpace(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return uint64(stat.Bavail) * uint64(stat.Bsize), nil
}
func isNoSpace(err error) bool {
	return errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EDQUOT)
}

func acquireLock(path string) (func() error, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, errCode("conflict", "model directory is locked by another process")
	}
	return func() error { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); return f.Close() }, nil
}
func replaceFile(from, to string) error { return os.Rename(from, to) }
