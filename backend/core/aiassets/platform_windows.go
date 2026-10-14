//go:build windows

package aiassets

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

func availableSpace(path string) (uint64, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var free uint64
	get := syscall.NewLazyDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")
	r, _, callErr := get.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&free)), 0, 0)
	if r == 0 {
		return 0, callErr
	}
	return free, nil
}
func isNoSpace(err error) bool {
	return errors.Is(err, syscall.Errno(112)) || errors.Is(err, syscall.Errno(39))
}

func acquireLock(path string) (func() error, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, errCode("conflict", "model directory is locked by another process")
	}
	return func() error { return syscall.CloseHandle(h) }, nil
}
func replaceFile(from, to string) error {
	f, err := syscall.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	t, err := syscall.UTF16PtrFromString(to)
	if err != nil {
		return err
	}
	move := syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")
	r, _, callErr := move.Call(uintptr(unsafe.Pointer(f)), uintptr(unsafe.Pointer(t)), uintptr(0x1|0x8))
	if r == 0 {
		return &os.PathError{Op: "replace", Path: to, Err: callErr}
	}
	return nil
}
