package cache

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRoundtrip(t *testing.T) {
	s := Store{Dir: filepath.Join(t.TempDir(), "cache"), MaxBytes: 1 << 20}
	original := []byte(strings.Repeat("秘密 ERROR\n", 200))
	ref, e := s.Put(original, Record{Format: "text"})
	if e != nil {
		t.Fatal(e)
	}
	b, r, e := s.Get(ref)
	if e != nil || string(b) != string(original) || r.ID != ref {
		t.Fatal(e, r)
	}
	for _, p := range []string{s.Dir, filepath.Join(s.Dir, ref+".zst"), filepath.Join(s.Dir, ref+".json")} {
		info, _ := os.Stat(p)
		// Windows uses inherited ACLs rather than POSIX permission bits.
		if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
			t.Fatal("public permissions", p)
		}
	}
	if _, _, e = s.Get("../../private"); e == nil {
		t.Fatal("path traversal")
	}
	os.WriteFile(filepath.Join(s.Dir, ref+".zst"), []byte("corrupt"), 0600)
	if _, _, e = s.Get(ref); e == nil {
		t.Fatal("corrupt accepted")
	}
}
func TestConcurrent(t *testing.T) {
	s := Store{Dir: filepath.Join(t.TempDir(), "cache"), MaxBytes: 1 << 20}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ref, e := s.Put([]byte("same output"), Record{})
			if e != nil {
				t.Error(e)
				return
			}
			if data, record, err := s.Get(ref); err != nil || string(data) != "same output" || record.ID != ref {
				t.Errorf("concurrent recovery: %q, %s, %v", data, record.ID, err)
			}
		}()
	}
	wg.Wait()
}
func TestPruneClear(t *testing.T) {
	s := Store{Dir: filepath.Join(t.TempDir(), "cache"), MaxBytes: 1 << 20}
	ref, e := s.Put([]byte("old"), Record{})
	if e != nil {
		t.Fatal(e)
	}
	old := time.Now().Add(-48 * time.Hour)
	os.Chtimes(filepath.Join(s.Dir, ref+".zst"), old, old)
	n, e := s.Prune(24*time.Hour, "")
	if e != nil || n != 1 {
		t.Fatal(n, e)
	}
	os.WriteFile(filepath.Join(s.Dir, "unrelated.txt"), []byte("keep"), 0600)
	s.Put([]byte("new"), Record{})
	if e = s.Clear(); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(s.Dir, "unrelated.txt")); e != nil {
		t.Fatal("removed unrelated")
	}
}
