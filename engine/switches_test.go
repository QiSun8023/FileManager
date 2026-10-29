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

func groupSummary(t *testing.T, e *Engine, page int) map[string]any {
	t.Helper()
	v, err := e.groupsPage(Request{Page: page})
	if err != nil {
		t.Fatal(err)
	}
	return v.(map[string]any)["selection"].(map[string]any)
}
func TestDuplicateScopeSwitches(t *testing.T) {
	e := fixture(t)
	for i := 0; i < 28; i++ {
		s := fmt.Sprintf("data%03d", i)
		put(t, e, fmt.Sprintf("文件%d.aac", i), s)
		put(t, e, fmt.Sprintf("文件%d(1).aac", i), s)
	}
	r := runScan(t, e)
	e.dispatch("select", Request{Scan: r.ID, Mode: "toggle-all"})
	s := groupSummary(t, e, 0)
	if s["allState"] != "on" || s["pageState"] != "on" || s["count"] != 28 {
		t.Fatal(s)
	}
	e.dispatch("select", Request{Scan: r.ID, Mode: "toggle-page", Page: 0})
	s = groupSummary(t, e, 0)
	if s["allState"] != "partial" || s["pageState"] != "off" || s["count"] != 3 {
		t.Fatal(s)
	}
	e.dispatch("select", Request{Scan: r.ID, Mode: "toggle-page", Page: 0})
	if groupSummary(t, e, 0)["count"] != 28 {
		t.Fatal("did not refill current page")
	}
	e.dispatch("select", Request{Scan: r.ID, Mode: "toggle-all"})
	if groupSummary(t, e, 0)["count"] != 0 {
		t.Fatal("did not clear all")
	}
}
func TestSwitchPreservesManualKeeper(t *testing.T) {
	e := fixture(t)
	for _, pair := range []string{"甲", "乙"} {
		put(t, e, pair+".aac", pair)
		put(t, e, pair+"(1).aac", pair)
	}
	r := runScan(t, e)
	groups := e.activeGroups(r)
	original := groups[0].Files[0].ID
	copyID := groups[0].Files[1].ID
	e.dispatch("select", Request{Scan: r.ID, Mode: "toggle", ID: original, Checked: true})
	e.dispatch("select", Request{Scan: r.ID, Mode: "toggle-all"})
	s := e.selection(r)
	if !s.IDs[original] || s.IDs[copyID] || len(s.IDs) != 2 {
		t.Fatal("manual keeper was changed", s)
	}
}
func TestSwitchStateIncludesHiddenGroupItems(t *testing.T) {
	e := fixture(t)
	for i := 0; i < 103; i++ {
		put(t, e, fmt.Sprintf("f%03d", i), "same")
	}
	r := runScan(t, e)
	e.dispatch("select", Request{Scan: r.ID, Mode: "toggle-all"})
	s := groupSummary(t, e, 0)
	if s["allState"] != "on" || s["pageState"] != "on" || s["count"] != 102 {
		t.Fatal(s)
	}
}
func TestRepairSelectionSwitchesAndPersistence(t *testing.T) {
	e := fixture(t)
	for i := 0; i < 61; i++ {
		put(t, e, fmt.Sprintf("f%03d.m4a.aac", i), "audio")
	}
	put(t, e, "f060.aac", "exists")
	plan := previewPlan(t, e, false, true)
	v, err := e.dispatch("select-items", Request{Job: "repair", Scan: plan.ID, Mode: "toggle-all"})
	if err != nil {
		t.Fatal(err)
	}
	s := v.(map[string]any)
	if s["count"] != 60 || s["allState"] != "on" {
		t.Fatal(s)
	}
	v, err = e.repairPage(Request{Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range v.(map[string]any)["items"].([]RepairItem) {
		if item.Problem == "" && !item.Selected {
			t.Fatal("cross-page repair selection lost")
		}
		if item.Problem != "" && item.Selected {
			t.Fatal("blocked item selected")
		}
	}
	fresh := &Engine{Data: e.Data, Storage: e.Storage}
	token, items, _ := fresh.flatItems("repair")
	if len(fresh.flatSelection("repair", token, items).IDs) != 60 {
		t.Fatal("reopen lost repair selection")
	}
	v, err = e.dispatch("select-items", Request{Job: "repair", Scan: plan.ID, Mode: "toggle-page", Page: 1})
	if err != nil || v.(map[string]any)["count"] != 50 {
		t.Fatal(v, err)
	}
	e.dispatch("select-items", Request{Job: "repair", Scan: plan.ID, Mode: "toggle-page", Page: 1})
	v, err = e.dispatch("select-items", Request{Job: "repair", Scan: plan.ID, Mode: "toggle-all"})
	if err != nil || v.(map[string]any)["count"] != 0 {
		t.Fatal(v, err)
	}
}
func seedRecycle(t *testing.T, e *Engine, n int) []string {
	t.Helper()
	if err := e.prepareTrash(); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(e.Storage, "Restored"), 0700)
	hash := sha256.Sum256([]byte("audio"))
	ids := []string{}
	for i := 0; i < n; i++ {
		id := token()
		ids = append(ids, id)
		ensureDir(filepath.Join(e.trashBase(), id))
		os.WriteFile(filepath.Join(e.trashBase(), id, "content"), []byte("audio"), 0600)
		item := Trash{ID: id, Hash: hex.EncodeToString(hash[:]), Size: 5, Path: filepath.Join(e.Storage, "Restored", fmt.Sprintf("file%04d.aac", i)), At: int64(i)}
		if err := save(filepath.Join(e.recordDir(), id+".json"), item); err != nil {
			t.Fatal(err)
		}
	}
	return ids
}
func TestTrashAllSwitchAcrossPages(t *testing.T) {
	e := fixture(t)
	seedRecycle(t, e, 60)
	v, err := e.dispatch("select-items", Request{Job: "trash", Scan: "trash", Mode: "toggle-all"})
	if err != nil || v.(map[string]any)["count"] != 60 {
		t.Fatal(v, err)
	}
	v, err = e.dispatch("trash", Request{Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range v.(map[string]any)["items"].([]Trash) {
		if !item.Selected {
			t.Fatal("trash selection lost on next page")
		}
	}
	v, err = e.dispatch("select-items", Request{Job: "trash", Scan: "trash", Mode: "toggle-all"})
	if err != nil || v.(map[string]any)["count"] != 0 {
		t.Fatal(v, err)
	}
}
func TestSelectionRevisionBlocksChangedConfirmation(t *testing.T) {
	e := fixture(t)
	seedRecycle(t, e, 2)
	e.dispatch("select-items", Request{Job: "trash", Scan: "trash", Mode: "toggle-all"})
	_, err := e.dispatch("start", Request{Job: "purge", Scan: "trash", Revision: "stale"})
	if err == nil || !strings.Contains(err.Error(), "选择已变化") {
		t.Fatal(err)
	}
	items, _ := e.trashList()
	if len(items) != 2 {
		t.Fatal("revision failure deleted files")
	}
}
func TestItemBatchesAbove500(t *testing.T) {
	for _, job := range []string{"restore", "purge", "repair-apply"} {
		t.Run(job, func(t *testing.T) {
			e := fixture(t)
			plan := Request{Job: job, Scan: "trash"}
			if job == "repair-apply" {
				for i := 0; i < 501; i++ {
					put(t, e, fmt.Sprintf("f%04d.m4a.aac", i), "audio")
				}
				p := previewPlan(t, e, false, true)
				plan.Scan = p.ID
				for _, item := range p.Items {
					plan.IDs = append(plan.IDs, item.ID)
				}
			} else {
				plan.IDs = seedRecycle(t, e, 501)
			}
			save(e.path("item-plan.json"), plan)
			if err := e.runItemJob(Request{Job: job, Scan: plan.Scan}); err != nil {
				t.Fatal(err)
			}
			if e.status.Moved != 501 || e.status.Done != 501 || e.status.Phase != "done" {
				t.Fatal(e.status)
			}
			if job == "restore" {
				files, _ := os.ReadDir(filepath.Join(e.Storage, "Restored"))
				if len(files) != 501 {
					t.Fatal("restore did not cover all pages")
				}
			} else if job == "purge" {
				items, _ := e.trashList()
				if len(items) != 0 {
					t.Fatal("purge missed some pages")
				}
			} else {
				if _, err := os.Stat(filepath.Join(e.Storage, "f0500.aac")); err != nil {
					t.Fatal("repair missed last page")
				}
			}
		})
	}
}
func TestItemBatchCancellation(t *testing.T) {
	for _, job := range []string{"restore", "purge", "repair-apply"} {
		t.Run(job, func(t *testing.T) {
			e := fixture(t)
			plan := Request{Job: job, Scan: "trash"}
			if job == "repair-apply" {
				put(t, e, "file.m4a.aac", "audio")
				p := previewPlan(t, e, false, true)
				plan.Scan = p.ID
				plan.IDs = []string{p.Items[0].ID}
			} else {
				plan.IDs = seedRecycle(t, e, 2)
			}
			save(e.path("item-plan.json"), plan)
			os.WriteFile(e.path("cancel"), []byte("1"), 0600)
			if err := e.runItemJob(Request{Job: job, Scan: plan.Scan}); err != nil {
				t.Fatal(err)
			}
			if e.status.Moved != 0 || e.status.Phase != "cancelled" {
				t.Fatal(e.status)
			}
		})
	}
}
