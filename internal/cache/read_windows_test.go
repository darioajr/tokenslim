package cache

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestCacheReplacementCompletesAfterReaderCloses(t *testing.T) {
	for _, extension := range []string{".zst", ".json"} {
		t.Run(extension, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "entry"+extension)
			if err := Atomic(path, []byte("old")); err != nil {
				t.Fatal(err)
			}
			reader, err := openCacheFile(path)
			if err != nil {
				t.Fatal(err)
			}
			// Replacement can remain blocked until the reader closes, even with
			// FILE_SHARE_DELETE. Exercise the real bounded-retry contract.
			done := make(chan error, 1)
			go func() { done <- Atomic(path, []byte("new")) }()
			time.Sleep(100 * time.Millisecond)
			old, readErr := io.ReadAll(reader)
			closeErr := reader.Close()
			replaceErr := <-done
			if readErr != nil || string(old) != "old" {
				t.Fatalf("reader snapshot: %q, %v", old, readErr)
			}
			if closeErr != nil || replaceErr != nil {
				t.Fatalf("close: %v; replacement: %v", closeErr, replaceErr)
			}
			current, err := readCacheFile(path)
			if err != nil || string(current) != "new" {
				t.Fatalf("replacement: %q, %v", current, err)
			}
		})
	}
}

func TestCacheReadWindowsSharingViolation(t *testing.T) {
	for _, permanent := range []bool{false, true} {
		name := "temporary"
		if permanent {
			name = "persistent"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "entry.json")
			if err := Atomic(path, []byte("original")); err != nil {
				t.Fatal(err)
			}
			wide, err := syscall.UTF16PtrFromString(path)
			if err != nil {
				t.Fatal(err)
			}
			// An exclusive handle deterministically reproduces the CI read error.
			handle, err := syscall.CreateFile(wide, syscall.GENERIC_READ, 0, nil,
				syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
			if err != nil {
				t.Fatal(err)
			}
			closed := make(chan error, 1)
			if !permanent {
				go func() {
					time.Sleep(100 * time.Millisecond)
					closed <- syscall.CloseHandle(handle)
				}()
			}
			data, readErr := readCacheFile(path)
			if permanent {
				closed <- syscall.CloseHandle(handle)
			}
			if err := <-closed; err != nil {
				t.Fatal(err)
			}
			if permanent {
				if !errors.Is(readErr, syscall.Errno(32)) {
					t.Fatalf("expected sharing violation, got %v", readErr)
				}
			} else if readErr != nil || string(data) != "original" {
				t.Fatalf("retry failed: %q, %v", data, readErr)
			}
			data, err = readCacheFile(path)
			if err != nil || string(data) != "original" {
				t.Fatalf("original changed: %q, %v", data, err)
			}
		})
	}
}

func TestCacheReadMissingFile(t *testing.T) {
	_, err := openCacheFile(filepath.Join(t.TempDir(), "missing"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing file, got %v", err)
	}
}
