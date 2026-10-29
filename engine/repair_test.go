package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func previewPlan(t *testing.T, e *Engine, fixTime, rename bool) RepairPlan {
	t.Helper()
	if err := e.previewRepairs(Request{Root: e.Storage, RepairTime: fixTime, RenameAAC: rename, Zone: "Asia/Shanghai"}); err != nil {
		t.Fatal(err)
	}
	var p RepairPlan
	if err := read(e.path("repair.json"), &p); err != nil {
		t.Fatal(err)
	}
	return p
}
func repairResult(t *testing.T, e *Engine, p RepairPlan, ids ...string) map[string]any {
	t.Helper()
	v, err := e.dispatch("repair-apply", Request{Scan: p.ID, IDs: ids})
	if err != nil {
		t.Fatal(err)
	}
	return v.(map[string]any)
}
func oldDate(t *testing.T, path string) time.Time {
	t.Helper()
	ts := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, ts, ts); err != nil {
		t.Fatal(err)
	}
	return ts
}
func TestFilenameDateParsing(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"微信 中华 2025-02-12 14-40-15.m4a", "微信 中华 2025-02-12 14-40-15.m4a.aac", "2025-02-12_14-40-15.aac"} {
		dt, err := dateFromName(name, loc)
		if err != nil || dt.Unix() != 1739342415 {
			t.Fatalf("%s %v %v", name, dt, err)
		}
	}
	for _, name := range []string{"2025-02-30 14-40-15.aac", "2025-02-12 25-40-15.aac", "1969-12-31 23-59-59.aac", "2025-02-12 14-40-150.aac", "12025-02-12 14-40-15.aac", "2025-02-12 14-40-15 2025-02-13 14-40-15.aac", "no date.aac"} {
		if _, err := dateFromName(name, loc); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	utc, _ := dateFromName("2025-02-12 14-40-15.aac", time.UTC)
	local, _ := dateFromName("2025-02-12 14-40-15.aac", loc)
	if utc.Sub(local) != 8*time.Hour {
		t.Fatal("wrong timezone offset")
	}
}
func TestRepairTimeOnly(t *testing.T) {
	e := fixture(t)
	path := put(t, e, "微信 中华 2025-02-12 14-40-15.m4a", "audio-bytes")
	old := oldDate(t, path)
	plan := previewPlan(t, e, true, false)
	if len(plan.Items) != 1 || plan.Items[0].Target != path || plan.Items[0].NewTime == nil {
		t.Fatal(plan)
	}
	info, _ := os.Stat(path)
	if !info.ModTime().Equal(old) {
		t.Fatal("preview changed timestamp")
	}
	result := repairResult(t, e, plan, plan.Items[0].ID)
	if result["done"] != 1 || result["timed"] != 1 || result["renamed"] != 0 {
		t.Fatal(result)
	}
	info, _ = os.Stat(path)
	if info.ModTime().Unix() != 1739342415 {
		t.Fatal(info.ModTime())
	}
	b, _ := os.ReadFile(path)
	if string(b) != "audio-bytes" {
		t.Fatal("content changed")
	}
	if len(previewPlan(t, e, true, false).Items) != 0 {
		t.Fatal("not idempotent")
	}
}
func TestRepairRenameOnly(t *testing.T) {
	e := fixture(t)
	path := put(t, e, "微信 中华 2025-02-12 14-40-15.m4a.aac", "abc")
	old := oldDate(t, path)
	put(t, e, "ordinary.m4a", "keep")
	put(t, e, "name.m4a.aac.bak", "keep")
	p := previewPlan(t, e, false, true)
	if len(p.Items) != 1 || p.Items[0].NewTime != nil {
		t.Fatal(p)
	}
	result := repairResult(t, e, p, p.Items[0].ID)
	if result["done"] != 1 || result["timed"] != 0 || result["renamed"] != 1 {
		t.Fatal(result)
	}
	dst := aacTarget(path)
	b, err := os.ReadFile(dst)
	if err != nil || string(b) != "abc" {
		t.Fatal(string(b), err)
	}
	info, _ := os.Stat(dst)
	if !info.ModTime().Equal(old) {
		t.Fatal("rename changed modification time")
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("old path still exists")
	}
	if len(previewPlan(t, e, false, true).Items) != 0 {
		t.Fatal("rename is not idempotent")
	}
}
func TestRepairCombinedAndJournal(t *testing.T) {
	e := fixture(t)
	path := put(t, e, "微信 中华 2025-02-12 14-40-15.M4A.AAC", "audio")
	old := oldDate(t, path)
	p := previewPlan(t, e, true, true)
	save(e.path("report.json"), Report{ID: "old"})
	r := repairResult(t, e, p, p.Items[0].ID)
	if r["done"] != 1 || r["renamed"] != 1 || r["timed"] != 1 {
		t.Fatal(r)
	}
	info, err := os.Stat(aacTarget(path))
	if err != nil || info.ModTime().Unix() != 1739342415 {
		t.Fatal(info, err)
	}
	if _, err = os.Stat(e.path("report.json")); !os.IsNotExist(err) {
		t.Fatal("stale duplicate report survived")
	}
	logs, _ := os.ReadDir(e.path("repairs"))
	if len(logs) != 1 {
		t.Fatal(logs)
	}
	var log RepairLog
	if read(filepath.Join(e.path("repairs"), logs[0].Name()), &log) != nil || log.State != "done" || log.Item.OldTime != old.Unix() || log.Item.Path != path {
		t.Fatal(log)
	}
	if _, err = e.dispatch("repair-apply", Request{Scan: p.ID, IDs: []string{p.Items[0].ID}}); err == nil {
		t.Fatal("repeated apply accepted")
	}
}
func TestRepairConflictsAndStaleFile(t *testing.T) {
	for _, scenario := range []string{"preview-conflict", "later-conflict", "content-change", "time-change"} {
		t.Run(scenario, func(t *testing.T) {
			e := fixture(t)
			src := put(t, e, "微信 中华 2025-02-12 14-40-15.m4a.aac", "abc")
			oldDate(t, src)
			dst := aacTarget(src)
			if scenario == "preview-conflict" {
				put(t, e, filepath.Base(dst), "existing")
			}
			p := previewPlan(t, e, true, true)
			item := p.Items[0]
			for _, candidate := range p.Items {
				if candidate.Path == src {
					item = candidate
					break
				}
			}
			switch scenario {
			case "preview-conflict":
				if item.Problem == "" {
					t.Fatal("missing conflict")
				}
				if _, err := e.dispatch("repair-apply", Request{Scan: p.ID, IDs: []string{item.ID}}); err == nil {
					t.Fatal("accepted blocked item")
				}
			case "later-conflict":
				put(t, e, filepath.Base(dst), "existing")
				r := repairResult(t, e, p, item.ID)
				if r["done"] != 0 {
					t.Fatal(r)
				}
				b, _ := os.ReadFile(dst)
				if string(b) != "existing" {
					t.Fatal("overwrote destination")
				}
			case "content-change":
				os.WriteFile(src, []byte("xyz"), 0600)
				r := repairResult(t, e, p, item.ID)
				if r["done"] != 0 {
					t.Fatal(r)
				}
			case "time-change":
				os.Chtimes(src, time.Now(), time.Now())
				r := repairResult(t, e, p, item.ID)
				if r["done"] != 0 {
					t.Fatal(r)
				}
			}
			if _, err := os.Stat(src); err != nil {
				t.Fatal("source lost", err)
			}
		})
	}
}
func TestRepairInvalidDateCanRename(t *testing.T) {
	e := fixture(t)
	path := put(t, e, "微信 中华 2025-02-30 14-40-15.m4a.aac", "abc")
	old := oldDate(t, path)
	p := previewPlan(t, e, true, true)
	if len(p.Items) != 1 || p.Items[0].Warning == "" || p.Items[0].NewTime != nil || p.Unmatched != 1 {
		t.Fatal(p)
	}
	r := repairResult(t, e, p, p.Items[0].ID)
	if r["done"] != 1 || r["timed"] != 0 {
		t.Fatal(r)
	}
	info, _ := os.Stat(aacTarget(path))
	if !info.ModTime().Equal(old) {
		t.Fatal("invalid date applied")
	}
}
func TestRepairSelectionAndStalePreview(t *testing.T) {
	e := fixture(t)
	for _, name := range []string{"a.m4a.aac", "b.m4a.aac"} {
		put(t, e, name, "abc")
	}
	p := previewPlan(t, e, false, true)
	if _, err := e.dispatch("repair-apply", Request{Scan: "stale", IDs: []string{p.Items[0].ID}}); err == nil {
		t.Fatal("accepted stale token")
	}
	r := repairResult(t, e, p, p.Items[0].ID)
	if r["done"] != 1 {
		t.Fatal(r)
	}
	if _, err := os.Stat(p.Items[1].Path); err != nil {
		t.Fatal("changed unselected file")
	}
	v, err := e.repairPage(Request{})
	if err != nil || v.(map[string]any)["total"] != 1 {
		t.Fatal(v, err)
	}
}
func TestRepairCancelledAndInvalidOptions(t *testing.T) {
	e := fixture(t)
	put(t, e, "a.m4a.aac", "abc")
	p := previewPlan(t, e, false, true)
	os.WriteFile(e.path("cancel"), []byte("1"), 0600)
	if err := e.previewRepairs(Request{Root: e.Storage, RenameAAC: true, Zone: "Asia/Shanghai"}); err != cancelled {
		t.Fatal(err)
	}
	var after RepairPlan
	read(e.path("repair.json"), &after)
	if after.ID != p.ID {
		t.Fatal("cancel destroyed preview")
	}
	if err := e.previewRepairs(Request{Root: e.Storage}); err == nil {
		t.Fatal("accepted no options")
	}
	if err := e.previewRepairs(Request{Root: e.Storage, RepairTime: true, Zone: "invalid/zone"}); err == nil {
		t.Fatal("accepted invalid zone")
	}
}
func TestCopyMoveFallback(t *testing.T) {
	for _, scenario := range []string{"ok", "conflict", "wrong-hash", "changed-source"} {
		t.Run(scenario, func(t *testing.T) {
			e := fixture(t)
			src := put(t, e, "a.m4a.aac", "audio")
			old := oldDate(t, src)
			hash, info, err := e.digest(src, false)
			if err != nil {
				t.Fatal(err)
			}
			dst := aacTarget(src)
			switch scenario {
			case "conflict":
				put(t, e, filepath.Base(dst), "existing")
			case "wrong-hash":
				hash = "bad"
			case "changed-source":
				os.Chtimes(src, time.Now(), time.Now())
			}
			err = e.copyMove(src, dst, hash, info, old)
			if scenario == "ok" {
				if err != nil {
					t.Fatal(err)
				}
				got, _ := os.Stat(dst)
				if got.ModTime() != info.ModTime() {
					t.Fatal("mtime changed")
				}
				if _, err = os.Stat(src); !os.IsNotExist(err) {
					t.Fatal("source survived successful move")
				}
			} else {
				if err == nil {
					t.Fatal("unsafe fallback succeeded")
				}
				if _, err = os.Stat(src); err != nil {
					t.Fatal("source lost")
				}
				if scenario == "conflict" {
					b, _ := os.ReadFile(dst)
					if string(b) != "existing" {
						t.Fatal("overwritten")
					}
				} else if _, err = os.Stat(dst); !os.IsNotExist(err) {
					t.Fatal("temporary copy survived")
				}
			}
		})
	}
}
