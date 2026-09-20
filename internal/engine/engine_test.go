package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"tokenslim/internal/config"
)

func large() string {
	var b strings.Builder
	for i := 0; i < 80; i++ {
		b.WriteString(strings.Repeat(fmt.Sprintf("INFO heartbeat group=%d accepted\n", i), 8))
		fmt.Fprintf(&b, "checkpoint %d is meaningful and must stay\n", i)
	}
	b.WriteString("ERROR refund failed\napp.go:31\n")
	return b.String()
}
func TestPipeline(t *testing.T) {
	c := config.Default()
	e := Engine{c, t.TempDir()}
	s := large()
	r := e.Process(Request{Stdout: s}, false)
	if !r.Metrics.Changed || !strings.Contains(r.Stdout, "ERROR refund failed") {
		t.Fatalf("%+v", r.Metrics)
	}
	if r.Metrics.OptimizedBytes >= len(s) {
		t.Fatal("no savings")
	}
	r2 := e.Process(Request{Stdout: s}, false)
	if r.Stdout != r2.Stdout {
		t.Fatal("not deterministic")
	}
	if e.Process(Request{Stdout: "small\n"}, false).Stdout != "small\n" {
		t.Fatal("small changed")
	}
}
func TestFailOpen(t *testing.T) {
	for _, name := range []string{"off", "cache-disabled", "cache-failed", "budget", "binary"} {
		t.Run(name, func(t *testing.T) {
			c := config.Default()
			home := t.TempDir()
			s := large()
			switch name {
			case "off":
				c.Mode = "off"
			case "cache-disabled":
				c.Cache.Enabled = false
			case "cache-failed":
				os.WriteFile(filepath.Join(home, "cache"), []byte("not directory"), 0600)
			case "budget":
				c.Target.MaxReduction = 1
			case "binary":
				s += "\x00"
			}
			r := (Engine{c, home}).Process(Request{Stdout: s}, false)
			if r.Stdout != s || r.Metrics.Changed {
				t.Fatal("not fail open")
			}
		})
	}
}
func TestBenchmarkNoWrites(t *testing.T) {
	home := filepath.Join(t.TempDir(), "absent")
	r := (Engine{config.Default(), home}).Process(Request{Stdout: large()}, true)
	if !r.Metrics.Changed {
		t.Fatal(r.Metrics)
	}
	if _, e := os.Stat(home); !os.IsNotExist(e) {
		t.Fatal("dry run wrote files")
	}
}
func BenchmarkPipeline(b *testing.B) {
	c := config.Default()
	s := strings.Repeat(large(), 10)
	e := Engine{c, b.TempDir()}
	b.SetBytes(int64(len(s)))
	for b.Loop() {
		e.Process(Request{Stdout: s}, true)
	}
}

func TestRepeatToggle(t *testing.T) {
	c := config.Default()
	o := c.Compressors["generic"]
	o.GroupRepeatedLines = false
	c.Compressors["generic"] = o
	s := large()
	r := (Engine{c, t.TempDir()}).Process(Request{Stdout: s}, true)
	if r.Stdout != s {
		t.Fatal("ignored group_repeated_lines")
	}
}
func TestDebug(t *testing.T) {
	c := config.Default()
	c.Debug.Enabled = true
	c.Metrics.Enabled = false
	home := t.TempDir()
	(Engine{c, home}).Process(Request{Stdout: "small private secret\n"}, false)
	b, e := os.ReadFile(filepath.Join(home, "logs", "tokenslim.log"))
	if e != nil || !strings.Contains(string(b), "below thresholds") || strings.Contains(string(b), "private secret") {
		t.Fatal(string(b), e)
	}
}
