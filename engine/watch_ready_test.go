package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatchCorruptCRCAndAPKOneRemainWaiting(t *testing.T) {
	e, c, src, _ := watchFixture(t)
	c.Rules[0].Extensions = []string{".apk.1", ".zip"}
	e.watchSave(c)
	p := filepath.Join(src, "corrupt.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	w, _ := z.CreateHeader(&zip.FileHeader{Name: "content", Method: zip.Store})
	w.Write([]byte("CRC protected payload"))
	z.Close()
	f.Close()
	b, _ := os.ReadFile(p)
	i := bytes.Index(b, []byte("CRC protected payload"))
	if i < 0 {
		t.Fatal("fixture payload missing")
	}
	b[i] ^= 1
	os.WriteFile(p, b, 0600)
	os.WriteFile(filepath.Join(src, "partial.apk.1"), []byte("unfinished"), 0600)
	tickWatch(t, e, time.Now())
	tickWatch(t, e, time.Now().Add(time.Hour))
	if len(pendingWatch(t, e)) != 0 {
		t.Fatal("corrupt CRC or incomplete APK.1 prompted")
	}
}

func TestWatchCompleteArchiveImmediateAndProtected(t *testing.T) {
	e, c, src, _ := watchFixture(t)
	if _, err := e.watchSave(c); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	p := filepath.Join(src, "ready.apk")
	watchArchive(t, p)
	tickWatch(t, e, now)
	if len(pendingWatch(t, e)) != 1 {
		t.Fatal("complete APK delayed on first detection")
	}
	q := filepath.Join(src, "downloading.zip")
	os.WriteFile(q, []byte("partial"), 0600)
	tickWatch(t, e, now.Add(time.Second))
	tickWatch(t, e, now.Add(time.Hour))
	if len(pendingWatch(t, e)) != 1 {
		t.Fatal("unfinished archive prompted")
	}
	watchArchive(t, q)
	os.WriteFile(q+".crdownload", nil, 0600)
	tickWatch(t, e, now.Add(2*time.Hour))
	tickWatch(t, e, now.Add(3*time.Hour))
	if len(pendingWatch(t, e)) != 1 {
		t.Fatal("download marker ignored")
	}
	os.Remove(q + ".crdownload")
	tickWatch(t, e, now.Add(3*time.Hour+time.Second))
	if len(pendingWatch(t, e)) != 2 {
		t.Fatal("finished download delayed")
	}
}

func TestWatchFallbackResetsWhenChanging(t *testing.T) {
	e, c, src, _ := watchFixture(t)
	c.StableSeconds = 10
	c.Rules[0].Extensions = []string{".rar"}
	e.watchSave(c)
	now := time.Now()
	p := filepath.Join(src, "unknown.rar")
	os.WriteFile(p, []byte("first"), 0600)
	tickWatch(t, e, now)
	tickWatch(t, e, now.Add(9*time.Second))
	if len(pendingWatch(t, e)) != 0 {
		t.Fatal("no stability fallback")
	}
	os.WriteFile(p, []byte("changed content"), 0600)
	tickWatch(t, e, now.Add(10*time.Second))
	tickWatch(t, e, now.Add(19*time.Second))
	if len(pendingWatch(t, e)) != 0 {
		t.Fatal("changing file prompted")
	}
	tickWatch(t, e, now.Add(20*time.Second))
	items := pendingWatch(t, e)
	if len(items) != 1 || items[0].Verified {
		t.Fatal("fallback failure", items)
	}
}

func TestWatchAPKOneSelectiveRenameAndConflict(t *testing.T) {
	for _, decision := range []string{"keep", "move", "conflict"} {
		t.Run(decision, func(t *testing.T) {
			e, c, src, dst := watchFixture(t)
			c.Rules[0].Extensions = []string{"APK.1"}
			if _, err := e.watchSave(c); err != nil {
				t.Fatal(err)
			}
			p := filepath.Join(src, "微信软件.APK.1")
			watchArchive(t, p)
			watchArchive(t, filepath.Join(src, "ordinary.apk"))
			tickWatch(t, e, time.Now())
			items := pendingWatch(t, e)
			if len(items) != 1 || !items[0].Verified || items[0].TargetName != "微信软件.apk" {
				t.Fatal(items)
			}
			target := filepath.Join(dst, "微信软件.apk")
			if decision == "conflict" {
				os.WriteFile(target, []byte("original"), 0600)
				if _, err := e.watchDecide(Request{ID: items[0].ID, Decision: "move"}); err == nil {
					t.Fatal("overwrote renamed destination")
				}
				b, _ := os.ReadFile(target)
				if string(b) != "original" {
					t.Fatal("destination changed")
				}
				if _, err := os.Stat(p); err != nil {
					t.Fatal("source lost")
				}
				return
			}
			if _, err := e.watchDecide(Request{ID: items[0].ID, Decision: decision}); err != nil {
				t.Fatal(err)
			}
			if decision == "keep" {
				if _, err := os.Stat(p); err != nil {
					t.Fatal("keep renamed source")
				}
				if _, err := os.Stat(target); !os.IsNotExist(err) {
					t.Fatal("keep created destination")
				}
			} else {
				if _, err := os.Stat(p); !os.IsNotExist(err) {
					t.Fatal("source remains")
				}
				if _, err := os.Stat(target); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
	e, c, src, _ := watchFixture(t)
	c.Rules[0].Extensions = []string{"apk"}
	e.watchSave(c)
	watchArchive(t, filepath.Join(src, "only.apk.1"))
	tickWatch(t, e, time.Now())
	if len(pendingWatch(t, e)) != 0 {
		t.Fatal("APK selection unintentionally includes APK.1")
	}
}

func TestWatchLegacyDelayPolicyMigration(t *testing.T) {
	e, c, _, _ := watchFixture(t)
	c.StableSeconds = 60
	save(e.path("watch-config.json"), c)
	got := e.watchConfig()
	if got.StableSeconds != 10 || got.ReadyPolicy != 1 {
		t.Fatal(got)
	}
	got.StableSeconds = 120
	e.watchSave(got)
	if e.watchConfig().StableSeconds != 120 {
		t.Fatal("explicit new fallback overwritten")
	}
}
