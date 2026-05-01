package events

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestFileRecorderRotateMovesFileAndStartsFresh(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	archivePath := filepath.Join(dir, "events-archive.jsonl")
	var stderr bytes.Buffer

	rec, err := NewFileRecorder(path, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	defer rec.Close() //nolint:errcheck // test cleanup

	rec.Record(Event{Type: BeadCreated, Actor: "human"})
	rec.Record(Event{Type: BeadCreated, Actor: "human"})

	if err := rec.Rotate(archivePath); err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	rec.Record(Event{Type: BeadClosed, Actor: "human"})

	archived, err := ReadAll(archivePath)
	if err != nil {
		t.Fatalf("ReadAll archive: %v", err)
	}
	if len(archived) != 2 {
		t.Fatalf("archive: got %d events, want 2", len(archived))
	}
	if archived[0].Seq != 1 || archived[1].Seq != 2 {
		t.Errorf("archive seqs = [%d, %d], want [1, 2]", archived[0].Seq, archived[1].Seq)
	}

	current, err := ReadAll(path)
	if err != nil {
		t.Fatalf("ReadAll current: %v", err)
	}
	if len(current) != 1 {
		t.Fatalf("current: got %d events, want 1", len(current))
	}
	if current[0].Seq != 1 {
		t.Errorf("post-rotate first event Seq = %d, want 1 (seq reset on rotate)", current[0].Seq)
	}
	if current[0].Type != BeadClosed {
		t.Errorf("post-rotate event Type = %q, want %q", current[0].Type, BeadClosed)
	}
}

func TestFileRecorderRotateOnEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	archivePath := filepath.Join(dir, "events-archive.jsonl")
	var stderr bytes.Buffer

	rec, err := NewFileRecorder(path, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	defer rec.Close() //nolint:errcheck // test cleanup

	if err := rec.Rotate(archivePath); err != nil {
		t.Fatalf("Rotate empty file: %v", err)
	}

	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("archive should exist: %v", err)
	}
	rec.Record(Event{Type: BeadCreated, Actor: "human"})

	current, err := ReadAll(path)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(current) != 1 || current[0].Seq != 1 {
		t.Errorf("got %d events with seq %d, want 1 event with seq 1", len(current), seqOrZero(current))
	}
}

func TestFileRecorderRotateClosed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	archivePath := filepath.Join(dir, "events-archive.jsonl")
	var stderr bytes.Buffer

	rec, err := NewFileRecorder(path, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if err := rec.Close(); err != nil {
		t.Fatal(err)
	}

	if err := rec.Rotate(archivePath); err == nil {
		t.Error("Rotate on closed recorder should error")
	}
}

func TestFileRecorderRotateArchiveCollision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	archivePath := filepath.Join(dir, "events-archive.jsonl")
	var stderr bytes.Buffer

	if err := os.WriteFile(archivePath, []byte("preexisting\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rec, err := NewFileRecorder(path, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	defer rec.Close() //nolint:errcheck // test cleanup

	rec.Record(Event{Type: BeadCreated, Actor: "human"})

	if err := rec.Rotate(archivePath); err == nil {
		t.Error("Rotate to existing archive path should error to avoid clobbering")
	}

	rec.Record(Event{Type: BeadClosed, Actor: "human"})
	current, err := ReadAll(path)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(current) != 2 {
		t.Errorf("recorder should still be writable after failed rotate: got %d events, want 2", len(current))
	}
}

func TestMaybeRotateSkipsBelowThreshold(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	if err := os.WriteFile(path, []byte("small\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rotated, archive, err := MaybeRotate(path, 1024)
	if err != nil {
		t.Fatalf("MaybeRotate: %v", err)
	}
	if rotated {
		t.Error("rotated = true, want false (file below threshold)")
	}
	if archive != "" {
		t.Errorf("archive = %q, want empty", archive)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("original path should still exist: %v", err)
	}
}

func TestMaybeRotateAtOrAboveThresholdRenames(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	payload := bytes.Repeat([]byte("x"), 2048)
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	rotated, archive, err := MaybeRotate(path, 1024)
	if err != nil {
		t.Fatalf("MaybeRotate: %v", err)
	}
	if !rotated {
		t.Fatal("rotated = false, want true")
	}
	if archive == "" {
		t.Fatal("archive path should be returned")
	}
	if filepath.Dir(archive) != dir {
		t.Errorf("archive dir = %q, want %q", filepath.Dir(archive), dir)
	}
	if _, err := os.Stat(archive); err != nil {
		t.Errorf("archive should exist: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("original path should not exist after rotate; stat err = %v", err)
	}
}

func TestMaybeRotateMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")

	rotated, archive, err := MaybeRotate(path, 1024)
	if err != nil {
		t.Fatalf("MaybeRotate on missing file: %v", err)
	}
	if rotated {
		t.Error("rotated = true on missing file, want false")
	}
	if archive != "" {
		t.Errorf("archive = %q, want empty", archive)
	}
}

func TestMaybeRotateAvoidsCollisionWithExistingArchive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), 2048), 0o644); err != nil {
		t.Fatal(err)
	}

	rotated1, archive1, err := MaybeRotate(path, 1024)
	if err != nil || !rotated1 {
		t.Fatalf("first rotate: rotated=%v err=%v", rotated1, err)
	}

	if err := os.WriteFile(path, bytes.Repeat([]byte("y"), 2048), 0o644); err != nil {
		t.Fatal(err)
	}
	rotated2, archive2, err := MaybeRotate(path, 1024)
	if err != nil || !rotated2 {
		t.Fatalf("second rotate: rotated=%v err=%v", rotated2, err)
	}
	if archive1 == archive2 {
		t.Errorf("archive paths collide: %q == %q", archive1, archive2)
	}
}

func TestReadFromResetsOffsetAfterRotate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	archivePath := filepath.Join(dir, "events-archive.jsonl")
	var stderr bytes.Buffer

	rec, err := NewFileRecorder(path, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	defer rec.Close() //nolint:errcheck // test cleanup

	rec.Record(Event{Type: BeadCreated, Actor: "human"})
	rec.Record(Event{Type: BeadCreated, Actor: "human"})

	_, offset, err := ReadFrom(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if offset == 0 {
		t.Fatal("offset advanced past 0 expected after reading two events")
	}

	if err := rec.Rotate(archivePath); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	rec.Record(Event{Type: BeadClosed, Actor: "human"})

	got, _, err := ReadFrom(path, offset)
	if err != nil {
		t.Fatalf("ReadFrom after rotate: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("post-rotate ReadFrom returned %d events, want 1", len(got))
	}
	if got[0].Type != BeadClosed {
		t.Errorf("post-rotate event Type = %q, want %q", got[0].Type, BeadClosed)
	}
}

func seqOrZero(events []Event) uint64 {
	if len(events) == 0 {
		return 0
	}
	return events[0].Seq
}
