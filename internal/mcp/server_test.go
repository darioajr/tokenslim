package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"tokenslim/internal/cache"
	"tokenslim/internal/recovery"
)

const handshake = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}` + "\n" + `{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"

func exchange(t *testing.T, in string, svc recovery.Service) []map[string]any {
	t.Helper()
	var out bytes.Buffer
	if err := Serve(strings.NewReader(in), &out, svc, "0.2.0"); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&out)
	var results []map[string]any
	for {
		var item map[string]any
		err := decoder.Decode(&item)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		results = append(results, item)
	}
	return results
}
func TestLifecycleAndTools(t *testing.T) {
	store := cache.Store{Dir: filepath.Join(t.TempDir(), "cache")}
	ref, err := store.Put([]byte("first\nsecond\n"), cache.Record{Format: "text"})
	if err != nil {
		t.Fatal(err)
	}
	input := handshake + `{"jsonrpc":"2.0","id":"list","method":"tools/list"}` + "\n" + `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":99}}` + "\n" + `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"tokenslim_get_range","arguments":{"ref":"` + ref + `","start_line":2,"end_line":2}}}` + "\n"
	responses := exchange(t, input, recovery.Service{Store: store})
	if len(responses) != 3 {
		t.Fatal(responses)
	}
	init := responses[0]["result"].(map[string]any)
	if init["protocolVersion"] != ProtocolVersion {
		t.Fatal(init)
	}
	listed := responses[1]["result"].(map[string]any)["tools"].([]any)
	if len(listed) != 4 {
		t.Fatal(listed)
	}
	result := responses[2]["result"].(map[string]any)
	if result["isError"] != false || result["structuredContent"].(map[string]any)["text"] != "second\n" {
		t.Fatal(result)
	}
	content := result["content"].([]any)[0].(map[string]any)
	if content["type"] != "text" || !json.Valid([]byte(content["text"].(string))) {
		t.Fatal(content)
	}
}
func TestProtocolErrors(t *testing.T) {
	for _, tc := range []struct {
		message string
		code    float64
	}{
		{"{", -32700}, {"[]", -32600}, {`{"jsonrpc":"2.0","id":null,"method":"ping"}`, -32600},
		{`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, -32002},
		{`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{}}`, -32602},
	} {
		result := exchange(t, tc.message+"\n", recovery.Service{})
		if len(result) != 1 || result[0]["error"].(map[string]any)["code"] != tc.code {
			t.Fatal(result)
		}
	}
	for _, params := range []string{
		`{"name":"unknown","arguments":{}}`,
		`{"name":"tokenslim_get_range","arguments":{"ref":"x"}}`,
		`{"name":"tokenslim_get_original","arguments":{"ref":"x","offset":-1}}`,
		`{"name":"tokenslim_get_original","arguments":{"ref":"x","path":"/etc/passwd"}}`,
		`{"name":"tokenslim_get_original","arguments":{"ref":"x","max_bytes":0}}`,
	} {
		results := exchange(t, handshake+`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":`+params+"}\n", recovery.Service{})
		if results[1]["error"].(map[string]any)["code"] != float64(-32602) {
			t.Fatal(results)
		}
	}
}
func TestToolFailureAndVersionNegotiation(t *testing.T) {
	input := strings.Replace(handshake, ProtocolVersion, "unknown-client-version", 1) + `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"tokenslim_get_original","arguments":{"ref":"../../secret"}}}` + "\n"
	results := exchange(t, input, recovery.Service{})
	if results[0]["result"].(map[string]any)["protocolVersion"] != ProtocolVersion || results[1]["result"].(map[string]any)["isError"] != true {
		t.Fatal(results)
	}
}
func TestOversizedInput(t *testing.T) {
	var out bytes.Buffer
	if err := Serve(strings.NewReader(strings.Repeat("x", MaxRequestBytes+1)), &out, recovery.Service{}, "test"); err == nil {
		t.Fatal("unbounded request")
	}
	if out.Len() != 0 {
		t.Fatal("partial response")
	}
}

func FuzzProtocol(f *testing.F) {
	for _, s := range []string{`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{"cursor":{}}}`, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":null}`, `[]`, `{`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 64<<10 {
			t.Skip()
		}
		var out bytes.Buffer
		if err := Serve(strings.NewReader(handshake+s+"\n"), &out, recovery.Service{}, "test"); err != nil {
			t.Fatal(err)
		}
		for _, line := range bytes.Split(bytes.TrimSpace(out.Bytes()), []byte("\n")) {
			if !json.Valid(line) {
				t.Fatalf("non-protocol output: %q", line)
			}
		}
	})
}
