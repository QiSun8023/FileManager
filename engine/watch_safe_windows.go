package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

func openWatchFile(root, path string, flags int, mode uint32) (*os.File, error) {
	if !within(root, path) {
		return nil, errors.New("outside storage")
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil || parent != filepath.Dir(path) {
		return nil, errors.New("unsafe parent")
	}
	if _, err := os.Lstat(path); err == nil {
		if err = safeRegular(path); err != nil {
			return nil, err
		}
	}
	return os.OpenFile(path, flags, fs.FileMode(mode))
}
func removeWatchFile(root, path string, expected fs.FileInfo) error {
	if !within(root, path) {
		return errors.New("outside storage")
	}
	if err := safeRegular(path); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !os.SameFile(info, expected) {
		return errors.New("changed identity")
	}
	return os.Remove(path)
}

func timeWatchFile(root, path string, t time.Time) error {
	if err := safeRegular(path); err != nil {
		return err
	}
	return os.Chtimes(path, t, t)
}
