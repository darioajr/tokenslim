// Package hook adapts Claude Code PostToolUse/Bash. No output means no replacement.
package hook

import (
	"bytes"
	"encoding/json"
	"io"
	"tokenslim/internal/config"
	"tokenslim/internal/engine"
)

type Input struct {
	SessionID     string          `json:"session_id"`
	CWD           string          `json:"cwd"`
	HookEventName string          `json:"hook_event_name"`
	ToolName      string          `json:"tool_name"`
	ToolUseID     string          `json:"tool_use_id"`
	ToolInput     json.RawMessage `json:"tool_input"`
	ToolResponse  json.RawMessage `json:"tool_response"`
}

// Run buffers its response so a parse error or panic never writes partial protocol JSON.
func Run(in io.Reader, out io.Writer, home, cwd string) { RunAgent(in, out, home, cwd, "claude") }
func RunAgent(in io.Reader, out io.Writer, home, cwd, agent string) {
	defer func() { _ = recover() }()
	// The hard cap bounds JSON decoding before the project configuration is known.
	b, err := io.ReadAll(io.LimitReader(in, (128<<20)+1))
	if err != nil || len(b) > 128<<20 {
		return
	}
	var p Input
	if json.Unmarshal(b, &p) != nil || p.HookEventName != "PostToolUse" || p.ToolName != "Bash" {
		return
	}
	if p.CWD != "" {
		cwd = p.CWD
	}
	c, err := config.Load(home, cwd)
	if err != nil || !c.Tools["Bash"].Enabled || len(b) > c.Limits.MaxInputMB<<20 {
		return
	}
	var input struct {
		Command string `json:"command"`
	}
	if json.Unmarshal(p.ToolInput, &input) != nil || input.Command == "" {
		return
	}
	if agent == "codex" {
		runCodex(p, input.Command, c, home, out)
		return
	}
	if agent != "claude" {
		return
	}
	var response map[string]json.RawMessage
	if json.Unmarshal(p.ToolResponse, &response) != nil || response == nil {
		return
	}
	var stdout, stderr string
	var interrupted, image bool
	// Required fields must have their documented types; reject null as well.
	for _, key := range []string{"stdout", "stderr", "interrupted", "isImage"} {
		v, ok := response[key]
		if !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return
		}
	}
	if json.Unmarshal(response["stdout"], &stdout) != nil || json.Unmarshal(response["stderr"], &stderr) != nil || json.Unmarshal(response["interrupted"], &interrupted) != nil || json.Unmarshal(response["isImage"], &image) != nil || interrupted || image {
		return
	}
	r := (engine.Engine{Config: c, Home: home}).Process(engine.Request{Stdout: stdout, Stderr: stderr, Command: input.Command, SessionID: p.SessionID, ToolUseID: p.ToolUseID, Original: p.ToolResponse, Format: "claude-bash-json"}, false)
	if !r.Metrics.Changed {
		return
	}
	response["stdout"], _ = json.Marshal(r.Stdout)
	response["stderr"], _ = json.Marshal(r.Stderr)
	result := map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": "PostToolUse", "updatedToolOutput": response}}
	encoded, err := json.Marshal(result)
	if err != nil {
		return
	}
	encoded = append(encoded, '\n')
	_, _ = out.Write(encoded)
}

// Codex uses continue:false/stopReason for replacement, rather than Claude's
// updatedToolOutput. Do not promote untrusted logs into additionalContext.
func runCodex(p Input, command string, c config.Config, home string, out io.Writer) {
	var text string
	var object map[string]json.RawMessage
	isString := json.Unmarshal(p.ToolResponse, &text) == nil
	if !isString {
		if json.Unmarshal(p.ToolResponse, &object) != nil || object == nil {
			return
		}
		if json.Unmarshal(object["output"], &text) != nil || bytes.Equal(bytes.TrimSpace(object["output"]), []byte("null")) {
			return
		}
		// Unknown object shapes fail open. Metadata is preserved verbatim as JSON.
		for key := range object {
			switch key {
			case "output", "exit_code", "wall_time_seconds", "session_id", "chunk_id", "original_token_count":
			default:
				return
			}
		}
		var exit int
		if raw, ok := object["exit_code"]; ok && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			if json.Unmarshal(raw, &exit) != nil {
				return
			}
		}
		// A live process result must remain native so polling continues normally.
		if raw, ok := object["session_id"]; ok && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return
		}
	}
	if bytes.Equal(bytes.TrimSpace(p.ToolResponse), []byte("null")) {
		return
	}
	r := (engine.Engine{Config: c, Home: home}).Process(engine.Request{Stdout: text, Command: command, SessionID: p.SessionID, ToolUseID: p.ToolUseID, Original: p.ToolResponse, Format: "codex-tool-json"}, false)
	if !r.Metrics.Changed {
		return
	}
	replacement := r.Stdout
	if !isString {
		object["output"], _ = json.Marshal(replacement)
		b, e := json.Marshal(object)
		if e != nil {
			return
		}
		replacement = string(b)
	}
	b, e := json.Marshal(map[string]any{"continue": false, "stopReason": replacement})
	if e != nil {
		return
	}
	_, _ = out.Write(append(b, '\n'))
}
