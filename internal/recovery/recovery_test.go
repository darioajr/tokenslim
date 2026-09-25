package recovery

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"tokenslim/internal/cache"
)

func fixture(t *testing.T, raw, format string) (Service, string) {
	t.Helper()
	store := cache.Store{Dir: filepath.Join(t.TempDir(), "cache")}
	ref, err := store.Put([]byte(raw), cache.Record{Format: format})
	if err != nil {
		t.Fatal(err)
	}
	return Service{store}, ref
}
func invoke(t *testing.T, s Service, name string, a Args) map[string]any {
	t.Helper()
	value, err := s.Call(name, a)
	if err != nil {
		t.Fatal(err)
	}
	return value.(map[string]any)
}
func TestOriginalBytePagination(t *testing.T) {
	original := "aç🙂\r\nlast"
	s, ref := fixture(t, original, "text")
	var recovered strings.Builder
	offset := 0
	for {
		page := invoke(t, s, "tokenslim_get_original", Args{Ref: ref, Offset: offset, MaxBytes: 4})
		recovered.WriteString(page["text"].(string))
		offset = page["next_offset"].(int)
		if !page["has_more"].(bool) {
			break
		}
	}
	if recovered.String() != original {
		t.Fatal(recovered.String())
	}
	for _, a := range []Args{{Ref: ref, Offset: 2}, {Ref: ref, Offset: 3, MaxBytes: 1}, {Ref: ref, Offset: -1}, {Ref: ref, MaxBytes: MaxPageBytes + 1}} {
		if _, err := s.Call("tokenslim_get_original", a); err == nil {
			t.Fatalf("accepted %+v", a)
		}
	}
}
func TestStreamsAndLines(t *testing.T) {
	for _, tc := range []struct{ raw, format, stream string }{
		{"first\r\nsecond\nlast", "text", "raw"},
		{`{"stdout":"first\r\nsecond\nlast","stderr":"WARN diagnostic\n","exitCode":1}`, "claude-bash-json", "stdout"},
		{`{"output":"first\r\nsecond\nlast","exit_code":1}`, "codex-tool-json", "output"},
		{`"first\r\nsecond\nlast"`, "codex-tool-json", "output"},
	} {
		s, ref := fixture(t, tc.raw, tc.format)
		original := invoke(t, s, "tokenslim_get_original", Args{Ref: ref})
		if original["text"] != tc.raw || original["stream"] != "raw" {
			t.Fatal(original)
		}
		got := invoke(t, s, "tokenslim_get_range", Args{Ref: ref, StartLine: 1, EndLine: 2})
		if got["text"] != "first\r\nsecond\n" || got["stream"] != tc.stream {
			t.Fatal(got)
		}
		last := invoke(t, s, "tokenslim_get_range", Args{Ref: ref, StartLine: 3, EndLine: 20})
		if last["text"] != "last" || last["end_line"] != 3 {
			t.Fatal(last)
		}
		if tc.format == "claude-bash-json" {
			stderr := invoke(t, s, "tokenslim_get_range", Args{Ref: ref, Stream: "stderr", StartLine: 1, EndLine: 1})
			if stderr["text"] != "WARN diagnostic\n" {
				t.Fatal(stderr)
			}
		}
		if _, err := s.Call("tokenslim_get_range", Args{Ref: ref, StartLine: 4, EndLine: 5}); err == nil {
			t.Fatal("out-of-range accepted")
		}
	}
}
func TestLiteralSearchPagination(t *testing.T) {
	s, ref := fixture(t, "x.* one\nother\nx.* two\nx.* three\n", "text")
	first := invoke(t, s, "tokenslim_search_original", Args{Ref: ref, Query: "x.*", Limit: 1})
	if !first["has_more"].(bool) || first["next_line"] != 3 {
		t.Fatal(first)
	}
	second := invoke(t, s, "tokenslim_search_original", Args{Ref: ref, Query: "x.*", StartLine: 3, Limit: 2})
	matches := second["matches"].([]map[string]any)
	if len(matches) != 2 || matches[0]["line"] != 3 || second["has_more"] != false {
		t.Fatal(second)
	}
	none := invoke(t, s, "tokenslim_search_original", Args{Ref: ref, Query: "X.*"})
	if len(none["matches"].([]map[string]any)) != 0 {
		t.Fatal(none)
	}
}
func TestLimitsAndIntegrity(t *testing.T) {
	s, ref := fixture(t, strings.Repeat("x", MaxPageBytes+1), "text")
	for name, a := range map[string]Args{
		"tokenslim_get_range":       {Ref: ref, StartLine: 1, EndLine: 1},
		"tokenslim_search_original": {Ref: ref, Query: "x"},
	} {
		if _, err := s.Call(name, a); err == nil {
			t.Fatal("unbounded response", name)
		}
	}
	for _, a := range []Args{{Ref: "../../etc/passwd"}, {Ref: ref, Stream: "stderr"}} {
		if _, err := s.Call("tokenslim_get_original", a); err == nil {
			t.Fatal("invalid recovery accepted")
		}
	}
	meta := invoke(t, s, "tokenslim_describe", Args{Ref: ref})
	if meta["record"].(cache.Record).OriginalBytes != MaxPageBytes+1 {
		t.Fatal(meta)
	}
	if err := os.WriteFile(filepath.Join(s.Store.Dir, ref+".zst"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Call("tokenslim_describe", Args{Ref: ref}); err == nil {
		t.Fatal("corruption accepted")
	}
}
func TestReadOnly(t *testing.T) {
	s, ref := fixture(t, "hello\n", "text")
	before, _ := os.ReadFile(filepath.Join(s.Store.Dir, ref+".json"))
	for _, name := range []string{"tokenslim_describe", "tokenslim_get_original", "tokenslim_get_range", "tokenslim_search_original"} {
		invoke(t, s, name, Args{Ref: ref, StartLine: 1, EndLine: 1, Query: "hello"})
	}
	after, _ := os.ReadFile(filepath.Join(s.Store.Dir, ref+".json"))
	if string(before) != string(after) {
		t.Fatal("metadata changed")
	}
	entries, _ := os.ReadDir(s.Store.Dir)
	if len(entries) != 2 {
		t.Fatal("recovery wrote files")
	}
	var record cache.Record
	if json.Unmarshal(after, &record) != nil {
		t.Fatal("invalid metadata")
	}
}
