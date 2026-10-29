package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) *Engine {
	t.Helper()
	base := t.TempDir()
	e := &Engine{Data: filepath.Join(base, "private"), Storage: filepath.Join(base, "storage")}
	for _, p := range []string{e.Data, e.Storage} {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	return e
}
func put(t *testing.T, e *Engine, name, content string) string {
	t.Helper()
	p := filepath.Join(e.Storage, name)
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func report(t *testing.T, e *Engine) Report {
	t.Helper()
	var r Report
	if err := read(e.path("report.json"), &r); err != nil {
		t.Fatal(err)
	}
	return r
}
func runScan(t *testing.T, e *Engine) Report {
	t.Helper()
	if err := e.scan(Request{Root: e.Storage, Min: 1}); err != nil {
		t.Fatal(err)
	}
	return report(t, e)
}
func TestScanContentAndExcludedPaths(t *testing.T) {
	e := fixture(t)
	for _, n := range []string{"a.txt", "中文 ' $` 引号.txt", "line\nname.txt"} {
		if strings.Contains(n, "\n") && os.PathSeparator == '\\' {
			continue
		}
		put(t, e, n, "same")
	}
	put(t, e, "not-duplicate.txt", "else")
	put(t, e, "Android/data/private", "same")
	put(t, e, ".hidden/secret", "same")
	put(t, e, "empty", "")
	r := runScan(t, e)
	if len(r.Groups) != 1 {
		t.Fatalf("groups: %+v", r.Groups)
	}
	expected := 3
	if os.PathSeparator == '\\' {
		expected = 2
	}
	if len(r.Groups[0].Files) != expected {
		t.Fatalf("files=%d", len(r.Groups[0].Files))
	}
	if e.status.Hashed != expected+1 {
		t.Fatalf("hashed=%d", e.status.Hashed)
	}
}
func TestKeepOneThenRestoreAndPurge(t *testing.T) {
	e := fixture(t)
	put(t, e, "a", "abc")
	put(t, e, "b", "abc")
	r := runScan(t, e)
	g := r.Groups[0]
	ids := []string{g.Files[0].ID, g.Files[1].ID}
	if _, err := e.dispatch("clean", Request{Scan: r.ID, IDs: ids}); err == nil {
		t.Fatal("allowed removing all copies")
	}
	v, err := e.dispatch("clean", Request{Scan: r.ID, IDs: ids[1:]})
	if err != nil {
		t.Fatal(err)
	}
	if v.(map[string]any)["moved"] != 1 {
		t.Fatal(v)
	}
	if _, err = os.Stat(g.Files[0].Path); err != nil {
		t.Fatal("keeper missing")
	}
	items, err := e.trashList()
	if err != nil || len(items) != 1 {
		t.Fatal(items, err)
	}
	put(t, e, "b", "new")
	v, err = e.dispatch("restore", Request{IDs: []string{items[0].ID}})
	if err != nil {
		t.Fatal(err)
	}
	if v.(map[string]any)["done"] != 0 {
		t.Fatal("overwrote existing file")
	}
	b, _ := os.ReadFile(g.Files[1].Path)
	if string(b) != "new" {
		t.Fatal("destination changed")
	}
	os.Remove(g.Files[1].Path)
	v, err = e.dispatch("restore", Request{IDs: []string{items[0].ID}})
	if err != nil || v.(map[string]any)["done"] != 1 {
		t.Fatal(v, err)
	}
	b, _ = os.ReadFile(g.Files[1].Path)
	if string(b) != "abc" {
		t.Fatal("restore mismatch")
	}
	r = runScan(t, e)
	_, err = e.dispatch("clean", Request{Scan: r.ID, IDs: []string{r.Groups[0].Files[1].ID}})
	if err != nil {
		t.Fatal(err)
	}
	items, _ = e.trashList()
	v, err = e.dispatch("purge", Request{IDs: []string{items[0].ID}})
	if err != nil || v.(map[string]any)["done"] != 1 {
		t.Fatal(v, err)
	}
	items, _ = e.trashList()
	if len(items) != 0 {
		t.Fatal("trash not empty")
	}
}
func TestChangedFileAndKeeperRejected(t *testing.T) {
	for _, index := range []int{0, 1} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			e := fixture(t)
			put(t, e, "a", "abc")
			put(t, e, "b", "abc")
			r := runScan(t, e)
			os.WriteFile(r.Groups[0].Files[index].Path, []byte("xyz"), 0600)
			v, err := e.dispatch("clean", Request{Scan: r.ID, IDs: []string{r.Groups[0].Files[1].ID}})
			if err != nil {
				t.Fatal(err)
			}
			if v.(map[string]any)["moved"] != 0 {
				t.Fatal("moved changed file or lost keeper")
			}
			for _, f := range r.Groups[0].Files {
				if _, err := os.Stat(f.Path); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
func TestRejectTraversalAndOldScan(t *testing.T) {
	e := fixture(t)
	for _, p := range []string{e.Data, filepath.Join(e.Storage, "Android"), filepath.Join(e.Storage, ".hidden"), filepath.Join(e.Storage, "..", "escape")} {
		if _, err := e.checked(p); err == nil {
			t.Fatalf("accepted %s", p)
		}
	}
	put(t, e, "a", "abc")
	put(t, e, "b", "abc")
	runScan(t, e)
	if _, err := e.dispatch("clean", Request{Scan: "stale", IDs: []string{"bad"}}); err == nil {
		t.Fatal("accepted stale scan")
	}
	if _, err := e.dispatch("purge", Request{IDs: []string{"../../escape"}}); err == nil {
		t.Fatal("accepted traversal id")
	}
}
func TestCancelPreservesPreviousReport(t *testing.T) {
	e := fixture(t)
	put(t, e, "a", "abc")
	put(t, e, "b", "abc")
	r := runScan(t, e)
	os.WriteFile(e.path("cancel"), []byte("1"), 0600)
	if err := e.scan(Request{Root: e.Storage}); err != cancelled {
		t.Fatal(err)
	}
	if report(t, e).ID != r.ID {
		t.Fatal("cancel destroyed report")
	}
}
func TestMinSizeAndSingleSizeNotHashed(t *testing.T) {
	e := fixture(t)
	put(t, e, "a", "a")
	put(t, e, "b", "a")
	put(t, e, "unique", "longer")
	if err := e.scan(Request{Root: e.Storage, Min: 2}); err != nil {
		t.Fatal(err)
	}
	if e.status.Hashed != 0 || len(report(t, e).Groups) != 0 {
		t.Fatal("unnecessary hash or wrong groups")
	}
}
func TestLockExclusiveAndRelease(t *testing.T) {
	e := fixture(t)
	unlock, err := e.lock()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.lock(); err == nil {
		t.Fatal("double lock")
	}
	unlock()
	unlock, err = e.lock()
	if err != nil {
		t.Fatal(err)
	}
	unlock()
}
func TestSymlinkExcluded(t *testing.T) {
	e := fixture(t)
	p := put(t, e, "a", "abc")
	put(t, e, "b", "abc")
	link := filepath.Join(e.Storage, "link")
	if err := os.Symlink(p, link); err != nil {
		t.Skip("symlink privileges unavailable")
	}
	r := runScan(t, e)
	if len(r.Groups[0].Files) != 2 {
		t.Fatal("followed symlink")
	}
	if _, err := e.checked(link); err == nil {
		t.Fatal("accepted symlink")
	}
	if err := os.Symlink(e.Data, e.trashBase()); err == nil {
		if _, err = e.dispatch("clean", Request{Scan: r.ID, IDs: []string{r.Groups[0].Files[1].ID}}); err == nil {
			t.Fatal("accepted symlink trash")
		}
	}
}
func TestLargeGroupAndPagination(t *testing.T) {
	e := fixture(t)
	for i := 0; i < 103; i++ {
		put(t, e, fmt.Sprintf("f%03d", i), "same")
	}
	runScan(t, e)
	v, err := e.dispatch("groups", Request{})
	if err != nil {
		t.Fatal(err)
	}
	g := v.(map[string]any)["groups"].([]Group)[0]
	if len(g.Files) != 100 || g.Count != 103 {
		t.Fatal("unbounded group response")
	}
	if v.(map[string]any)["reclaimable"] != int64(102*4) {
		t.Fatal("hidden group files were omitted from capacity")
	}
	if _, err = e.dispatch("groups", Request{Page: -1}); err == nil {
		t.Fatal("accepted invalid page")
	}
}
func TestTamperedTrashRestoreRejected(t *testing.T) {
	e := fixture(t)
	put(t, e, "a", "abc")
	put(t, e, "b", "abc")
	r := runScan(t, e)
	_, err := e.dispatch("clean", Request{Scan: r.ID, IDs: []string{r.Groups[0].Files[1].ID}})
	if err != nil {
		t.Fatal(err)
	}
	items, _ := e.trashList()
	p, _ := e.payload(items[0].ID)
	os.WriteFile(p, []byte("tamper"), 0600)
	v, err := e.dispatch("restore", Request{IDs: []string{items[0].ID}})
	if err != nil || v.(map[string]any)["done"] != 0 {
		t.Fatal("restored tampered trash", v, err)
	}
}
