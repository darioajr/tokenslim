// Package cache stores content-addressed originals with restrictive permissions.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

type Record struct {
	ID             string    `json:"id"`
	CreatedAt      time.Time `json:"created_at"`
	SessionID      string    `json:"session_id,omitempty"`
	ToolUseID      string    `json:"tool_use_id,omitempty"`
	CommandHash    string    `json:"command_hash"`
	OriginalBytes  int       `json:"original_bytes"`
	OptimizedBytes int       `json:"optimized_bytes"`
	Compressor     string    `json:"compressor"`
	Mode           string    `json:"mode"`
	Format         string    `json:"format"`
}
type Store struct {
	Dir      string
	MaxBytes int64
}

func Hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func Ref(b []byte) string  { return "ts_" + Hash(b) }

var validRef = regexp.MustCompile(`^ts_[0-9a-f]{64}$`)

func PrivateDir(path string) error {
	if e := os.MkdirAll(path, 0700); e != nil {
		return e
	}
	info, e := os.Lstat(path)
	if e != nil {
		return e
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("not a private directory: %s", path)
	}
	return os.Chmod(path, 0700)
}
func Atomic(path string, data []byte) error {
	f, e := os.CreateTemp(filepath.Dir(path), ".tmp-")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.Write(data); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(tmp, path)
}
func (s Store) Put(b []byte, r Record) (string, error) {
	if s.MaxBytes > 0 && int64(len(b)) > s.MaxBytes {
		return "", fmt.Errorf("original exceeds cache limit")
	}
	if e := PrivateDir(filepath.Dir(s.Dir)); e != nil {
		return "", e
	}
	if e := PrivateDir(s.Dir); e != nil {
		return "", e
	}
	r.ID = Ref(b)
	r.CreatedAt = time.Now().UTC()
	r.OriginalBytes = len(b)
	enc, e := zstd.NewWriter(nil, zstd.WithEncoderConcurrency(1))
	if e != nil {
		return "", e
	}
	compressed := enc.EncodeAll(b, nil)
	enc.Close()
	if s.MaxBytes > 0 && int64(len(compressed)) > s.MaxBytes {
		return "", fmt.Errorf("compressed entry exceeds cache budget")
	}
	if e = Atomic(filepath.Join(s.Dir, r.ID+".zst"), compressed); e != nil {
		return "", e
	}
	meta, e := json.Marshal(r)
	if e != nil {
		return "", e
	}
	if e = Atomic(filepath.Join(s.Dir, r.ID+".json"), meta); e != nil {
		return "", e
	}
	return r.ID, nil
}
func (s Store) Get(ref string) ([]byte, Record, error) {
	var r Record
	if !validRef.MatchString(ref) {
		return nil, r, fmt.Errorf("invalid cache reference")
	}
	p := filepath.Join(s.Dir, ref+".zst")
	info, e := os.Lstat(p)
	if e != nil {
		return nil, r, e
	}
	if !info.Mode().IsRegular() {
		return nil, r, fmt.Errorf("not a regular cache file")
	}
	f, e := os.Open(p)
	if e != nil {
		return nil, r, e
	}
	defer f.Close()
	max := s.MaxBytes
	if max <= 0 {
		max = 100 << 20
	}
	dec, e := zstd.NewReader(f, zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxMemory(uint64(max)+64<<20))
	if e != nil {
		return nil, r, e
	}
	defer dec.Close()
	b, e := io.ReadAll(io.LimitReader(dec, max+1))
	if e != nil {
		return nil, r, e
	}
	if int64(len(b)) > max {
		return nil, r, fmt.Errorf("cache entry exceeds input limit")
	}
	if Ref(b) != ref {
		return nil, r, fmt.Errorf("cache integrity mismatch")
	}
	meta, e := os.ReadFile(filepath.Join(s.Dir, ref+".json"))
	if e != nil {
		return nil, r, e
	}
	if e = json.Unmarshal(meta, &r); e != nil {
		return nil, r, e
	}
	if r.ID != ref || r.OriginalBytes != len(b) {
		return nil, r, fmt.Errorf("cache metadata mismatch")
	}
	return b, r, nil
}
func (s Store) Clear() error {
	entries, e := os.ReadDir(s.Dir)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	for _, entry := range entries {
		stem := strings.TrimSuffix(strings.TrimSuffix(entry.Name(), ".zst"), ".json")
		if validRef.MatchString(stem) && !entry.IsDir() {
			if e = os.Remove(filepath.Join(s.Dir, entry.Name())); e != nil {
				return e
			}
		}
	}
	return nil
}
func (s Store) Prune(retention time.Duration, keep string) (int, error) {
	entries, e := os.ReadDir(s.Dir)
	if os.IsNotExist(e) {
		return 0, nil
	}
	if e != nil {
		return 0, e
	}
	type item struct {
		ref     string
		size    int64
		created time.Time
	}
	var items []item
	var total int64
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".zst") {
			continue
		}
		ref := strings.TrimSuffix(entry.Name(), ".zst")
		if !validRef.MatchString(ref) {
			continue
		}
		info, e := entry.Info()
		if e != nil {
			return 0, e
		}
		size := info.Size()
		if m, e := os.Stat(filepath.Join(s.Dir, ref+".json")); e == nil {
			size += m.Size()
		}
		items = append(items, item{ref, size, info.ModTime()})
		total += size
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].created.Equal(items[j].created) {
			return items[i].ref < items[j].ref
		}
		return items[i].created.Before(items[j].created)
	})
	n := 0
	for _, x := range items {
		if x.ref == keep {
			continue
		}
		if time.Since(x.created) > retention || s.MaxBytes > 0 && total > s.MaxBytes {
			for _, ext := range []string{".zst", ".json"} {
				if e = os.Remove(filepath.Join(s.Dir, x.ref+ext)); e != nil && !os.IsNotExist(e) {
					return n, e
				}
			}
			total -= x.size
			n++
		}
	}
	return n, nil
}
