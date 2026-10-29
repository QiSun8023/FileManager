//go:build linux || android

package main

import (
	"errors"
	"os"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

func accessTime(i os.FileInfo) time.Time {
	if s, ok := i.Sys().(*syscall.Stat_t); ok {
		return time.Unix(s.Atim.Sec, s.Atim.Nsec)
	}
	return i.ModTime()
}
func renameNoReplace(src, dst string) error {
	a, err := syscall.BytePtrFromString(src)
	if err != nil {
		return err
	}
	b, err := syscall.BytePtrFromString(dst)
	if err != nil {
		return err
	}
	const renameNoReplaceFlag = 1
	var number uintptr
	switch runtime.GOARCH {
	case "arm64":
		number = 276
	case "amd64":
		number = 316
	default:
		return syscall.ENOSYS
	}
	_, _, errno := syscall.Syscall6(number, ^uintptr(99), uintptr(unsafe.Pointer(a)), ^uintptr(99), uintptr(unsafe.Pointer(b)), renameNoReplaceFlag, 0)
	if errno != 0 {
		return errno
	}
	return nil
}
func unsupportedRename(err error) bool {
	return errors.Is(err, syscall.ENOSYS) || errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.EOPNOTSUPP)
}
