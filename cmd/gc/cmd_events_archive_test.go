package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEventsArchiveRotatesWhenIdle(t *testing.T) {
	dir := t.TempDir()
	gcDir := filepath.Join(dir, ".gc")
	if err := os.MkdirAll(gcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	evPath := filepath.Join(gcDir, "events.jsonl")
	if err := os.WriteFile(evPath, []byte("payload\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	withEventsArchiveStubs(t, 0, func(_ string) int { return 0 })

	var stdout, stderr bytes.Buffer
	code := cmdEventsArchive(dir, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("cmdEventsArchive = %d; stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(evPath); !os.IsNotExist(err) {
		t.Errorf("events.jsonl should be removed; stat err = %v", err)
	}

	entries, err := os.ReadDir(gcDir)
	if err != nil {
		t.Fatal(err)
	}
	var archives []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "events-") && strings.HasSuffix(e.Name(), ".jsonl") {
			archives = append(archives, e.Name())
		}
	}
	if len(archives) != 1 {
		t.Fatalf("got %d archive files (%v), want 1", len(archives), archives)
	}
	if !strings.Contains(stdout.String(), "archived to") {
		t.Errorf("stdout should report archive path; got %q", stdout.String())
	}
}

func TestEventsArchiveRefusesWhenSupervisorAlive(t *testing.T) {
	dir := t.TempDir()
	gcDir := filepath.Join(dir, ".gc")
	if err := os.MkdirAll(gcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	evPath := filepath.Join(gcDir, "events.jsonl")
	if err := os.WriteFile(evPath, []byte("payload\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	withEventsArchiveStubs(t, 4321, func(_ string) int { return 0 })

	var stdout, stderr bytes.Buffer
	code := cmdEventsArchive(dir, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("cmdEventsArchive should refuse when supervisor alive; stdout=%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "supervisor") {
		t.Errorf("stderr should mention supervisor; got %q", stderr.String())
	}
	if _, err := os.Stat(evPath); err != nil {
		t.Errorf("events.jsonl should be untouched: %v", err)
	}
}

func TestEventsArchiveRefusesWhenControllerAlive(t *testing.T) {
	dir := t.TempDir()
	gcDir := filepath.Join(dir, ".gc")
	if err := os.MkdirAll(gcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	evPath := filepath.Join(gcDir, "events.jsonl")
	if err := os.WriteFile(evPath, []byte("payload\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	withEventsArchiveStubs(t, 0, func(_ string) int { return 9999 })

	var stdout, stderr bytes.Buffer
	code := cmdEventsArchive(dir, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("cmdEventsArchive should refuse when controller alive; stdout=%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "controller") {
		t.Errorf("stderr should mention controller; got %q", stderr.String())
	}
	if _, err := os.Stat(evPath); err != nil {
		t.Errorf("events.jsonl should be untouched: %v", err)
	}
}

func TestEventsArchiveSkipsWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	gcDir := filepath.Join(dir, ".gc")
	if err := os.MkdirAll(gcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	evPath := filepath.Join(gcDir, "events.jsonl")
	if err := os.WriteFile(evPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	withEventsArchiveStubs(t, 0, func(_ string) int { return 0 })

	var stdout, stderr bytes.Buffer
	code := cmdEventsArchive(dir, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("cmdEventsArchive = %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "nothing to archive") {
		t.Errorf("stdout should report nothing to archive; got %q", stdout.String())
	}
	if _, err := os.Stat(evPath); err != nil {
		t.Errorf("events.jsonl should remain after no-op: %v", err)
	}
}

func TestEventsArchiveSkipsWhenMissing(t *testing.T) {
	dir := t.TempDir()
	gcDir := filepath.Join(dir, ".gc")
	if err := os.MkdirAll(gcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	withEventsArchiveStubs(t, 0, func(_ string) int { return 0 })

	var stdout, stderr bytes.Buffer
	code := cmdEventsArchive(dir, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("cmdEventsArchive = %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "nothing to archive") {
		t.Errorf("stdout should report nothing to archive; got %q", stdout.String())
	}
}

func withEventsArchiveStubs(t *testing.T, supervisorPID int, controllerAliveFn func(string) int) {
	t.Helper()
	oldAlive := supervisorAliveHook
	oldController := eventsControllerAliveHook
	supervisorAliveHook = func() int { return supervisorPID }
	eventsControllerAliveHook = controllerAliveFn
	t.Cleanup(func() {
		supervisorAliveHook = oldAlive
		eventsControllerAliveHook = oldController
	})
}
