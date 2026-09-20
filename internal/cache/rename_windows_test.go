package cache

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Hold a Windows handle that explicitly denies delete/rename sharing, as an
// ordinary reader or another process can do while the cache is being replaced.
func lockDestination(t *testing.T, path string) syscall.Handle {
	t.Helper()
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	return handle
}

func TestAtomicWindowsReaderReleasesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "original")
	if err := Atomic(path, []byte("old")); err != nil {
		t.Fatal(err)
	}
	handle := lockDestination(t, path)
	closed := make(chan error, 1)
	go func() {
		time.Sleep(100 * time.Millisecond)
		closed <- syscall.CloseHandle(handle)
	}()
	err := Atomic(path, []byte("new"))
	if closeErr := <-closed; closeErr != nil {
		t.Fatal(closeErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "new" {
		t.Fatalf("replacement: %q, %v", data, err)
	}
}

func TestAtomicWindowsLockedFilePreservesOriginal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "original")
	if err := Atomic(path, []byte("old")); err != nil {
		t.Fatal(err)
	}
	handle := lockDestination(t, path)
	err := Atomic(path, []byte("new"))
	if closeErr := syscall.CloseHandle(handle); closeErr != nil {
		t.Fatal(closeErr)
	}
	if !errors.Is(err, syscall.ERROR_ACCESS_DENIED) && !errors.Is(err, syscall.Errno(32)) {
		t.Fatalf("expected lock error, got %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "old" {
		t.Fatalf("original lost: %q, %v", data, err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 || files[0].Name() != "original" {
		t.Fatalf("temporary file leaked: %v, %v", files, err)
	}
}
