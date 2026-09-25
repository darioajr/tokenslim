package rules

import "testing"

func validSpec() Spec {
	var s Spec
	s.Name = "health"
	s.Match.Command = "worker logs*"
	s.Match.Regex = `(?P<tick>[0-9]+) INFO api=(?P<api>\w+) GET /health duration=(?P<duration>[0-9]+)ms`
	s.IgnoreGroups = []string{"tick", "duration"}
	s.Action.Aggregate = true
	return s
}
func TestGroupingIdentity(t *testing.T) {
	compiled, err := Compile([]Spec{validSpec()})
	if err != nil {
		t.Fatal(err)
	}
	r := compiled[0]
	a, ok := r.Key("1 INFO api=one GET /health duration=3ms")
	if !ok {
		t.Fatal("no match")
	}
	b, _ := r.Key("2 INFO api=one GET /health duration=5ms")
	c, _ := r.Key("2 INFO api=two GET /health duration=5ms")
	if a != b || a == c {
		t.Fatal("identity lost")
	}
	if !r.MatchesCommand("worker logs /tmp/app.log") || r.MatchesCommand("echo worker logs") {
		t.Fatal("incorrect command scope")
	}
	if _, ok := r.Key("prefix 1 INFO api=one GET /health duration=3ms"); ok {
		t.Fatal("partial match")
	}
	s := validSpec()
	s.IgnoreGroups = nil
	compiled, _ = Compile([]Spec{s})
	a, _ = compiled[0].Key("1 INFO api=one GET /health duration=3ms")
	b, _ = compiled[0].Key("2 INFO api=one GET /health duration=3ms")
	if a == b {
		t.Fatal("implicit ignored fields")
	}
}
func TestInvalidRules(t *testing.T) {
	for _, mutate := range []func(*Spec){
		func(s *Spec) { s.Name = "bad\nname" }, func(s *Spec) { s.Match.Regex = "[" }, func(s *Spec) { s.Match.Command = "" }, func(s *Spec) { s.Action.Aggregate = false }, func(s *Spec) { s.IgnoreGroups = []string{"missing"} }, func(s *Spec) { s.IgnoreGroups = []string{"tick", "tick"} }, func(s *Spec) { s.Match.Regex = `(?P<tick>a)(?P<tick>b)` },
	} {
		s := validSpec()
		mutate(&s)
		if _, err := Compile([]Spec{s}); err == nil {
			t.Fatalf("accepted %+v", s)
		}
	}
	if _, err := Compile([]Spec{validSpec(), validSpec()}); err == nil {
		t.Fatal("duplicate name")
	}
	if _, err := Compile(make([]Spec, 33)); err == nil {
		t.Fatal("unbounded rules")
	}
	s := validSpec()
	disabled := false
	s.Enabled = &disabled
	compiled, err := Compile([]Spec{s})
	if err != nil || len(compiled) != 0 {
		t.Fatal(compiled, err)
	}
	s = validSpec()
	s.Match.Regex = `(?P<tick>a(?P<duration>b))`
	compiled, _ = Compile([]Spec{s})
	if _, ok := compiled[0].Key("ab"); ok {
		t.Fatal("overlapping captures")
	}
}
