//go:build windows

package main

import (
	"os"
	"syscall"
	"time"
)

func accessTime(i os.FileInfo) time.Time {
	if s, ok := i.Sys().(*syscall.Win32FileAttributeData); ok {
		return time.Unix(0, s.LastAccessTime.Nanoseconds())
	}
	return i.ModTime()
}
func renameNoReplace(src, dst string) error {
	a, err := syscall.UTF16PtrFromString(src)
	if err != nil {
		return err
	}
	b, err := syscall.UTF16PtrFromString(dst)
	if err != nil {
		return err
	}
	return syscall.MoveFile(a, b)
}
func unsupportedRename(err error) bool { return false }
