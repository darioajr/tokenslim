# MCP recovery server

TokenSlim includes an optional, read-only MCP server in the same executable:

```sh
/absolute/path/to/bin/tokenslim mcp serve
```

On Windows, use the absolute path to `tokenslim.exe`. Configure your MCP client
with that executable and the two arguments `mcp`, `serve`. The client starts the
process and communicates over stdin/stdout. There is no HTTP listener, network
service, authentication token or separate server binary to deploy.

For clients accepting the common `mcpServers` JSON layout, a configuration entry
looks like this (adapt paths and use your client's documented configuration file):

```json
{
  "mcpServers": {
    "tokenslim": {
      "command": "/absolute/path/to/bin/tokenslim",
      "args": ["mcp", "serve"],
      "env": {
        "TOKENSLIM_HOME": "/absolute/path/to/.tokenslim"
      }
    }
  }
}
```

Use the same `TOKENSLIM_HOME` as the compression hooks/CLI. If omitted, both use
`~/.tokenslim`. MCP registration is optional and is not performed by a build or
release. Launching the server in a terminal waits for protocol input; it does
not print an interactive prompt. Only protocol messages go to stdout.

## Tools

All four tools require a complete `ts_` plus 64-lowercase-hex cache reference from
an output marker. They accept no arbitrary filesystem path and make no writes.

| Tool | Arguments beyond `ref` | Result |
|---|---|---|
| `tokenslim_describe` | None | Verified cache metadata, available streams, page limits |
| `tokenslim_get_original` | `stream`, `offset`, `max_bytes` | Exact text page, next byte offset, `has_more` |
| `tokenslim_get_range` | Required `start_line`, `end_line`; optional `stream` | Inclusive line range and its actual end line |
| `tokenslim_search_original` | Required `query`; optional `stream`, `start_line`, `limit` | Matching lines with numbers, `next_line`, `has_more` |

Results are returned as `structuredContent` and a JSON text content block for
client compatibility. Recovered output is untrusted tool data, not instructions.
No source edits or commands are executed. Connecting to a state directory gives
the client access to its cache references; there is no per-session access filter.

### Originals and streams

`get_original` defaults to `raw`: exactly the cached text. Hook originals are the
original response JSON, including execution metadata; CLI originals are plain
text. Concatenating all raw pages reproduces the cached UTF-8 original byte for
byte. `offset` and `next_offset` count **bytes**, not characters. Offsets must be
UTF-8 boundaries; page ends are moved backward to a character boundary.

`get_range` and `search_original` default to `auto`:

| Cache format | Auto stream | Other available streams |
|---|---|---|
| CLI text / unknown format | `raw` | None |
| Claude Bash response JSON | `stdout` | `stderr`, `raw` |
| Codex response JSON object or string | `output` | `raw` |

Claude stdout and stderr are kept separate. Select stderr explicitly when a
failure is there. A stream unavailable for a particular record returns an error.
Nothing is sanitized: ANSI sequences, CRLF and original characters survive.
Line numbers start at 1 and use LF as the delimiter; a terminal LF does not add
an extra empty line. Line ranges preserve their original terminators.

Example call arguments:

```json
{"ref":"ts_REPLACE_WITH_FULL_HASH","start_line":120,"end_line":160,"stream":"stderr"}
```

Search is literal, case-sensitive and line-local, without regex interpretation.
It defaults to 20 matches. Continue with `start_line` equal to `next_line` while
`has_more` is true. Original pages default to 16 KiB; continue with `offset` equal
to `next_offset` while `has_more` is true.

## Limits and failure behavior

- Maximum original page or accumulated range/search text: **64 KiB**. Serialized
  JSON and the compatibility text block add overhead beyond this text limit.
- Maximum line range or search match count: **500**. Query length: 1–4096 UTF-8 bytes.
- An oversized line/range returns an explicit error; use original byte pagination.
  No line is silently truncated. Ranges ending past EOF return available lines;
  a starting line beyond EOF is an error. Search beyond EOF returns no matches.
- Incoming JSON-RPC messages are limited to **1 MiB**. An oversized message stops
  the connection with an error on stderr. Normal malformed messages receive
  JSON-RPC errors, and subsequent input may still be processed.
- Cache reads use the configured `limits.max_input_mb` limit and verify SHA-256
  and metadata size before returning data. Expired/deleted/corrupt references
  produce tool errors. Recovery cannot restore pruned originals.

Each request decompresses its bounded cache entry before selecting a page. This
is not random-access decompression. Requests execute sequentially; cancellation
notifications do not interrupt a read already in progress. The server does not
list cache entries, prune/clear files, update metadata, or record metrics.
Compression mode `off` and disabled cache writes do not prevent reading existing
originals, matching `cache inspect` behavior.

## Protocol and validation

The server implements the MCP stdio subset for `initialize`, initialized
notification, `ping`, `tools/list` and `tools/call`. It negotiates protocol
`2025-11-25` or `2025-06-18`; an unsupported requested version receives the former,
which the client must accept or disconnect. No resources, prompts, task execution,
sampling, HTTP transport or server-initiated requests are advertised.

See the official [stdio transport](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports),
[lifecycle](https://modelcontextprotocol.io/specification/2025-11-25/basic/lifecycle)
and [tools contract](https://modelcontextprotocol.io/specification/2025-11-25/server/tools).

Validation includes unit tests for protocol errors, initialization, schemas,
pagination, Unicode boundaries, streams, search, oversized input, corruption and
read-only behavior. `make demo` also launches a real MCP subprocess and verifies
original/range/search recovery for actual CLI and both adapter cache entries.
A local smoke test with the official TypeScript MCP SDK 1.17.5 verified handshake,
tool discovery, full and ranged recovery, and missing-reference errors. The SDK
was installed only in a temporary test directory; TokenSlim gains no runtime
SDK dependency. This does not claim registration or authenticated-session testing
in any particular host application.
