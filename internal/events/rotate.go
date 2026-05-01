package events

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// archiveFilenameLayout is the timestamp suffix used by MaybeRotate when
// generating a default archive filename. It encodes UTC time so archives
// sort lexicographically in chronological order.
const archiveFilenameLayout = "20060102T150405Z"

// Rotate closes the current event log file, renames it to archivePath, and
// opens a fresh empty file at the original path. The recorder's sequence
// counter resets to 0 — events written after Rotate begin at seq 1.
//
// Concurrent Record calls block while rotation runs. Other processes that
// hold an open append-mode FD on the original path will continue writing
// into the renamed archive until they re-open (typically next CLI invocation).
//
// Rotate refuses to overwrite an existing archivePath. On rename failure
// the recorder reopens the original file and remains usable; on reopen
// failure the recorder is closed and further Record calls become no-ops.
func (r *FileRecorder) Rotate(archivePath string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return errors.New("events: recorder is closed")
	}

	if _, err := os.Stat(archivePath); err == nil {
		return fmt.Errorf("events: archive path already exists: %s", archivePath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("events: stat archive path: %w", err)
	}

	if err := r.file.Close(); err != nil {
		return fmt.Errorf("events: closing event log: %w", err)
	}

	if err := os.Rename(r.path, archivePath); err != nil {
		if f, openErr := os.OpenFile(r.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); openErr == nil {
			r.file = f
		} else {
			r.closed = true
		}
		return fmt.Errorf("events: renaming event log: %w", err)
	}

	file, err := os.OpenFile(r.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		r.closed = true
		return fmt.Errorf("events: opening fresh event log after rotate: %w", err)
	}

	r.file = file
	r.seq = 0
	return nil
}

// MaybeRotate renames path to a timestamped archive sibling when its
// current size is at least minSize bytes. The archive name has the form
// "<base>-<UTC timestamp>.jsonl" (collisions are resolved with a numeric
// suffix). Returns (rotated, archivePath, err). When rotated is false,
// archivePath is empty.
//
// MaybeRotate is intended for offline rotation — call it before any
// FileRecorder is opened against path. While a long-lived FileRecorder
// holds an FD on path, callers should use FileRecorder.Rotate instead.
func MaybeRotate(path string, minSize int64) (bool, string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, "", nil
		}
		return false, "", fmt.Errorf("events: stat %s: %w", path, err)
	}
	if fi.Size() < minSize {
		return false, "", nil
	}

	archivePath, err := uniqueArchivePath(path, time.Now().UTC())
	if err != nil {
		return false, "", err
	}
	if err := os.Rename(path, archivePath); err != nil {
		return false, "", fmt.Errorf("events: renaming %s -> %s: %w", path, archivePath, err)
	}
	return true, archivePath, nil
}

// uniqueArchivePath returns a path of the form
// "<dir>/<base>-<UTC ts>.jsonl" that does not yet exist on disk. Callers
// pass the canonical events path (e.g. ".gc/events.jsonl"); the suffix
// becomes the archive filename. If a collision occurs, a numeric suffix
// "-1", "-2", ... is appended before the extension.
func uniqueArchivePath(path string, now time.Time) (string, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if ext == "" {
		ext = ".jsonl"
	}

	stamp := now.Format(archiveFilenameLayout)
	candidate := filepath.Join(dir, fmt.Sprintf("%s-%s%s", stem, stamp, ext))
	for i := 1; i < 1000; i++ {
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("events: stat candidate archive path: %w", err)
		}
		candidate = filepath.Join(dir, fmt.Sprintf("%s-%s-%d%s", stem, stamp, i, ext))
	}
	return "", fmt.Errorf("events: could not find unique archive name for %s", path)
}
