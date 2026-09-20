package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func output() string {
	var b strings.Builder
	for i := 0; i < 60; i++ {
		b.WriteString(strings.Repeat(fmt.Sprintf("INFO retry group=%d\n", i), 10))
		fmt.Fprintf(&b, "checkpoint %d needs to remain visible\n", i)
	}
	b.WriteString("ERROR refund failed app.go:42\nExpected: 1; Actual: 2\n")
	return b.String()
}
func payload(response any) map[string]any {
	return map[string]any{"hook_event_name": "PostToolUse", "tool_name": "Bash", "tool_input": map[string]any{"command": "npm test"}, "tool_response": response}
}
func invoke(t *testing.T, p any, agent string) string {
	t.Helper()
	b, _ := json.Marshal(p)
	var out bytes.Buffer
	RunAgent(bytes.NewReader(b), &out, t.TempDir(), t.TempDir(), agent)
	return out.String()
}
func TestClaude(t *testing.T) {
	p := payload(map[string]any{"stdout": output(), "stderr": "WARN keep stderr\n", "interrupted": false, "isImage": false, "exitCode": 1, "future": map[string]any{"keep": true}})
	out := invoke(t, p, "claude")
	var r struct {
		Hook struct {
			Updated map[string]json.RawMessage `json:"updatedToolOutput"`
		} `json:"hookSpecificOutput"`
	}
	if json.Unmarshal([]byte(out), &r) != nil {
		t.Fatal(out)
	}
	if string(r.Hook.Updated["exitCode"]) != "1" || string(r.Hook.Updated["future"]) != "{\"keep\":true}" || !strings.Contains(string(r.Hook.Updated["stdout"]), "ERROR refund failed") {
		t.Fatal(out)
	}
}
func TestCodex(t *testing.T) {
	for _, response := range []any{output(), map[string]any{"output": output(), "exit_code": 1, "wall_time_seconds": 0.1}} {
		out := invoke(t, payload(response), "codex")
		var r struct {
			Continue *bool  `json:"continue"`
			Reason   string `json:"stopReason"`
		}
		if json.Unmarshal([]byte(out), &r) != nil || r.Continue == nil || *r.Continue || !strings.Contains(r.Reason, "ERROR refund failed") {
			t.Fatal(out)
		}
		if strings.Contains(out, "additionalContext") {
			t.Fatal("promoted logs")
		}
	}
}
func TestPassThrough(t *testing.T) {
	for _, agent := range []string{"claude", "codex"} {
		for _, response := range []any{nil, map[string]any{"stdout": output()}, map[string]any{"stdout": output(), "stderr": "", "interrupted": true, "isImage": false}, map[string]any{"stdout": output(), "stderr": "", "interrupted": false, "isImage": true}, map[string]any{"output": output(), "unknown": true}, map[string]any{"output": output(), "session_id": 123}} {
			if out := invoke(t, payload(response), agent); out != "" {
				t.Fatalf("%s accepted unsupported response: %s", agent, out)
			}
		}
		p := payload(output())
		p["tool_name"] = "Read"
		if invoke(t, p, agent) != "" {
			t.Fatal("modified Read")
		}
	}
}
func TestInvalid(t *testing.T) {
	for _, s := range []string{"{bad", "{}", "null", "[]", "{}{}"} {
		var out bytes.Buffer
		Run(strings.NewReader(s), &out, t.TempDir(), t.TempDir())
		if out.Len() != 0 {
			t.Fatal(out.String())
		}
	}
}
func FuzzHook(f *testing.F) {
	f.Add([]byte(`{"tool_name":"Read"}`))
	f.Add([]byte(`{bad`))
	home := f.TempDir()
	cwd := f.TempDir()
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		var out bytes.Buffer
		Run(bytes.NewReader(b), &out, home, cwd)
		if out.Len() > 0 && !json.Valid(out.Bytes()) {
			t.Fatal("invalid protocol")
		}
	})
}
