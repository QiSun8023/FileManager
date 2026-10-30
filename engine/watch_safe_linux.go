//go:build linux || android

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Traverse every directory using descriptors, so a concurrently replaced symlink cannot escape storage.
func watchParent(root, path string) (int, string, error) {
	if !within(root, path) || filepath.Clean(path) == root {
		return -1, "", errors.New("路径不在共享存储中")
	}
	rel, _ := filepath.Rel(root, path)
	parts := strings.Split(rel, string(os.PathSeparator))
	fd, err := syscall.Open(root, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return -1, "", err
	}
	for _, part := range parts[:len(parts)-1] {
		next, er := syscall.Openat(fd, part, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		syscall.Close(fd)
		if er != nil {
			return -1, "", er
		}
		fd = next
	}
	return fd, parts[len(parts)-1], nil
}
func openWatchFile(root, path string, flags int, mode uint32) (*os.File, error) {
	fd, name, err := watchParent(root, path)
	if err != nil {
		return nil, err
	}
	defer syscall.Close(fd)
	file, err := syscall.Openat(fd, name, flags|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, mode)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(file), path)
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, errors.New("只允许普通文件")
	}
	return f, nil
}
func removeWatchFile(root, path string, expected fs.FileInfo) error {
	fd, name, err := watchParent(root, path)
	if err != nil {
		return err
	}
	defer syscall.Close(fd)
	info, err := os.Lstat(fmt.Sprintf("/proc/self/fd/%d/%s", fd, name))
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || !os.SameFile(info, expected) {
		return errors.New("文件身份已变化，已保留")
	}
	return syscall.Unlinkat(fd, name)
}

func timeWatchFile(root, path string, t time.Time) error {
	f, err := openWatchFile(root, path, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	tv := syscall.NsecToTimeval(t.UnixNano())
	return syscall.Futimes(int(f.Fd()), []syscall.Timeval{tv, tv})
}
