package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOriginalNameRecommendation(t *testing.T) {
	for _, copyName := range []string{"文件1(1).m4a", "文件1 (1).m4a", "文件1（2）.m4a", "文件1(1)(2).m4a"} {
		files := []File{{ID: "copy", Path: filepath.Join("a", copyName)}, {ID: "original", Path: filepath.Join("z", "文件1.m4a")}}
		reason := rankFiles(files)
		if files[0].ID != "original" || !strings.Contains(reason, "无副本编号") {
			t.Fatal(files, reason)
		}
	}
}
func TestSelectionAcrossPages(t *testing.T) {
	e := fixture(t)
	for i := 0; i < 28; i++ {
		body := fmt.Sprintf("unique-content-%03d", i)
		put(t, e, fmt.Sprintf("目录/文件%d.m4a", i), body)
		put(t, e, fmt.Sprintf("目录/文件%d(1).m4a", i), body)
	}
	r := runScan(t, e)
	if len(r.Groups) != 28 {
		t.Fatal(len(r.Groups))
	}
	v, err := e.dispatch("select", Request{Scan: r.ID, Mode: "suggest-all"})
	if err != nil || v.(map[string]any)["count"] != 28 {
		t.Fatal(v, err)
	}
	v, err = e.dispatch("groups", Request{Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	d := v.(map[string]any)
	if d["selection"].(map[string]any)["count"] != 28 {
		t.Fatal(d)
	}
	for _, g := range d["groups"].([]Group) {
		if g.Files[0].Selected || !g.Files[1].Selected {
			t.Fatal("wrong original or selection", g)
		}
	}
	fresh := &Engine{Data: e.Data, Storage: e.Storage}
	if len(fresh.selection(r).IDs) != 28 {
		t.Fatal("selection lost after reopening")
	}
	v, err = e.dispatch("select", Request{Scan: r.ID, Mode: "clear-page", Page: 0})
	if err != nil || v.(map[string]any)["count"] != 3 {
		t.Fatal(v, err)
	}
	v, err = e.dispatch("select", Request{Scan: r.ID, Mode: "clear-all"})
	if err != nil || v.(map[string]any)["count"] != 0 {
		t.Fatal(v, err)
	}
}
func TestSelectionKeepsOneAndRejectsStaleReport(t *testing.T) {
	e := fixture(t)
	put(t, e, "原件.m4a", "a")
	put(t, e, "原件(1).m4a", "a")
	r := runScan(t, e)
	e.dispatch("select", Request{Scan: r.ID, Mode: "suggest-all"})
	groups := e.activeGroups(r)
	if _, err := e.dispatch("select", Request{Scan: r.ID, Mode: "toggle", ID: groups[0].Files[0].ID, Checked: true}); err == nil {
		t.Fatal("selected entire group")
	}
	if len(e.selection(r).IDs) != 1 {
		t.Fatal("failed request altered selection")
	}
	if _, err := e.dispatch("select", Request{Scan: "stale", Mode: "clear-all"}); err == nil {
		t.Fatal("accepted stale scan")
	}
}
func TestDirectoryBrowser(t *testing.T) {
	e := fixture(t)
	for _, dir := range []string{"Download/微信录音", "Pictures", "Android/FileManager/Trash", ".hidden"} {
		os.MkdirAll(filepath.Join(e.Storage, dir), 0700)
	}
	v, err := e.dispatch("directories", Request{Root: e.Storage})
	if err != nil {
		t.Fatal(err)
	}
	d := v.(map[string]any)
	if d["total"] != 2 || d["parent"] != "" {
		t.Fatal(d)
	}
	v, err = e.dispatch("directories", Request{Root: filepath.Join(e.Storage, "Download")})
	if err != nil {
		t.Fatal(err)
	}
	d = v.(map[string]any)
	if d["parent"] != e.Storage || len(d["items"].([]map[string]string)) != 1 {
		t.Fatal(d)
	}
	for _, path := range []string{e.Data, e.trashRoot(), filepath.Dir(e.Storage)} {
		if _, err = e.dispatch("directories", Request{Root: path}); err == nil {
			t.Fatal("accepted excluded path", path)
		}
	}
}
func legacyTrash(t *testing.T, e *Engine, content string) Trash {
	t.Helper()
	id := token()
	hash := sha256.Sum256([]byte(content))
	ts := int64(1739342415)
	item := Trash{ID: id, Path: filepath.Join(e.Storage, "微信 中华 2025-02-12 14-40-15.aac"), Hash: hex.EncodeToString(hash[:]), Size: int64(len(content)), At: 100, Modified: &ts}
	p := filepath.Join(e.Storage, ".dupeguard-trash", id)
	if err := ensureDir(p); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(p, "content"), []byte(content), 0600)
	ensureDir(e.path("trash"))
	if err := save(filepath.Join(e.path("trash"), id+".json"), item); err != nil {
		t.Fatal(err)
	}
	return item
}
func TestLegacyTrashMigrationAndRestore(t *testing.T) {
	e := fixture(t)
	item := legacyTrash(t, e, "audio")
	os.WriteFile(e.path("unrelated-state"), []byte("keep"), 0600)
	v, err := e.dispatch("trash", Request{})
	if err != nil {
		t.Fatal(err)
	}
	d := v.(map[string]any)
	if d["total"] != 1 || d["bytes"] != int64(5) || d["root"] != e.trashRoot() {
		t.Fatal(d)
	}
	if _, err = os.Stat(filepath.Join(e.path("trash"), item.ID+".json")); !os.IsNotExist(err) {
		t.Fatal("old record not retired")
	}
	if _, err = e.readTrash(item.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(e.Storage, ".dupeguard-trash", item.ID, "content")); !os.IsNotExist(err) {
		t.Fatal("old payload not retired")
	}
	v, err = e.dispatch("restore", Request{IDs: []string{item.ID}})
	if err != nil || v.(map[string]any)["done"] != 1 {
		t.Fatal(v, err)
	}
	b, _ := os.ReadFile(item.Path)
	i, _ := os.Stat(item.Path)
	if string(b) != "audio" || i.ModTime().Unix() != *item.Modified {
		t.Fatal("restore content or date failed")
	}
	b, _ = os.ReadFile(e.path("unrelated-state"))
	if string(b) != "keep" {
		t.Fatal("private state was moved or removed")
	}
}
func TestMigrationResumeAndConflicts(t *testing.T) {
	for _, scenario := range []string{"copied", "source-removed", "bad-source", "conflict", "unwritable-layout"} {
		t.Run(scenario, func(t *testing.T) {
			e := fixture(t)
			item := legacyTrash(t, e, "audio")
			old := filepath.Join(e.Storage, ".dupeguard-trash", item.ID, "content")
			switch scenario {
			case "copied", "source-removed":
				e.prepareTrash()
				ensureDir(filepath.Join(e.trashBase(), item.ID))
				os.WriteFile(filepath.Join(e.trashBase(), item.ID, "content"), []byte("audio"), 0600)
				if scenario == "source-removed" {
					save(filepath.Join(e.recordDir(), item.ID+".json"), item)
					os.Remove(old)
				}
			case "bad-source":
				os.WriteFile(old, []byte("changed"), 0600)
			case "conflict":
				e.prepareTrash()
				ensureDir(filepath.Join(e.trashBase(), item.ID))
				os.WriteFile(filepath.Join(e.trashBase(), item.ID, "content"), []byte("other"), 0600)
			case "unwritable-layout":
				os.WriteFile(filepath.Join(e.Storage, "Android"), []byte("not a directory"), 0600)
			}
			err := e.migrateTrash()
			if scenario == "copied" || scenario == "source-removed" {
				if err != nil {
					t.Fatal(err)
				}
				items, err := e.trashList()
				if err != nil || len(items) != 1 {
					t.Fatal(items, err)
				}
			} else {
				if err == nil {
					t.Fatal("unsafe migration succeeded")
				}
				if _, err = os.Stat(old); err != nil {
					t.Fatal("old payload lost", err)
				}
				if _, err = os.Stat(filepath.Join(e.path("trash"), item.ID+".json")); err != nil {
					t.Fatal("old record lost")
				}
			}
		})
	}
}
func TestTrashRootAlwaysExcluded(t *testing.T) {
	e := fixture(t)
	e.prepareTrash()
	os.WriteFile(filepath.Join(e.trashBase(), "should-not-scan.m4a.aac"), []byte("same"), 0600)
	put(t, e, "visible.m4a.aac", "same")
	r := runScan(t, e)
	if len(r.Groups) != 0 {
		t.Fatal("trash included in duplicates")
	}
	p := previewPlan(t, e, false, true)
	if len(p.Items) != 1 || strings.Contains(p.Items[0].Path, "Android") {
		t.Fatal(p)
	}
}
func TestBatchCleanupMoreThan500(t *testing.T) {
	e := fixture(t)
	put(t, e, "原件.m4a", "same")
	for i := 1; i <= 505; i++ {
		put(t, e, fmt.Sprintf("原件(%d).m4a", i), "same")
	}
	r := runScan(t, e)
	if _, err := e.dispatch("select", Request{Scan: r.ID, Mode: "suggest-all"}); err != nil {
		t.Fatal(err)
	}
	sel := e.selection(r)
	ids := []string{}
	for id := range sel.IDs {
		ids = append(ids, id)
	}
	save(e.path("clean-plan.json"), Request{Scan: r.ID, IDs: ids})
	if err := e.runClean(Request{Scan: r.ID}); err != nil {
		t.Fatal(err)
	}
	if e.status.Moved != 505 || e.status.Done != 505 || e.status.Phase != "done" {
		t.Fatal(e.status)
	}
	if _, err := os.Stat(filepath.Join(e.Storage, "原件.m4a")); err != nil {
		t.Fatal("original lost")
	}
	items, err := e.trashList()
	if err != nil || len(items) != 505 {
		t.Fatal(len(items), err)
	}
	if len(e.selection(report(t, e)).IDs) != 0 {
		t.Fatal("moved selections not pruned")
	}
}
func TestBatchCancelAndChangedKeeper(t *testing.T) {
	for _, scenario := range []string{"cancel", "changed-keeper"} {
		t.Run(scenario, func(t *testing.T) {
			e := fixture(t)
			put(t, e, "原件.m4a", "same")
			put(t, e, "原件(1).m4a", "same")
			r := runScan(t, e)
			e.dispatch("select", Request{Scan: r.ID, Mode: "suggest-all"})
			ids := []string{}
			for id := range e.selection(r).IDs {
				ids = append(ids, id)
			}
			save(e.path("clean-plan.json"), Request{Scan: r.ID, IDs: ids})
			if scenario == "cancel" {
				os.WriteFile(e.path("cancel"), []byte("1"), 0600)
			} else {
				os.WriteFile(filepath.Join(e.Storage, "原件.m4a"), []byte("else"), 0600)
			}
			if err := e.runClean(Request{Scan: r.ID}); err != nil {
				t.Fatal(err)
			}
			if e.status.Moved != 0 {
				t.Fatal("moved file without verified keeper")
			}
			if scenario == "cancel" && e.status.Phase != "cancelled" {
				t.Fatal(e.status)
			}
			if _, err := os.Stat(filepath.Join(e.Storage, "原件(1).m4a")); err != nil {
				t.Fatal("unprocessed file lost")
			}
		})
	}
}
