package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestHelperWakeChild(t *testing.T) {
	mode := os.Args[len(os.Args)-1]
	switch mode {
	case "helper-success":
		fmt.Println("Starting service: Intent")
	case "helper-error":
		fmt.Println("Error: background service denied")
	case "helper-stall":
		time.Sleep(time.Minute)
	default:
		return
	}
	os.Exit(0)
}

func TestHelperWakeDetectsSystemFailureAndTimeout(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"-test.run=^TestHelperWakeChild$", "--", "helper-success"}
	if err = runHelperWake(5*time.Second, self, args...); err != nil {
		t.Fatal(err)
	}
	args[2] = "helper-error"
	if err = runHelperWake(5*time.Second, self, args...); err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("zero-exit system errors must be reported: %v", err)
	}
	args[2] = "helper-stall"
	start := time.Now()
	if err = runHelperWake(250*time.Millisecond, self, args...); err == nil || !strings.Contains(err.Error(), "超时") {
		t.Fatalf("stalled service launch must time out: %v", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("timed-out service launch blocked monitoring")
	}
}
