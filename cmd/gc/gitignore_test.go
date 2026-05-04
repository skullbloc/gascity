package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gastownhall/gascity/internal/fsys"
)

func TestEnsureGitignoreEntries_CreatesNewFile(t *testing.T) {
	f := fsys.NewFake()

	if err := ensureGitignoreEntries(f, "/city", []string{".gc/", ".beads/*", "!.beads/config.yaml", "!.beads/metadata.json"}); err != nil {
		t.Fatalf("ensureGitignoreEntries: %v", err)
	}

	got := string(f.Files[filepath.Join("/city", ".gitignore")])
	for _, want := range []string{".gc/", ".beads/*", "!.beads/config.yaml", "!.beads/metadata.json"} {
		if !strings.Contains(got, want) {
			t.Errorf(".gitignore missing %q; got:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "# Gas City") {
		t.Error(".gitignore missing section header '# Gas City'")
	}
}

func TestEnsureGitignoreEntries_RigEntriesTrackCanonicalBeadsFilesOnly(t *testing.T) {
	f := fsys.NewFake()

	if err := ensureGitignoreEntries(f, "/rig", rigGitignoreEntries); err != nil {
		t.Fatalf("ensureGitignoreEntries: %v", err)
	}

	got := string(f.Files[filepath.Join("/rig", ".gitignore")])
	for _, want := range rigGitignoreEntries {
		if !strings.Contains(got, want) {
			t.Errorf("rig .gitignore missing %q; got:\n%s", want, got)
		}
	}
	for _, forbidden := range []string{".gc/", "hooks/", ".runtime/"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("rig .gitignore should not contain %q; got:\n%s", forbidden, got)
		}
	}
}

func TestEnsureGitignoreEntries_SkipsExisting(t *testing.T) {
	f := fsys.NewFake()
	f.Files[filepath.Join("/city", ".gitignore")] = []byte(".gc/\n.beads/\n/.beads/\nnode_modules/\n")

	if err := ensureGitignoreEntries(f, "/city", []string{".gc/", ".beads/*", "!.beads/config.yaml", "!.beads/metadata.json"}); err != nil {
		t.Fatalf("ensureGitignoreEntries: %v", err)
	}

	got := string(f.Files[filepath.Join("/city", ".gitignore")])
	// .gc/ should appear only once (the original).
	if strings.Count(got, ".gc/") != 1 {
		t.Errorf(".gc/ appears %d times, want 1; got:\n%s", strings.Count(got, ".gc/"), got)
	}
	// Canonical .beads rules should be added.
	for _, want := range []string{".beads/*", "!.beads/config.yaml", "!.beads/metadata.json"} {
		if !strings.Contains(got, want) {
			t.Errorf(".gitignore missing %q; got:\n%s", want, got)
		}
	}
	for _, legacy := range []string{"\n.beads/\n", "\n/.beads/\n"} {
		if strings.Contains("\n"+got, legacy) {
			t.Errorf("legacy .beads ignore %q should be removed; got:\n%s", strings.TrimSpace(legacy), got)
		}
	}
	// Original content preserved.
	if !strings.Contains(got, "node_modules/") {
		t.Errorf("original content lost; got:\n%s", got)
	}
}

func TestEnsureGitignoreEntries_Idempotent(t *testing.T) {
	f := fsys.NewFake()

	entries := cityGitignoreEntries
	for i := 0; i < 3; i++ {
		if err := ensureGitignoreEntries(f, "/city", entries); err != nil {
			t.Fatalf("pass %d: ensureGitignoreEntries: %v", i, err)
		}
	}

	got := string(f.Files[filepath.Join("/city", ".gitignore")])
	// Count exact line matches; substring counting double-counts entries
	// that share prefixes (e.g. ".beads/*" inside "!.beads/*.jsonl").
	lineCounts := make(map[string]int)
	for _, line := range strings.Split(got, "\n") {
		lineCounts[line]++
	}
	for _, entry := range entries {
		if lineCounts[entry] != 1 {
			t.Errorf("%q appears as a line %d times after 3 passes, want 1; got:\n%s",
				entry, lineCounts[entry], got)
		}
	}
}

func TestEnsureGitignoreEntries_NoOpWhenAllPresent(t *testing.T) {
	f := fsys.NewFake()
	original := ".gc/\n.beads/*\n!.beads/config.yaml\n!.beads/metadata.json\n!.beads/*.jsonl\nhooks/\n.runtime/\n"
	f.Files[filepath.Join("/city", ".gitignore")] = []byte(original)

	if err := ensureGitignoreEntries(f, "/city", cityGitignoreEntries); err != nil {
		t.Fatalf("ensureGitignoreEntries: %v", err)
	}

	got := string(f.Files[filepath.Join("/city", ".gitignore")])
	if got != original {
		t.Errorf("file was modified when it shouldn't have been;\nwant: %q\ngot:  %q", original, got)
	}
}

func TestDoInit_WritesGitignoreEntries(t *testing.T) {
	f := fsys.NewFake()

	var stdout, stderr bytes.Buffer
	code := doInit(f, "/bright-lights", defaultWizardConfig(), "", &stdout, &stderr)
	if code != 0 {
		t.Fatalf("doInit = %d, want 0; stderr: %s", code, stderr.String())
	}

	gitignorePath := filepath.Join("/bright-lights", ".gitignore")
	data, ok := f.Files[gitignorePath]
	if !ok {
		t.Fatal(".gitignore not created by doInit")
	}
	got := string(data)
	for _, want := range cityGitignoreEntries {
		if !strings.Contains(got, want) {
			t.Errorf(".gitignore missing %q; got:\n%s", want, got)
		}
	}
}

func TestDoInit_GitignoreIdempotent(t *testing.T) {
	f := fsys.NewFake()

	var stdout, stderr bytes.Buffer
	code := doInit(f, "/bright-lights", defaultWizardConfig(), "", &stdout, &stderr)
	if code != 0 {
		t.Fatalf("first doInit = %d, want 0; stderr: %s", code, stderr.String())
	}

	first := string(f.Files[filepath.Join("/bright-lights", ".gitignore")])

	// Run ensureGitignoreEntries again (simulating a second init-like operation).
	if err := ensureGitignoreEntries(f, "/bright-lights", cityGitignoreEntries); err != nil {
		t.Fatalf("second ensureGitignoreEntries: %v", err)
	}

	second := string(f.Files[filepath.Join("/bright-lights", ".gitignore")])
	if first != second {
		t.Errorf("gitignore changed on second pass;\nfirst:  %q\nsecond: %q", first, second)
	}
}

// TestGitignoreEntries_AllowJSONLBeadsExports asserts the bd auto-export
// contract: `.beads/*.jsonl` files (issues.jsonl, interactions.jsonl,
// routes.jsonl) must NOT be ignored. Without the carve-out, `git add
// .beads/*.jsonl` fails with "paths are ignored", and bd auto-export
// emits "Warning: auto-export: git add failed: exit status 1" on every
// write. The negation must follow the broad `.beads/*` rule for git's
// gitignore semantics to apply it.
func TestGitignoreEntries_AllowJSONLBeadsExports(t *testing.T) {
	for name, entries := range map[string][]string{
		"city": cityGitignoreEntries,
		"rig":  rigGitignoreEntries,
	} {
		t.Run(name, func(t *testing.T) {
			ignoreIdx, allowIdx := -1, -1
			for i, e := range entries {
				switch e {
				case ".beads/*":
					ignoreIdx = i
				case "!.beads/*.jsonl":
					allowIdx = i
				}
			}
			if ignoreIdx < 0 {
				t.Fatalf("%s entries missing %q rule; got: %v", name, ".beads/*", entries)
			}
			if allowIdx < 0 {
				t.Fatalf("%s entries missing %q carve-out for bd auto-export; got: %v", name, "!.beads/*.jsonl", entries)
			}
			if allowIdx < ignoreIdx {
				t.Errorf("%s entries: %q at index %d must come AFTER %q at index %d (gitignore negation order); got: %v",
					name, "!.beads/*.jsonl", allowIdx, ".beads/*", ignoreIdx, entries)
			}
		})
	}
}

// TestEnsureGitignoreEntries_PreservesNegationOrder asserts that the
// rendered .gitignore preserves slice order so that the `!.beads/*.jsonl`
// negation appears below the broad `.beads/*` rule. Git's gitignore
// semantics require the negation to follow the prior matching rule.
func TestEnsureGitignoreEntries_PreservesNegationOrder(t *testing.T) {
	f := fsys.NewFake()

	if err := ensureGitignoreEntries(f, "/rig", rigGitignoreEntries); err != nil {
		t.Fatalf("ensureGitignoreEntries: %v", err)
	}

	got := string(f.Files[filepath.Join("/rig", ".gitignore")])
	ignoreAt := strings.Index(got, "\n.beads/*\n")
	allowAt := strings.Index(got, "\n!.beads/*.jsonl\n")
	if ignoreAt < 0 {
		t.Fatalf("rendered .gitignore missing %q on its own line; got:\n%s", ".beads/*", got)
	}
	if allowAt < 0 {
		t.Fatalf("rendered .gitignore missing %q on its own line; got:\n%s", "!.beads/*.jsonl", got)
	}
	if allowAt < ignoreAt {
		t.Errorf("rendered .gitignore has %q before %q; negation will not apply; got:\n%s",
			"!.beads/*.jsonl", ".beads/*", got)
	}
}

func TestDoInit_GitignorePreservesUserEntries(t *testing.T) {
	f := fsys.NewFake()
	// Pre-populate a .gitignore with user content.
	userContent := "node_modules/\n*.log\n"
	f.Files[filepath.Join("/bright-lights", ".gitignore")] = []byte(userContent)
	// Pre-populate city.toml so doInit sees it as existing city (bootstrap path).
	// Instead, just test ensureGitignoreEntries directly since doInit won't
	// run on a directory that already has a scaffold.
	if err := ensureGitignoreEntries(f, "/bright-lights", cityGitignoreEntries); err != nil {
		t.Fatalf("ensureGitignoreEntries: %v", err)
	}

	got := string(f.Files[filepath.Join("/bright-lights", ".gitignore")])
	if !strings.Contains(got, "node_modules/") {
		t.Error("user entry 'node_modules/' was lost")
	}
	if !strings.Contains(got, "*.log") {
		t.Error("user entry '*.log' was lost")
	}
	for _, want := range cityGitignoreEntries {
		if !strings.Contains(got, want) {
			t.Errorf("missing city entry %q; got:\n%s", want, got)
		}
	}
}
