package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatchEnableBeforeDirectories(t *testing.T) {
	e, configured, source, _ := watchFixture(t)
	empty := WatchConfig{Enabled: true, StableSeconds: 10, Rules: []WatchRule{}}
	if _, err := e.watchSave(empty); err != nil {
		t.Fatal(err)
	}
	reopened := &Engine{Data: e.Data, Storage: e.Storage}
	if c := reopened.watchConfig(); !c.Enabled || len(c.Rules) != 0 {
		t.Fatalf("master switch lost after reopening: %+v", c)
	}
	if err := reopened.watchTick(time.Now()); err != nil {
		t.Fatal(err)
	}
	state, err := reopened.watchState()
	if err != nil || len(state.Entries) != 0 || len(state.Items) != 0 {
		t.Fatalf("empty rules must not monitor storage: %+v, %v", state, err)
	}
	// Adding the first rule while enabled establishes a baseline for existing files.
	file := filepath.Join(source, "existing.zip")
	if err := os.WriteFile(file, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.watchSave(configured); err != nil {
		t.Fatal(err)
	}
	state, err = reopened.watchState()
	if err != nil || len(state.Entries) != 1 || len(state.Items) != 0 {
		t.Fatalf("first rule must baseline existing files: %+v, %v", state, err)
	}
	for _, entry := range state.Entries {
		if !entry.Baseline {
			t.Fatal("existing file was not baselined")
		}
	}
}
