package main

import (
	"errors"
	"os/exec"
)

func runWatchDaemon(e *Engine) error { return errors.New("Android only") }

func watchWriting(path string) bool { return false }

func detachWatch(cmd *exec.Cmd) {}

func watchDaemonAlive(e *Engine) bool { return false }

func reexecWatch() error { return errors.New("Android only") }
