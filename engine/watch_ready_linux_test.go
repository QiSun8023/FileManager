//go:build linux || android

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatchOpenWriterBlocksCompleteArchive(t *testing.T) {
	e, c, src, _ := watchFixture(t)
	e.watchSave(c)
	p := filepath.Join(src, "writer.apk")
	watchArchive(t, p)
	f, err := os.OpenFile(p, os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	tickWatch(t, e, time.Now())
	if len(pendingWatch(t, e)) != 0 {
		f.Close()
		t.Fatal("open writer ignored")
	}
	f.Close()
	tickWatch(t, e, time.Now().Add(time.Second))
	if len(pendingWatch(t, e)) != 1 {
		t.Fatal("closed complete archive delayed")
	}
}
