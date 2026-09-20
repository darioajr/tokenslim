# Architecture and compatibility

`hook → config → classifier → compressor → integrity guard → cache → metrics → adapter`

The Go core accepts text streams and contains no agent SDK, command executor,
network client, source editor or model call. Hooks preserve unrecognized outputs
by returning exit status zero with empty stdout. Successful transformations are
buffered before protocol serialization. Read/Edit/Write are excluded in code,
regardless of configuration. Bash arguments are never rewritten or executed.

## Host adapters

Claude Code receives `hookSpecificOutput.updatedToolOutput`, preserving the Bash
response object and extra fields. Required fields are stdout/stderr strings and
interrupted/isImage booleans. Image and interrupted results pass through.

Codex receives `continue: false` and `stopReason` containing the optimized tool
result. This is the documented PostToolUse replacement mechanism; it does not
cancel the completed command. Using `decision: block` would reject nested code
mode promises, so TokenSlim does not use it. Native source tools are excluded.
Supported Codex responses are model-facing strings or completed exec objects with
`output`, `exit_code`, `wall_time_seconds`, `session_id`, `chunk_id`, and
`original_token_count`. Unknown fields/shapes and live sessions pass through.
Structured metadata is preserved as JSON in the replacement text. Codex may wrap
or truncate hook feedback according to its own output limits. In code mode,
`continue:false` does not reject the nested promise; TokenSlim does not claim to
rewrite every intermediate JavaScript value or cover tools that bypass hooks.

The local contract tests do not prove acceptance by an authenticated model
session. Host versions and hook settings matter. Codex requires hook trust via
`/hooks`; enabling a plugin alone is insufficient. No trust settings are changed
by building or running the tests.

## Storage

Originals are zstd-compressed and content-addressed by SHA-256. The deterministic
reference is the hash of the original text (CLI) or exact tool-response JSON
(hooks). Metadata includes command hash, not the raw command. Cache directories
are 0700 and atomically replaced files 0600 on Unix. Windows uses inherited
filesystem ACLs. A failed cache write rejects the
transformation. Recovery verifies the hash and size. Concurrent identical outputs
share an original; metadata records the most recent writer. Retention and oldest
first size pruning run on writes. Active references can expire with cache pruning.

Metrics use one atomic JSON file per invocation, avoiding SQLite and concurrent
append corruption. They contain sizes, estimated tokens, timing and session ID,
not output or commands. Metrics failure does not discard an otherwise safe result.
Metrics retention is manual in 0.1; disabling metrics avoids future records.

Inputs are bounded (100 MiB configurable, 128 MiB absolute hook JSON cap). CLI
optimize streams input through unchanged above its limit. Benchmark rejects
oversized input. The compression path holds bounded strings in memory, rather
than pretending to offer streaming structured reduction.

`target.max_output_bytes` is a soft, reserved preference: the MVP never truncates
unique diagnostics to hit it. Minimum savings and maximum reduction are enforced,
including marker bytes. Very repetitive output can therefore be rejected by the
95% reduction guard. Off mode, unknown formats, malformed YAML, failed guards and
cache failures preserve the original result.

## Official contracts checked during implementation

- [Claude Code hooks](https://code.claude.com/docs/en/hooks#posttooluse-decision-control)
- [Codex hooks and code-mode semantics](https://learn.chatgpt.com/docs/hooks)
- [Codex skills](https://learn.chatgpt.com/docs/build-skills)
