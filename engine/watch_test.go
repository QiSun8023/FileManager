package main

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func watchFixture(t *testing.T) (*Engine, WatchConfig, string, string) {
	t.Helper()
	base := t.TempDir()
	e := &Engine{Data: filepath.Join(base, "data"), Storage: base}
	src := filepath.Join(base, "incoming")
	dst := filepath.Join(base, "sorted")
	for _, p := range []string{e.Data, src, dst} {
		if err := os.MkdirAll(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	return e, WatchConfig{Enabled: true, StableSeconds: 30, Rules: []WatchRule{{Root: src, Target: dst, Extensions: []string{"zip", "APK"}}}}, src, dst
}
func watchArchive(t *testing.T, p string) {
	t.Helper()
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	w, _ := z.Create("content.txt")
	w.Write([]byte("complete archive payload"))
	z.Close()
	f.Close()
}
func pendingWatch(t *testing.T, e *Engine) []WatchItem {
	t.Helper()
	s, err := e.watchState()
	if err != nil {
		t.Fatal(err)
	}
	v := []WatchItem{}
	for _, i := range s.Items {
		if i.State == "pending" {
			v = append(v, i)
		}
	}
	return v
}
func tickWatch(t *testing.T, e *Engine, now time.Time) {
	t.Helper()
	if err := e.watchTick(now); err != nil {
		t.Fatal(err)
	}
}
func TestWatchBaselineStabilityAndRestart(t *testing.T) {
	e, c, src, _ := watchFixture(t)
	watchArchive(t, filepath.Join(src, "old.zip"))
	if _, err := e.watchSave(c); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	tickWatch(t, e, now)
	if len(pendingWatch(t, e)) != 0 {
		t.Fatal("old file prompted")
	}
	watchArchive(t, filepath.Join(src, "new.APK"))
	tickWatch(t, e, now)
	tickWatch(t, e, now.Add(29*time.Second))
	if len(pendingWatch(t, e)) != 0 {
		t.Fatal("premature prompt")
	}
	e = &Engine{Data: e.Data, Storage: e.Storage}
	tickWatch(t, e, now.Add(31*time.Second))
	items := pendingWatch(t, e)
	if len(items) != 1 || !items[0].Verified {
		t.Fatal(items)
	}
	tickWatch(t, e, now.Add(time.Hour))
	if len(pendingWatch(t, e)) != 1 {
		t.Fatal("duplicate prompt")
	}
}
func TestWatchIncompleteAndTemporary(t *testing.T) {
	e, c, src, _ := watchFixture(t)
	e.watchSave(c)
	p := filepath.Join(src, "new.zip")
	os.WriteFile(p, []byte("unfinished"), 0600)
	now := time.Now()
	tickWatch(t, e, now)
	tickWatch(t, e, now.Add(time.Hour))
	if len(pendingWatch(t, e)) != 0 {
		t.Fatal("incomplete archive prompted")
	}
	watchArchive(t, p)
	os.WriteFile(p+".aria2", []byte("downloading"), 0600)
	tickWatch(t, e, now.Add(2*time.Hour))
	tickWatch(t, e, now.Add(3*time.Hour))
	if len(pendingWatch(t, e)) != 0 {
		t.Fatal("temporary download prompted")
	}
	os.Remove(p + ".aria2")
	tickWatch(t, e, now.Add(4*time.Hour))
	if len(pendingWatch(t, e)) != 1 {
		t.Fatal("completed file missing")
	}
}
func TestWatchKeepAndMove(t *testing.T) {
	for _, decision := range []string{"keep", "move"} {
		t.Run(decision, func(t *testing.T) {
			e, c, src, dst := watchFixture(t)
			e.watchSave(c)
			p := filepath.Join(src, "new.zip")
			watchArchive(t, p)
			now := time.Now()
			tickWatch(t, e, now)
			tickWatch(t, e, now.Add(time.Minute))
			item := pendingWatch(t, e)[0]
			if _, err := e.watchDecide(Request{ID: item.ID, Decision: decision}); err != nil {
				t.Fatal(err)
			}
			if len(pendingWatch(t, e)) != 0 {
				t.Fatal("decision pending")
			}
			if decision == "move" {
				if _, err := os.Stat(p); !os.IsNotExist(err) {
					t.Fatal("source not removed")
				}
				if _, err := os.Stat(filepath.Join(dst, "new.zip")); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := os.Stat(p); err != nil {
					t.Fatal(err)
				}
				os.WriteFile(p, []byte("changed but already kept"), 0600)
				tickWatch(t, e, now.Add(time.Hour))
				tickWatch(t, e, now.Add(2*time.Hour))
				if len(pendingWatch(t, e)) != 0 {
					t.Fatal("kept file repeated")
				}
			}
		})
	}
}
func TestWatchMoveNeverOverwritesOrMovesChangedFiles(t *testing.T) {
	e, c, src, dst := watchFixture(t)
	e.watchSave(c)
	p := filepath.Join(src, "new.zip")
	watchArchive(t, p)
	now := time.Now()
	tickWatch(t, e, now)
	tickWatch(t, e, now.Add(time.Minute))
	item := pendingWatch(t, e)[0]
	target := filepath.Join(dst, "new.zip")
	os.WriteFile(target, []byte("existing"), 0600)
	if _, err := e.watchDecide(Request{ID: item.ID, Decision: "move"}); err == nil {
		t.Fatal("overwrote target")
	}
	b, _ := os.ReadFile(target)
	if string(b) != "existing" {
		t.Fatal("target changed")
	}
	os.Remove(target)
	os.WriteFile(p, []byte("changed after confirmation"), 0600)
	if _, err := e.watchDecide(Request{ID: item.ID, Decision: "move"}); err == nil {
		t.Fatal("moved changed source")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("unexpected destination")
	}
}
func TestWatchNewRulesBaselineAndTargets(t *testing.T) {
	e, c, src, _ := watchFixture(t)
	c.Rules[0].Target = filepath.Join(src, "nested")
	os.Mkdir(c.Rules[0].Target, 0700)
	if _, err := e.watchSave(c); err == nil || !strings.Contains(err.Error(), "目标目录") {
		t.Fatal("accepted loop", err)
	}
	c.Rules[0].Target = filepath.Join(e.Storage, "sorted")
	e.watchSave(c)
	c = e.watchConfig()
	watchArchive(t, filepath.Join(src, "before-reenable.zip"))
	c.Enabled = false
	e.watchSave(c)
	c.Enabled = true
	e.watchSave(c)
	now := time.Now()
	tickWatch(t, e, now)
	tickWatch(t, e, now.Add(time.Hour))
	if len(pendingWatch(t, e)) != 0 {
		t.Fatal("disabled-period files prompted")
	}
}
func TestWatchMissingDirectoryRetainsBaseline(t *testing.T) {
	e, c, src, _ := watchFixture(t)
	watchArchive(t, filepath.Join(src, "old.zip"))
	e.watchSave(c)
	before, _ := os.ReadFile(e.path("watch-state.json"))
	os.Rename(src, src+"-away")
	if err := e.watchTick(time.Now()); err == nil {
		t.Fatal("missing source accepted")
	}
	os.Rename(src+"-away", src)
	tickWatch(t, e, time.Now().Add(time.Hour))
	if len(pendingWatch(t, e)) != 0 {
		t.Fatal("lost old baseline")
	}
	if len(before) == 0 {
		t.Fatal("no baseline persisted")
	}
}

func TestWatchStatusRejectsForeignPID(t *testing.T) {
	e, _, _, _ := watchFixture(t)
	os.MkdirAll(e.path("watch-daemon"), 0700)
	os.WriteFile(e.path("watch-daemon/pid"), []byte(fmt.Sprint(os.Getpid())), 0600)
	v, err := e.watchDispatch("watch-status", Request{})
	if err != nil {
		t.Fatal(err)
	}
	if v.(map[string]any)["running"] != false {
		t.Fatal("foreign/reused PID advertised as a running monitor")
	}
}
