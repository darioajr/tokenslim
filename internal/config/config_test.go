package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrecedence(t *testing.T) {
	h := t.TempDir()
	p := t.TempDir()
	os.WriteFile(filepath.Join(h, "config.yaml"), []byte("mode: smart\nthresholds:\n  minimum_bytes: 123\n"), 0600)
	os.WriteFile(filepath.Join(p, ".tokenslim.yaml"), []byte("mode: safe\ncompressors:\n  maven:\n    enabled: false\n"), 0600)
	c, e := Load(h, p)
	if e != nil {
		t.Fatal(e)
	}
	if c.Mode != "safe" || c.Thresholds.MinimumBytes != 123 || c.Compressors["maven"].Enabled || !c.Compressors["docker"].Enabled {
		t.Fatalf("bad merge: %+v", c)
	}
}
func TestInvalid(t *testing.T) {
	for _, s := range []string{"mode: risky", "thresholds:\n  minimum_bytes: -1", "typo: 1", "cache:\n  retention: -7d", "version: 2", "mode: safe\n---\nmode: smart"} {
		h := t.TempDir()
		os.WriteFile(filepath.Join(h, "config.yaml"), []byte(s), 0600)
		if _, e := Load(h, t.TempDir()); e == nil {
			t.Fatalf("accepted %s", s)
		}
	}
}

func TestNonFinite(t *testing.T) {
	h := t.TempDir()
	os.WriteFile(filepath.Join(h, "config.yaml"), []byte("token_estimate:\n  characters_per_token: .nan\n"), 0600)
	if _, e := Load(h, t.TempDir()); e == nil {
		t.Fatal("accepted NaN")
	}
}

func TestRuleConfiguration(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	valid := "mode: smart\nrules:\n  - name: health\n    match:\n      command: 'worker logs*'\n      regex: '(?P<tick>[0-9]+) INFO ready'\n    ignore_groups: [tick]\n    action:\n      aggregate: true\n"
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte(valid), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(home, project)
	if err != nil || len(c.Rules) != 1 {
		t.Fatal(c, err)
	}
	if err := os.WriteFile(filepath.Join(project, ".tokenslim.yaml"), []byte("rules: []\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err = Load(home, project)
	if err != nil || len(c.Rules) != 0 {
		t.Fatal("project did not replace rule list", c, err)
	}
	for _, bad := range []string{"rules:\n  - name: incomplete\n", "rules:\n  - name: bad\n    action:\n      execute: rm\n"} {
		if err := os.WriteFile(filepath.Join(project, ".tokenslim.yaml"), []byte(bad), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(home, project); err == nil {
			t.Fatal("accepted malformed rules")
		}
	}
}
