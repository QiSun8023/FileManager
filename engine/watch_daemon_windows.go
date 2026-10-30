package main

import (
	"errors"
	"os/exec"
)

func runWatchDaemon(e *Engine) error { return errors.New("Android only") }

func watchWriting(path string) bool { return false }

func detachWatch(cmd *exec.Cmd) {}
