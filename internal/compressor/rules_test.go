package compressor

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"tokenslim/internal/rules"
)

func customRule(t *testing.T) []rules.Rule {
	t.Helper()
	var s rules.Spec
	s.Name = "health"
	s.Match.Command = "worker logs*"
	s.Match.Regex = `(?P<tick>[0-9]+) INFO api=\w+ GET /health duration=(?P<duration>[0-9]+)ms`
	s.IgnoreGroups = []string{"tick", "duration"}
	s.Action.Aggregate = true
	compiled, err := rules.Compile([]rules.Spec{s})
	if err != nil {
		t.Fatal(err)
	}
	return compiled
}
func healthLines(api string) string {
	var b strings.Builder
	for i := 0; i < 10; i++ {
		fmt.Fprintf(&b, "%d INFO api=%s GET /health duration=%dms\n", i, api, i)
	}
	return b.String()
}
func TestRuleBoundariesAndDiagnostics(t *testing.T) {
	prefix := healthLines("one") + healthLines("two")
	body := "ERROR request rejected\n" + healthLines("one")
	source := prefix + body
	result := (Reducer{"generic"}).Compress(Context{Mode: "smart", Command: "worker logs", Rules: customRule(t)}, source)
	expected := "0 INFO api=one GET /health duration=0ms\n[TokenSlim rule=health: 10 lines aggregated]\n0 INFO api=two GET /health duration=0ms\n[TokenSlim rule=health: 10 lines aggregated]\n" + body
	if result.Output != expected || !Intact(source, result.Output) {
		t.Fatal(result.Output)
	}
	if result.Rules["health"] != (rules.Effect{Groups: 2, Lines: 20}) {
		t.Fatal(result.Rules)
	}
	for _, ctx := range []Context{{Mode: "safe", Command: "worker logs", Rules: customRule(t)}, {Mode: "smart", Command: "other logs", Rules: customRule(t)}} {
		if got := (Reducer{"generic"}).Compress(ctx, source); got.Output != source || len(got.Rules) != 0 {
			t.Fatal(got)
		}
	}
}
func TestRulesFirstMatchAndJSON(t *testing.T) {
	first := customRule(t)
	second := customRule(t)
	second[0].Name = "second"
	result := (Reducer{"generic"}).Compress(Context{Mode: "smart", Command: "worker logs", Rules: append(first, second...)}, healthLines("one"))
	if len(result.Rules) != 1 || result.Rules["health"].Groups != 1 {
		t.Fatal(result.Rules)
	}
	s := `{"text":"1 INFO api=one GET /health duration=3ms"}`
	result = (Reducer{"generic"}).Compress(Context{Mode: "smart", Command: "worker logs", Rules: first}, s)
	if result.Output != s || len(result.Rules) != 0 {
		t.Fatal(result)
	}
}
func FuzzCustomRules(f *testing.F) {
	f.Add(healthLines("one"))
	f.Add("ERROR\n" + healthLines("one"))
	f.Add("\x1b[31mWARN\x1b[0m\n")
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 64<<10 {
			t.Skip()
		}
		ctx := Context{Mode: "smart", Command: "worker logs", Rules: customRule(t)}
		a := (Reducer{"generic"}).Compress(ctx, s)
		b := (Reducer{"generic"}).Compress(ctx, s)
		if !reflect.DeepEqual(a, b) || !Intact(s, a.Output) {
			t.Fatal("nondeterminism or lost diagnostics")
		}
	})
}
