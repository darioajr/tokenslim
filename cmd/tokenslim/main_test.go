package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI(t *testing.T) {
	t.Setenv("TOKENSLIM_HOME", t.TempDir())
	for _, args := range [][]string{{"version"}, {"status"}, {"config", "show"}, {"config", "path"}, {"stats"}, {"stats", "--json"}, {"cache", "prune"}, {"help"}} {
		var out bytes.Buffer
		if e := run(args, strings.NewReader(""), &out); e != nil || out.Len() == 0 {
			t.Fatal(args, e)
		}
	}
}
func TestBenchmarkCLI(t *testing.T) {
	home := filepath.Join(t.TempDir(), "absent")
	t.Setenv("TOKENSLIM_HOME", home)
	var out bytes.Buffer
	e := run([]string{"benchmark", "--json", "-"}, strings.NewReader("small\n"), &out)
	if e != nil || !json.Valid(out.Bytes()) {
		t.Fatal(e, out.String())
	}
	if _, e = os.Stat(home); !os.IsNotExist(e) {
		t.Fatal("dry run wrote files")
	}
}
func TestCLIErrors(t *testing.T) {
	t.Setenv("TOKENSLIM_HOME", t.TempDir())
	for _, args := range [][]string{{"no-such-command"}, {"optimize"}, {"optimize", "--mode", "dangerous", "-"}, {"cache", "inspect", "../../etc/passwd"}} {
		if e := run(args, strings.NewReader(""), &bytes.Buffer{}); e == nil {
			t.Fatal(args)
		}
	}
	var out bytes.Buffer
	if e := run([]string{"hook", "post-tool-use"}, strings.NewReader("{bad"), &out); e != nil || out.Len() != 0 {
		t.Fatal("hook did not fail open")
	}
}
