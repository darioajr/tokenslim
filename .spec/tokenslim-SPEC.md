# TokenSlim — SPEC.md

**Status:** Draft / MVP  
**Version:** 0.1.0  
**Date:** 2026-09-19  
**Primary target:** Claude Code  
**Implementation language:** Go  
**License suggestion:** Apache-2.0  
**Working name:** TokenSlim

---

## 1. Executive Summary

TokenSlim is a local Claude Code plugin that reduces unnecessary LLM context created by tool outputs.

The project does **not** minify or rewrite source code being edited. Instead, it intercepts high-volume outputs produced by tools such as builds, tests, logs, Kubernetes, Docker, Terraform and Ansible, removes low-value repetition/noise, preserves diagnostically important information, and only then allows the result to enter Claude's context.

Example:

```text
Claude Code
    |
    | Bash: mvn test
    v
30,000-token output
    |
    v
TokenSlim PostToolUse hook
    |
    +-- classify output
    +-- remove ANSI/noise
    +-- aggregate repeated lines
    +-- preserve errors/failures
    +-- store original locally
    |
    v
4,500-token optimized output
    |
    v
Claude Cloud
```

Primary objective:

> Reduce token consumption caused by tool output without degrading Claude Code's ability to understand, debug and modify the real project.

TokenSlim must prefer correctness over token savings.

---

# 2. Problem

Coding agents frequently generate large quantities of context from tool execution.

Typical examples:

- `mvn test`
- `gradle build`
- `npm test`
- `pnpm test`
- `pytest`
- `go test`
- `cargo test`
- `kubectl logs`
- `docker compose logs`
- `terraform plan`
- `ansible-playbook`
- CI command output
- stack traces
- JSON responses
- dependency installation
- repetitive compiler output

A command may produce tens of thousands of tokens although only a small percentage is useful for the next reasoning step.

Example:

```text
[INFO] Scanning for projects...
[INFO] ...
[INFO] ...
[INFO] ...
Downloading ...
Downloaded ...
...
Tests run: 312
Failures: 2
...
```

The useful information may only be:

```text
Tests: 312
Passed: 310
Failed: 2

FAIL PaymentServiceTest.refundExpiredPayment
PaymentService.java:221
Expected: REJECTED
Actual: APPROVED
```

Sending all original output:

1. consumes context window capacity;
2. increases model input token usage;
3. can trigger context compaction earlier;
4. increases latency;
5. adds noise to the model's reasoning;
6. increases monetary/API usage where token-based billing applies.

---

# 3. Product Principle

TokenSlim is **not a source-code minifier**.

The following must remain untouched:

```text
project files
source formatting
git working tree
tool execution
tool arguments
tool side effects
```

Only the representation of eligible tool output delivered to Claude is transformed.

Core rule:

> Never modify source code merely to save context tokens.

For code that Claude needs to edit, Claude should continue using the original file through its native `Read`, `Edit`, `Write`, LSP and other Claude Code capabilities.

---

# 4. Why a Claude Code Plugin

Claude Code supports lifecycle hooks.

The first implementation uses:

```text
PostToolUse
```

The plugin receives the completed tool result locally, processes it, then returns an `updatedToolOutput`.

Conceptually:

```text
tool executes
    |
    v
original result
    |
    v
PostToolUse hook
    |
    v
TokenSlim
    |
    +---- original saved locally
    |
    v
optimized result
    |
    v
updatedToolOutput
    |
    v
Claude context
```

This makes TokenSlim complementary to Claude Code's own context management.

Claude remains responsible for:

- deciding which files to inspect;
- reading source files;
- editing source files;
- searching symbols;
- using subagents;
- normal context compaction.

TokenSlim is responsible for:

- reducing tool-output noise before it enters the model context.

---

# 5. Goals

## 5.1 Primary goals

TokenSlim must:

1. run entirely on the user's machine;
2. install as a Claude Code plugin;
3. intercept supported tool outputs using Claude Code hooks;
4. reduce repetitive and low-value output;
5. preserve errors, warnings and diagnostic context;
6. preserve source code returned by normal file reads in MVP;
7. preserve the original tool output in a local cache when lossy compression occurs;
8. provide measurable token/character savings;
9. add minimal latency;
10. fail open: when uncertain or broken, return the original output.

## 5.2 Secondary goals

TokenSlim should eventually support:

- Codex CLI;
- Gemini CLI;
- other agentic coding tools;
- MCP tool outputs;
- customizable compressor rules;
- organization-wide policies;
- telemetry export controlled by the user.

---

# 6. Non-Goals

MVP will **not**:

- modify files to reduce indentation;
- minify Java, Kotlin, Go, TypeScript, Python or other source code;
- replace Claude Code's own source discovery;
- create a vector database of the entire repository;
- summarize arbitrary source files using another cloud LLM;
- proxy Anthropic API traffic;
- inspect private source remotely;
- send telemetry to an external TokenSlim service;
- call another AI model in order to compress results;
- guarantee exact provider token counts.

---

# 7. Safety Model

Compression can remove information Claude later needs.

Therefore TokenSlim follows four rules.

## 7.1 Fail open

If parsing, classification or compression fails:

```text
return original tool output
```

Never block the Claude workflow because TokenSlim failed.

## 7.2 Preserve diagnostic priority

Never intentionally discard:

- `ERROR`
- `FATAL`
- `FAIL`
- assertion messages
- exception names
- root causes
- failed test names
- file names related to errors
- line numbers
- exit status information
- compiler errors
- security warnings
- Terraform destructive action summaries
- Ansible failed/unreachable hosts

## 7.3 Source reads are untouched by default

MVP configuration:

```yaml
tools:
  Read:
    enabled: false
  Edit:
    enabled: false
  Write:
    enabled: false
```

This avoids changing text Claude may later use for exact edits.

## 7.4 Original output is recoverable

When TokenSlim performs lossy compression, it stores the original result locally and returns a reference.

Example:

```text
[TokenSlim ref=ts_01JABCD original=184210B optimized=22881B]
```

Future MCP support can allow Claude to retrieve that original content selectively.

---

# 8. Processing Modes

TokenSlim supports three modes.

## 8.1 Off

```yaml
mode: off
```

No transformation.

Useful for debugging.

## 8.2 Safe — default

```yaml
mode: safe
```

Allowed transformations:

- remove ANSI terminal control sequences;
- remove trailing whitespace;
- collapse excessive blank lines;
- normalize carriage returns;
- optionally collapse immediately repeated identical lines with an explicit count;
- preserve all error/failure sections;
- minify standalone machine JSON only when detected with high confidence;
- truncate obvious progress-bar redraws.

Safe mode should avoid semantic summarization.

## 8.3 Smart

```yaml
mode: smart
```

Includes Safe plus structured deterministic reducers:

- build-output reduction;
- test aggregation;
- log deduplication;
- stack-trace repetition reduction;
- dependency-download suppression;
- Kubernetes log grouping;
- Docker log grouping;
- Terraform section reduction;
- Ansible task-result reduction.

No LLM is required.

---

# 9. Architecture

```text
+------------------------------------------------------+
|                    Claude Code                       |
+--------------------------+---------------------------+
                           |
                    successful tool call
                           |
                           v
+------------------------------------------------------+
|                  PostToolUse Hook                    |
+--------------------------+---------------------------+
                           |
                           v
+------------------------------------------------------+
|                    TokenSlim Core                    |
|                                                      |
|  1. Input Adapter                                    |
|  2. Eligibility                                      |
|  3. Classifier                                       |
|  4. Sanitizer                                        |
|  5. Compressor Registry                              |
|  6. Integrity Guard                                  |
|  7. Cache                                            |
|  8. Metrics                                          |
|  9. Output Adapter                                   |
+---------------+--------------------+-----------------+
                |                    |
                v                    v
        ~/.tokenslim/cache     ~/.tokenslim/stats.db
                |
                v
          original output
```

---

# 10. Repository Layout

Recommended repository:

```text
tokenslim/
├── .claude-plugin/
│   └── plugin.json
│
├── hooks/
│   └── hooks.json
│
├── skills/
│   └── tokenslim/
│       └── SKILL.md
│
├── cmd/
│   └── tokenslim/
│       └── main.go
│
├── internal/
│   ├── hook/
│   │   ├── input.go
│   │   └── output.go
│   │
│   ├── classifier/
│   │   └── classifier.go
│   │
│   ├── compressor/
│   │   ├── compressor.go
│   │   ├── generic.go
│   │   ├── json.go
│   │   ├── logs.go
│   │   ├── maven.go
│   │   ├── gradle.go
│   │   ├── node_test.go
│   │   ├── pytest.go
│   │   ├── go_test.go
│   │   ├── kubernetes.go
│   │   ├── docker.go
│   │   ├── terraform.go
│   │   └── ansible.go
│   │
│   ├── cache/
│   │   └── cache.go
│   │
│   ├── config/
│   │   └── config.go
│   │
│   ├── metrics/
│   │   └── metrics.go
│   │
│   └── tokenestimate/
│       └── estimate.go
│
├── testdata/
│   ├── maven/
│   ├── node/
│   ├── kubernetes/
│   ├── terraform/
│   └── ansible/
│
├── docs/
│   ├── architecture.md
│   └── compression-rules.md
│
├── go.mod
├── Makefile
├── LICENSE
├── README.md
└── SPEC.md
```

---

# 11. Claude Plugin Manifest

Example:

```json
{
  "name": "tokenslim",
  "description": "Local tool-output compression for Claude Code",
  "version": "0.1.0"
}
```

The plugin is intentionally thin.

Most implementation lives inside the Go binary.

---

# 12. Hook Configuration

Initial scope: Bash output only.

Example conceptual `hooks/hooks.json`:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "${CLAUDE_PLUGIN_ROOT}/bin/tokenslim hook post-tool-use"
          }
        ]
      }
    ]
  }
}
```

Later versions may add MCP tool output and selected additional tools.

Do not enable `Read` transformation in MVP.

---

# 13. Hook Input

The hook reads JSON from stdin.

TokenSlim must deserialize only fields it needs and tolerate additional future fields.

Conceptual model:

```go
type PostToolUseInput struct {
    SessionID     string          `json:"session_id"`
    HookEventName string          `json:"hook_event_name"`
    ToolName      string          `json:"tool_name"`
    ToolInput     json.RawMessage `json:"tool_input"`
    ToolResponse  json.RawMessage `json:"tool_response"`
    ToolUseID     string          `json:"tool_use_id"`
}
```

For Bash:

```go
type BashInput struct {
    Command string `json:"command"`
}
```

and the current Claude Code output shape must be represented exactly by the adapter.

Important:

> `updatedToolOutput` must match Claude Code's expected output schema for the corresponding tool.

Unknown/unrecognized output shapes must pass through unchanged.

---

# 14. Hook Output

Conceptual response:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PostToolUse",
    "updatedToolOutput": {
      "stdout": "...optimized...",
      "stderr": "...optimized...",
      "interrupted": false,
      "isImage": false
    }
  }
}
```

No extra text should be printed to stdout because stdout is part of hook protocol communication.

Operational/debug logs must go to stderr or a file depending on Claude Code hook behavior and TokenSlim debug mode.

---

# 15. Compression Pipeline

Every supported output passes through:

```text
INPUT
  |
  v
validate hook payload
  |
  v
eligibility check
  |
  v
detect command/output type
  |
  v
baseline measurement
  |
  v
sanitization
  |
  v
domain compressor
  |
  v
integrity guard
  |
  +---- rejected ---> original
  |
  v
cache original if lossy
  |
  v
append compact metadata marker
  |
  v
record metrics
  |
  v
updatedToolOutput
```

---

# 16. Eligibility Rules

Compression should only happen when useful.

Default:

```yaml
thresholds:
  minimum_bytes: 4096
  minimum_lines: 40
  minimum_expected_reduction_percent: 10
```

If output is below thresholds, return it unchanged.

Example:

```text
npm test
10 lines
```

Do nothing.

Example:

```text
kubectl logs
45,000 lines
```

Compress.

---

# 17. Command Classification

Classification must be deterministic.

Use command + output signatures.

Examples:

```text
mvn, mvnw
    => maven

gradle, gradlew
    => gradle

npm test, npm run test, npx jest
pnpm test
yarn test
vitest
    => node-test

pytest
    => pytest

go test
    => go-test

cargo test
    => rust-test

kubectl logs
    => kubernetes-log

docker logs
docker compose logs
    => docker-log

terraform plan
terraform apply
    => terraform

ansible-playbook
    => ansible
```

If no specialized compressor matches:

```text
generic
```

---

# 18. Generic Sanitizer

The generic sanitizer runs before specialized compressors.

Operations:

```text
strip ANSI sequences
normalize CRLF
collapse progress-bar redraws
trim line-ending whitespace
collapse >2 blank lines
remove terminal spinner frames
```

It must not alter substantive textual content.

---

# 19. Repeated-Line Compression

Example input:

```text
Connecting to database
Connecting to database
Connecting to database
Connecting to database
Connection failed
```

Output:

```text
Connecting to database [repeated x4]
Connection failed
```

Rules:

- only exact normalized matches in Safe mode;
- count must always be explicit;
- never collapse lines classified as fatal/error unless exactly identical and adjacent;
- preserve first occurrence;
- preserve relative ordering.

---

# 20. Timestamp-Aware Log Compression

Smart mode may identify lines differing only by timestamp.

Input:

```text
2026-09-19T20:10:01 INFO Retrying connection
2026-09-19T20:10:02 INFO Retrying connection
2026-09-19T20:10:03 INFO Retrying connection
2026-09-19T20:10:04 INFO Retrying connection
```

Output:

```text
INFO Retrying connection [x4; 20:10:01..20:10:04]
```

Requirements:

- preserve first timestamp;
- preserve last timestamp;
- preserve count;
- only group known timestamp formats;
- do not group lines with different severity;
- do not group different exception/error messages.

---

# 21. Stack Trace Compression

The compressor may collapse known repeated framework frames.

It must preserve:

- exception type;
- exception message;
- `Caused by`;
- application frames;
- first framework frames surrounding application frames;
- file/line references;
- suppressed exceptions when meaningful.

Example:

```text
java.lang.IllegalStateException: payment invalid
  at com.acme.PaymentService.refund(PaymentService.java:221)
  ...
  [24 repeated Spring framework frames omitted]
Caused by: ...
```

The exact omitted-frame count must be returned.

---

# 22. Maven Compressor

Preserve:

- compilation errors;
- test failures;
- plugin errors;
- BUILD SUCCESS/FAILURE;
- module summary;
- failed test names;
- Surefire/Failsafe failure details;
- application stack frames.

Candidates for reduction:

- dependency download progress;
- repeated `[INFO]`;
- repeated repository transfer status;
- unchanged reactor boilerplate;
- repeated successful test lines.

Example output:

```text
[TokenSlim:maven]

Build: FAILURE
Modules: 8
Failed modules: payment-service

Tests:
  total: 312
  passed: 310
  failed: 2

Failures:

1. PaymentServiceTest.refundExpiredPayment
   PaymentServiceTest.java:188
   Expected: REJECTED
   Actual: APPROVED

2. PaymentControllerTest.refund
   PaymentControllerTest.java:94
   Expected HTTP: 400
   Actual HTTP: 500

Relevant errors:
...
```

---

# 23. Gradle Compressor

Similar behavior to Maven.

Preserve:

- failed tasks;
- compiler messages;
- test failures;
- stack causes;
- build result;
- affected modules.

Reduce:

- progress output;
- task success boilerplate;
- dependency transfer noise.

---

# 24. Node Test Compressor

Initial targets:

```text
Jest
Vitest
Mocha
npm test wrapper output
```

Preserve:

- failed suite names;
- failed test names;
- assertion differences;
- relevant stack lines;
- summary;
- snapshots failed/updated;
- unhandled promise errors.

Successful tests may be summarized.

Input:

```text
PASS src/a.test.ts
PASS src/b.test.ts
PASS src/c.test.ts
...
FAIL src/payment.test.ts
...
```

Output:

```text
Test suites: 48
Passed: 47
Failed: 1

FAIL src/payment.test.ts
  refund expired payment
  expected "REJECTED"
  received "APPROVED"

  src/payment.test.ts:88
```

---

# 25. Pytest Compressor

Preserve:

- failing test node IDs;
- assertion diffs;
- traceback application frames;
- warnings summary;
- test summary.

Reduce:

- successful test dots;
- repetitive captured output;
- repeated framework internals where safe.

---

# 26. Go Test Compressor

Preserve:

```text
--- FAIL
panic
race detector findings
package name
file:line
assertion output
coverage summary when requested
```

Successful package output may be aggregated.

---

# 27. Kubernetes Log Compressor

Target:

```text
kubectl logs
kubectl logs -f
```

Preserve:

- ERROR/FATAL/PANIC;
- WARN by default;
- exception blocks;
- pod/container identity when available;
- restart/crash indicators;
- readiness/liveness failures;
- unique messages;
- first/last timestamps of grouped repetitions.

Potentially reduce:

- health endpoint access logs;
- heartbeat messages;
- repeated retries;
- identical framework startup lines;
- repetitive successful request patterns.

Configuration:

```yaml
compressors:
  kubernetes:
    group_repeated_lines: true
    group_timestamp_variants: true
    keep_warning_lines: true
    keep_error_lines: true
```

---

# 28. Docker Log Compressor

Targets:

```text
docker logs
docker compose logs
```

Additional requirement:

Preserve service/container prefix while grouping.

Input:

```text
api-1 | INFO retry database
api-1 | INFO retry database
api-1 | INFO retry database
```

Output:

```text
api-1 | INFO retry database [x3]
```

Never merge messages from different containers unless configured.

---

# 29. Terraform Compressor

Terraform requires conservative handling.

Preserve:

- resource addresses;
- actions;
- add/change/destroy counts;
- errors;
- provider errors;
- drift;
- replacement indicators;
- sensitive/destructive warnings.

Output example:

```text
Terraform plan summary

Add:     4
Change:  2
Destroy: 1

Destructive changes:
- aws_instance.legacy

Changed:
- aws_security_group.api
- aws_db_instance.main

Warnings:
...

Errors:
none
```

For the MVP, Terraform compression should default to **Safe**, regardless of global Smart mode, until strong regression tests exist.

---

# 30. Ansible Compressor

Preserve:

- `failed`;
- `fatal`;
- `unreachable`;
- changed hosts;
- task names associated with failure;
- final recap;
- error details;
- return codes;
- stderr.

Reduce:

- repeated `ok`;
- skipped tasks when large;
- duplicated task headers;
- verbose connection/debug output unless requested.

Example:

```text
Ansible summary

Hosts:
app01  ok=41 changed=3 failed=0 unreachable=0
app02  ok=28 changed=1 failed=1 unreachable=0

Failure:
host: app02
task: Deploy application
rc: 1
stderr:
...
```

---

# 31. JSON Handling

Do not blindly minify every JSON-looking block inside logs.

Standalone JSON can be minified only if:

1. the entire relevant output parses successfully;
2. serialization round-trip preserves data;
3. output size is meaningfully reduced.

Example:

```json
{
  "name": "pollify",
  "active": true
}
```

becomes:

```json
{"name":"pollify","active":true}
```

Large API responses should **not** have arbitrary fields removed in MVP.

---

# 32. Original Output Cache

Default path:

```text
~/.tokenslim/cache/
```

Suggested structure:

```text
~/.tokenslim/
├── cache/
│   └── 2026/
│       └── 09/
│           └── ts_01J....zst
├── stats.db
└── config.yaml
```

Use:

```text
zstd
```

for cached original output.

Cache record:

```go
type CacheRecord struct {
    ID             string
    CreatedAt      time.Time
    SessionID      string
    ToolUseID      string
    CommandHash    string
    OriginalBytes  int64
    OptimizedBytes int64
    Compressor     string
    Mode           string
}
```

Do not store tool arguments that may contain secrets unless necessary.

---

# 33. Cache Security

The cache may contain:

- source snippets;
- logs;
- credentials accidentally printed by applications;
- tokens;
- customer data.

Requirements:

```text
directory permission: 0700
file permission:      0600
```

Never upload cached content.

Provide:

```bash
tokenslim cache clear
tokenslim cache prune
tokenslim cache inspect <ref>
```

Default retention:

```yaml
cache:
  enabled: true
  retention: 7d
  max_size_mb: 1024
```

---

# 34. Secret Handling

TokenSlim must not claim to be a secret scanner.

However, cached metadata/logging must avoid echoing obvious secrets.

The debug logger should redact common forms:

```text
Authorization: Bearer ...
ANTHROPIC_API_KEY=...
OPENAI_API_KEY=...
AWS_SECRET_ACCESS_KEY=...
password=...
token=...
```

The optimized output sent to Claude should not silently redact content unless the user explicitly enables redaction, because that changes semantics.

---

# 35. Recovery Marker

Lossy output should include a compact marker.

Example:

```text
[TokenSlim ref=ts_01JQ... compressor=k8s-log reduction=78%]
```

The marker should be short.

Do not include long explanations.

---

# 36. Future MCP Recovery Server

Phase 2 introduces an MCP server bundled with the plugin.

Tools:

```text
tokenslim_get_original
tokenslim_get_range
tokenslim_search_original
tokenslim_stats
```

Example:

```json
{
  "ref": "ts_01JQ...",
  "start_line": 1200,
  "end_line": 1270
}
```

This allows Claude to request the original omitted region only when needed.

Concept:

```text
compressed result
      |
      | ref=ts_x
      v
Claude detects missing detail
      |
      v
TokenSlim MCP
      |
      v
local cache
      |
      v
exact original fragment
```

This feature is intentionally excluded from MVP 0.1 unless implementation proves simple.

---

# 37. Metrics

TokenSlim should measure:

```text
original bytes
optimized bytes
original lines
optimized lines
estimated original tokens
estimated optimized tokens
processing duration
compressor used
compression ratio
```

Token counts are estimates by default.

Do not represent local estimates as exact provider billing tokens.

---

# 38. Token Estimation

MVP:

```text
estimated_tokens
```

Use a configurable local heuristic.

Possible implementations:

```text
UTF-8 character/token approximation
language-aware approximation
optional pluggable tokenizer
```

The most important invariant is that before/after estimates use the same algorithm.

Display:

```text
Estimated token reduction: 63%
```

not:

```text
Exactly 12,331 tokens saved
```

unless an exact supported tokenizer/counting mechanism is later implemented.

---

# 39. CLI

Required commands:

```bash
tokenslim version
tokenslim status
tokenslim stats
tokenslim config show
tokenslim config path
tokenslim benchmark <file>
tokenslim optimize <file>
tokenslim cache inspect <ref>
tokenslim cache clear
tokenslim cache prune
tokenslim hook post-tool-use
```

Examples:

```bash
tokenslim benchmark build.log
```

Output:

```text
TokenSlim Benchmark

Type                 maven
Mode                 safe

Original
  bytes              1,204,882
  lines              14,928
  estimated tokens   287,340

Optimized
  bytes                221,043
  lines                2,118
  estimated tokens    54,118

Estimated reduction   81.2%
Processing             18 ms
```

---

# 40. Statistics

Example:

```bash
tokenslim stats
```

Output:

```text
TokenSlim

Current session
------------------------------------------------
Tool outputs processed             28
Original bytes                 8.42 MB
Optimized bytes                2.11 MB
Estimated reduction              74.9%
Processing time                 143 ms

By compressor
------------------------------------------------
maven                            72%
kubernetes-log                   88%
generic                          19%
json                             31%
```

Global statistics should be optional.

---

# 41. Configuration

Global:

```text
~/.tokenslim/config.yaml
```

Project override:

```text
.tokenslim.yaml
```

Example:

```yaml
version: 1

mode: safe

thresholds:
  minimum_bytes: 4096
  minimum_lines: 40
  minimum_expected_reduction_percent: 10

cache:
  enabled: true
  retention: 7d
  max_size_mb: 1024

tools:
  Bash:
    enabled: true
  Read:
    enabled: false

compressors:
  maven:
    enabled: true

  gradle:
    enabled: true

  node_test:
    enabled: true

  kubernetes:
    enabled: true

  docker:
    enabled: true

  terraform:
    enabled: true
    mode: safe

  ansible:
    enabled: true

metrics:
  enabled: true

debug:
  enabled: false
```

Precedence:

```text
CLI flags
>
project config
>
global config
>
defaults
```

---

# 42. Performance Requirements

Target processing overhead:

```text
< 10 MB output:
p95 under 100 ms

< 1 MB output:
p95 under 20 ms
```

Large outputs must be streamed where practical.

Avoid loading unbounded output into multiple copies in memory.

Maximum default processable payload:

```yaml
limits:
  max_input_mb: 100
```

Above the limit, use a conservative streaming reducer or pass through according to configuration.

---

# 43. Determinism

The same:

```text
input + configuration + TokenSlim version
```

must produce the same optimized output.

No random sampling.

No remote LLM summarization.

This makes behavior testable and reproducible.

---

# 44. Integrity Guard

After compression, validate:

1. output is non-empty unless original was empty;
2. if original contains recognized errors, optimized output contains corresponding error fingerprints;
3. compression ratio is plausible;
4. critical summary fields remain;
5. output is valid for required hook schema.

If guard fails:

```text
return original
```

Example critical fingerprint:

```text
Exception class
failed test identifier
ERROR message hash
resource address
Ansible failed task name
```

---

# 45. Compression Budget

Do not compress simply because possible.

Default desired ceiling:

```yaml
target:
  max_output_bytes: 200000
  max_reduction_percent: 95
```

`max_reduction_percent` prevents an overly aggressive compressor from turning a huge diagnostic output into an untrustworthy tiny summary.

Specialized tests can override this later.

---

# 46. Testing Strategy

Each compressor must have golden fixtures.

Structure:

```text
testdata/maven/
├── success-large.input.txt
├── success-large.expected.txt
├── failure.input.txt
└── failure.expected.txt
```

Tests:

```text
unit tests
golden output tests
property/invariant tests
fuzz tests
integration hook tests
performance benchmarks
```

---

# 47. Required Invariants

Automated tests must verify:

```text
ERROR lines survive
failed test names survive
source file:line references survive
exit/failure summary survives
order of distinct critical errors survives
original cache hash matches original output
unknown tool shape returns original output
invalid hook JSON never causes destructive behavior
```

---

# 48. Evaluation Dataset

Build a local corpus of real outputs.

Minimum initial dataset:

```text
20 Maven logs
20 Gradle logs
20 Node/Jest/Vitest logs
20 pytest outputs
20 Kubernetes logs
20 Docker logs
10 Terraform plans/errors
20 Ansible outputs
20 generic outputs
```

Include:

```text
success
failure
warnings
large repetition
Unicode
ANSI color
very long lines
mixed stdout/stderr
```

No customer/private data in the public repository.

---

# 49. Quality Metrics

Track:

```text
compression ratio
critical-information retention
processing latency
fallback rate
false compression rate
regression rate
```

Most important metric:

```text
Critical Information Retention = 100%
```

for the curated failure corpus.

Token reduction is secondary to this.

---

# 50. MVP Scope — 0.1

Ship only:

```text
Claude Code plugin
Go binary
PostToolUse/Bash integration
generic sanitizer
repeated-line reducer
Maven reducer
Node test reducer
Kubernetes log reducer
Docker log reducer
local cache
CLI stats
benchmark command
safe mode
project/global config
golden tests
```

Do not delay MVP for:

```text
MCP recovery
GUI
Codex
Gemini
Gradle specialization
Terraform smart compression
Ansible smart compression
exact token counting
cloud telemetry
```

---

# 51. MVP User Story

A developer installs TokenSlim.

They run Claude Code normally:

```bash
claude
```

Claude executes:

```bash
mvn test
```

The command outputs 2 MB.

TokenSlim:

```text
detects Maven
stores the original locally
removes download/progress noise
summarizes successful tests
preserves 2 failures verbatim enough for diagnosis
returns optimized Bash output
records metrics
```

Claude sees the optimized result and continues debugging normally.

The user runs:

```bash
tokenslim stats
```

and sees the estimated reduction.

No project file was modified by TokenSlim.

---

# 52. Installation UX

Desired future installation:

```bash
brew install tokenslim
```

or:

```bash
curl -fsSL https://.../install.sh | sh
```

Plugin installation should eventually be available through a Claude Code plugin marketplace.

Development installation:

```bash
git clone ...
cd tokenslim
make build

claude --plugin-dir .
```

---

# 53. Developer Build

Suggested Make targets:

```bash
make build
make test
make benchmark
make lint
make install
make plugin-test
```

Output binary:

```text
bin/tokenslim
```

Support initially:

```text
darwin/arm64
darwin/amd64
linux/amd64
linux/arm64
```

Windows can be phase 2 unless straightforward.

---

# 54. Logging

TokenSlim must be quiet by default.

No normal operational messages should pollute hook stdout.

Debug logs:

```text
~/.tokenslim/logs/tokenslim.log
```

Enable:

```bash
TOKENSLIM_DEBUG=1 claude
```

or:

```yaml
debug:
  enabled: true
```

---

# 55. Failure Behavior

Any unexpected exception/panic must result in one of:

```text
no replacement output
```

or:

```text
original tool output
```

depending on hook protocol requirements.

TokenSlim must never intentionally replace an output with:

```text
compression failed
```

because Claude would lose the original diagnostic data.

---

# 56. Privacy

TokenSlim is local-first.

MVP guarantees:

```text
No TokenSlim cloud service.
No analytics endpoint.
No user source upload.
No original-output upload.
No model call performed by TokenSlim.
```

The only cloud interaction remains whatever Claude Code itself performs.

---

# 57. Threat Model

Token output can be attacker-controlled.

Examples:

```text
malicious package install output
malicious test output
repository scripts
remote Kubernetes logs
```

TokenSlim must treat output as data.

Never:

```text
eval output
execute commands extracted from output
source output as shell
interpret log contents as configuration
```

Parsing must be non-executable.

---

# 58. Extensibility Interface

Compressor interface:

```go
type Compressor interface {
    Name() string
    Detect(ctx DetectContext) bool
    Compress(ctx CompressContext) (Result, error)
}
```

Result:

```go
type Result struct {
    Output       string
    Lossy        bool
    Critical     []Fingerprint
    OriginalSize int
    FinalSize    int
    Metadata     map[string]string
}
```

This allows community compressors.

---

# 59. Compressor Registry

Example:

```go
registry.Register(
    MavenCompressor{},
    NodeTestCompressor{},
    KubernetesLogCompressor{},
    DockerLogCompressor{},
    GenericCompressor{},
)
```

Resolution order:

```text
specific
>
generic
```

Exactly one primary compressor should process an output in MVP.

---

# 60. Future Agent Adapters

Core compression logic must not depend on Claude-specific types.

Use:

```text
Claude Adapter
      |
      v
Canonical Tool Result
      |
      v
TokenSlim Core
      |
      v
Canonical Optimized Result
      |
      v
Claude Adapter
```

Future:

```text
Claude Code Adapter
Codex Adapter
Gemini CLI Adapter
OpenAI Agents Adapter
MCP Proxy Adapter
```

---

# 61. Future: PreToolUse Optimization

Potential future feature:

Intercept commands before execution and suggest quieter forms.

Example:

```text
mvn test
```

could become, with explicit opt-in:

```text
mvn -q test
```

However this is **out of MVP** because changing command arguments may alter behavior.

TokenSlim should first prove value by post-processing only.

---

# 62. Future: User Rules

Example:

```yaml
rules:
  - name: ignore-healthchecks
    match:
      command: "kubectl logs*"
      regex: 'GET /health'
    action:
      aggregate: true
```

Rules must be deterministic and inspectable.

---

# 63. Future: Interactive Report

Potential TUI:

```text
TokenSlim Session

┌─────────────────────┬──────────┬───────────┬─────────┐
│ Tool                │ Original │ Optimized │ Saving  │
├─────────────────────┼──────────┼───────────┼─────────┤
│ mvn test            │ 84,120   │ 12,882    │ 84.7%   │
│ kubectl logs        │ 91,230   │  9,213    │ 89.9%   │
│ npm test            │ 22,810   │  6,292    │ 72.4%   │
└─────────────────────┴──────────┴───────────┴─────────┘
```

Not required for MVP.

---

# 64. Future: MCP Recovery

Once proven stable, bundle:

```text
tokenslim-mcp
```

Capabilities:

```text
get_original(ref)
get_lines(ref, start, end)
search(ref, query)
describe(ref)
```

This turns compression into effectively reversible context virtualization.

---

# 65. Acceptance Criteria — MVP

MVP is considered complete when all of the following are true:

### Installation

```text
[ ] Plugin loads in Claude Code
[ ] PostToolUse hook executes
[ ] Go binary receives Bash result
```

### Behavior

```text
[ ] Small output passes unchanged
[ ] Unsupported output passes unchanged
[ ] Large generic output is sanitized
[ ] Repeated lines are aggregated
[ ] Maven failure retains failed tests/errors
[ ] Node test failure retains assertions
[ ] Kubernetes error messages survive
[ ] Docker service identity survives
```

### Safety

```text
[ ] Read output is not modified
[ ] Project files are never modified
[ ] Compression errors fail open
[ ] Unknown Bash schema fails open
[ ] Original lossy output is cached
[ ] Cache files have restricted permissions
```

### Metrics

```text
[ ] Original/final size recorded
[ ] Estimated reduction recorded
[ ] CLI stats works
[ ] Benchmark command works
```

### Quality

```text
[ ] Unit tests pass
[ ] Golden tests pass
[ ] Fuzz test does not panic
[ ] Critical-information corpus has 100% required fingerprint retention
```

---

# 66. Suggested Development Milestones

## Milestone 1 — Plugin skeleton

Implement:

```text
plugin manifest
PostToolUse hook
Go binary
stdin JSON parsing
safe pass-through
```

Definition of done:

Claude runs Bash and receives exactly the original result through the adapter.

## Milestone 2 — Generic compression

Implement:

```text
ANSI removal
blank-line normalization
repeated-line aggregation
metrics
benchmark
```

## Milestone 3 — Test/build awareness

Implement:

```text
Maven
Jest/Vitest
generic test summary
```

## Milestone 4 — Runtime logs

Implement:

```text
Kubernetes
Docker
timestamp grouping
stack reducer
```

## Milestone 5 — Cache and release

Implement:

```text
zstd cache
retention
stats DB
configuration
release binaries
documentation
```

## Milestone 6 — Recovery

Optional 0.2:

```text
MCP server
get_original
get_range
search_original
```

---

# 67. Suggested Tech Stack

Core:

```text
Go 1.25+
```

Suggested libraries/features:

```text
standard library first
cobra or urfave/cli for CLI
yaml.v3 for configuration
modernc.org/sqlite or SQLite driver for metrics
klauspost/compress/zstd for cache
```

Avoid unnecessary dependencies.

The hook execution path should remain small and fast.

---

# 68. Example End-to-End

Claude invokes:

```bash
kubectl logs payment-api-77f -n prod --tail=10000
```

Original:

```text
3.4 MB
41,281 lines
~800k estimated tokens
```

TokenSlim detects:

```text
kubernetes-log
```

It finds:

```text
31,021 health-check/access repeats
7,901 retry duplicates
2,213 informational lines
94 warnings
52 errors
```

Optimized result:

```text
[TokenSlim ref=ts_01... compressor=kubernetes-log reduction=91%]

Summary:
- INFO groups: 184
- WARN: 94
- ERROR: 52

Repeated:
- GET /health 200 [x18,241]
- database reconnect attempt [x7,901]
- metrics scrape 200 [x12,780]

Errors:
2026-09-19T21:03:41 ERROR Database connection timeout
...
```

Claude investigates the actual application source using normal Claude Code tools.

TokenSlim never changes that source.

---

# 69. Product Positioning

Short description:

> TokenSlim reduces noisy tool output before it enters coding-agent context.

Longer:

> TokenSlim is a local-first context efficiency plugin for coding agents. It deterministically compresses repetitive build, test, infrastructure and log output while preserving actionable failures and keeping the original result locally recoverable.

Do not market MVP as:

```text
AI code compressor
```

Prefer:

```text
tool-output compressor
context efficiency layer
coding-agent context optimizer
```

---

# 70. Key Design Decision

The most important architectural decision is:

> Optimize what tools produce, not the source code Claude needs to edit.

This avoids conflicting with Claude Code's native source discovery/editing workflow and targets one of the largest avoidable sources of context growth: verbose tool results.

---

# 71. Official Claude Code References

Implementation should always be validated against the currently installed Claude Code version.

Current design is based on:

```text
Claude Code Hooks Reference
https://code.claude.com/docs/en/hooks

Claude Code Plugins
https://code.claude.com/docs/en/plugins
```

Important dependency:

`PostToolUse.updatedToolOutput` must continue to support replacement of the corresponding tool result, and the replacement object must match the tool's required output shape.

If Claude Code changes this contract, the adapter layer must be updated while keeping TokenSlim Core independent.

---

# 72. Codex Implementation Prompt

The repository can be bootstrapped from this SPEC using the following implementation order:

```text
1. Create the Go project and Claude plugin skeleton.
2. Implement exact PostToolUse/Bash adapter with pass-through behavior.
3. Add fixture-based integration tests for Claude hook JSON.
4. Add generic ANSI/whitespace/progress cleanup.
5. Add exact repeated-line compression.
6. Add metrics and benchmark CLI.
7. Add Maven compressor and golden tests.
8. Add Jest/Vitest compressor and golden tests.
9. Add Kubernetes/Docker log compressors.
10. Add local zstd original-output cache.
11. Add configuration and fail-open integrity guard.
12. Package release binaries and plugin installation documentation.
```

At every step:

```text
do not modify project source files;
do not enable Read compression;
do not use a remote LLM;
preserve critical diagnostic information;
fail open to original output;
add tests before expanding compression aggressiveness.
```

---

# 73. Definition of Success

The project is successful if a developer can use Claude Code normally and observe:

```text
materially lower tool-output context volume
+
no meaningful loss in debugging capability
+
no changes to source files
+
minimal added latency
+
fully local processing
```

A useful initial target for real-world sessions is:

```text
30–60% reduction in eligible tool-output volume
```

but this is a **measurement target, not a correctness requirement**.

Correctness and diagnostic retention always take priority over compression ratio.

