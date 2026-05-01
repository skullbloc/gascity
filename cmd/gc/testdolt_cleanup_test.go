package main

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// addTestSpawnedDoltCleanup registers a t.Cleanup that scans each
// directory under root for known dolt sql-server PID files and
// terminates the recorded processes (SIGTERM, then SIGKILL after a
// grace period).
//
// Two PID file conventions are recognized:
//
//   - bd-spawned servers:    <scope>/.beads/dolt-server.pid
//   - gc-managed servers:    <city>/.gc/runtime/packs/dolt/dolt.pid
//
// Use this in any test that may spawn a dolt sql-server child (directly
// via `bd init --server` or transitively via `gc dolt-state start`,
// `startBeadsLifecycle`, etc.). Without this cleanup, the dolt children
// outlive the test, get reparented to user systemd, and accumulate as
// "(deleted)"-cwd zombies that the deacon must reap.
//
// The cleanup is best-effort: missing PID files, unreadable files, and
// already-exited processes are silently skipped. Production dolt
// servers are not at risk because their PID files live outside the
// test's t.TempDir() roots.
func addTestSpawnedDoltCleanup(t *testing.T, roots ...string) {
	t.Helper()
	t.Cleanup(func() {
		var pids []int
		for _, root := range roots {
			pids = append(pids, collectTestSpawnedDoltPIDs(root)...)
		}
		terminatePIDsWithGrace(pids, 2*time.Second)
	})
}

// collectTestSpawnedDoltPIDs walks root for files named dolt-server.pid
// or dolt.pid (the two conventions used by bd and gc respectively) and
// returns the PIDs they contain. Best-effort: walk errors and unreadable
// PID files are skipped silently.
func collectTestSpawnedDoltPIDs(root string) []int {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	var pids []int
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		switch d.Name() {
		case "dolt-server.pid", "dolt.pid":
		default:
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
		if parseErr != nil || pid <= 0 {
			return nil
		}
		pids = append(pids, pid)
		return nil
	})
	return pids
}

// terminatePIDsWithGrace SIGTERMs each pid, waits up to grace for them
// to exit, then SIGKILLs any that remain. Caller is responsible for
// ensuring the PIDs are appropriate to kill (e.g. recorded by the test).
func terminatePIDsWithGrace(pids []int, grace time.Duration) {
	if len(pids) == 0 {
		return
	}
	for _, pid := range pids {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
	deadline := time.Now().Add(grace)
	for time.Now().Before(deadline) {
		anyAlive := false
		for _, pid := range pids {
			if syscall.Kill(pid, 0) == nil {
				anyAlive = true
				break
			}
		}
		if !anyAlive {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	for _, pid := range pids {
		if syscall.Kill(pid, 0) == nil {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
}

func TestAddTestSpawnedDoltCleanupTerminatesProcessFromPIDFile(t *testing.T) {
	root := t.TempDir()

	cmd := exec.Command("sleep", "120")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start dummy process: %v", err)
	}
	pid := cmd.Process.Pid
	waited := make(chan struct{})
	go func() {
		_ = cmd.Wait() // reap so a successful SIGTERM clears the zombie
		close(waited)
	}()
	t.Cleanup(func() {
		// Belt-and-suspenders: never let the dummy leak even if the
		// helper under test fails.
		_ = syscall.Kill(pid, syscall.SIGKILL)
		<-waited
	})

	beadsDir := filepath.Join(root, "rig", ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir beads: %v", err)
	}
	if err := os.WriteFile(filepath.Join(beadsDir, "dolt-server.pid"), []byte(strconv.Itoa(pid)+"\n"), 0o644); err != nil {
		t.Fatalf("write pid file: %v", err)
	}

	pids := collectTestSpawnedDoltPIDs(root)
	if len(pids) != 1 || pids[0] != pid {
		t.Fatalf("collectTestSpawnedDoltPIDs(%q) = %v, want [%d]", root, pids, pid)
	}

	terminatePIDsWithGrace(pids, 3*time.Second)

	select {
	case <-waited:
	case <-time.After(3 * time.Second):
		t.Fatalf("process %d still running after terminatePIDsWithGrace", pid)
	}
}

func TestAddTestSpawnedDoltCleanupRecognizesGcManagedPIDFile(t *testing.T) {
	root := t.TempDir()

	cmd := exec.Command("sleep", "120")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start dummy process: %v", err)
	}
	pid := cmd.Process.Pid
	t.Cleanup(func() {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		_ = cmd.Wait()
	})

	doltDir := filepath.Join(root, ".gc", "runtime", "packs", "dolt")
	if err := os.MkdirAll(doltDir, 0o755); err != nil {
		t.Fatalf("mkdir dolt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(doltDir, "dolt.pid"), []byte(strconv.Itoa(pid)+"\n"), 0o644); err != nil {
		t.Fatalf("write pid file: %v", err)
	}

	pids := collectTestSpawnedDoltPIDs(root)
	if len(pids) != 1 || pids[0] != pid {
		t.Fatalf("collectTestSpawnedDoltPIDs(%q) = %v, want [%d]", root, pids, pid)
	}
}

func TestCollectTestSpawnedDoltPIDsIgnoresMalformedFiles(t *testing.T) {
	root := t.TempDir()
	beadsDir := filepath.Join(root, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cases := []struct {
		name    string
		content string
	}{
		{"empty", ""},
		{"non-numeric", "not-a-pid\n"},
		{"negative", "-1\n"},
		{"zero", "0\n"},
	}
	for _, tc := range cases {
		if err := os.WriteFile(filepath.Join(beadsDir, "dolt-server.pid"), []byte(tc.content), 0o644); err != nil {
			t.Fatalf("write %s: %v", tc.name, err)
		}
		if pids := collectTestSpawnedDoltPIDs(root); len(pids) != 0 {
			t.Fatalf("%s: collectTestSpawnedDoltPIDs = %v, want empty", tc.name, pids)
		}
	}
}

func TestCollectTestSpawnedDoltPIDsHandlesMissingRoot(t *testing.T) {
	if pids := collectTestSpawnedDoltPIDs(""); pids != nil {
		t.Fatalf("collectTestSpawnedDoltPIDs(\"\") = %v, want nil", pids)
	}
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if pids := collectTestSpawnedDoltPIDs(missing); len(pids) != 0 {
		t.Fatalf("collectTestSpawnedDoltPIDs(%q) = %v, want empty", missing, pids)
	}
}
