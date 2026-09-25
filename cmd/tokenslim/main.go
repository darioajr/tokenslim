package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go.yaml.in/yaml/v3"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"tokenslim/internal/cache"
	"tokenslim/internal/config"
	"tokenslim/internal/engine"
	"tokenslim/internal/hook"
	"tokenslim/internal/mcp"
	"tokenslim/internal/metrics"
	"tokenslim/internal/recovery"
	"tokenslim/internal/report"
)

var version = "0.1.0"

func main() {
	if e := run(os.Args[1:], os.Stdin, os.Stdout); e != nil {
		fmt.Fprintln(os.Stderr, "tokenslim:", e)
		os.Exit(1)
	}
}
func run(args []string, in io.Reader, out io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		fmt.Fprintln(out, `TokenSlim — local tool-output compression

Commands:
  version | status
  stats [--session ID] [--json]
  config show | path | validate
  report [--session ID] [--format text|json|html]
  optimize [--mode safe|smart|off] [--command COMMAND] FILE|-
  benchmark [--mode safe|smart|off] [--command COMMAND] [--json] FILE|-
  cache inspect REF [--metadata] | clear | prune
  hook post-tool-use [--agent claude|codex]
  mcp serve

Flags must precede the input file. TOKENSLIM_HOME overrides ~/.tokenslim.
Benchmark is a dry run: no cache or metrics writes. Tokens are estimates.`)
		return nil
	}
	if args[0] == "version" {
		fmt.Fprintln(out, "tokenslim", version)
		return nil
	}
	home, e := config.Home()
	if e != nil {
		return e
	}
	cwd, e := os.Getwd()
	if e != nil {
		return e
	}
	if args[0] == "hook" {
		if len(args) < 2 || args[1] != "post-tool-use" {
			return nil
		}
		agent := "claude"
		if len(args) == 4 && args[2] == "--agent" {
			agent = args[3]
		} else if len(args) != 2 {
			return nil
		}
		hook.RunAgent(in, out, home, cwd, agent)
		return nil
	}
	if args[0] == "config" && len(args) == 2 && args[1] == "path" {
		fmt.Fprintln(out, filepath.Join(home, "config.yaml"))
		return nil
	}
	c, e := config.Load(home, cwd)
	if e != nil {
		return e
	}
	switch args[0] {
	case "mcp":
		if len(args) != 2 || args[1] != "serve" {
			return fmt.Errorf("usage: mcp serve")
		}
		return mcp.Serve(in, out, recovery.Service{Store: cache.Store{Dir: filepath.Join(home, "cache"), MaxBytes: int64(c.Limits.MaxInputMB) << 20}}, version)
	case "status":
		fmt.Fprintf(out, "TokenSlim %s\nMode: %s\nHome: %s\nCache: %t\nMetrics: %t\nClaude Code: PostToolUse / updatedToolOutput\nCodex: PostToolUse / continue:false + stopReason\nInstallation and hook trust must be configured in the host.\n", version, c.Mode, home, c.Cache.Enabled, c.Metrics.Enabled)
		return nil
	case "report":
		fs := flag.NewFlagSet("report", flag.ContinueOnError)
		session := fs.String("session", "", "session filter")
		format := fs.String("format", "text", "text, json or html")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("usage: report [--session ID] [--format text|json|html]")
		}
		summary, err := metrics.Read(home, *session)
		if err != nil {
			return err
		}
		return report.Render(out, report.Build(summary, *session), *format)
	case "config":
		if len(args) == 2 && args[1] == "validate" {
			fmt.Fprintln(out, "Configuration valid")
			return nil
		}
		if len(args) != 2 || args[1] != "show" {
			return fmt.Errorf("usage: config show|path|validate")
		}
		b, e := yaml.Marshal(c)
		if e == nil {
			_, e = out.Write(b)
		}
		return e
	case "stats":
		fs := flag.NewFlagSet("stats", flag.ContinueOnError)
		session := fs.String("session", "", "session filter")
		asJSON := fs.Bool("json", false, "JSON output")
		if e := fs.Parse(args[1:]); e != nil {
			return e
		}
		s, e := metrics.Read(home, *session)
		if e != nil {
			return e
		}
		if *asJSON {
			return json.NewEncoder(out).Encode(s)
		}
		fmt.Fprintf(out, "TokenSlim statistics (local; tokens estimated)\nProcessed: %d   Compressed: %d\nOriginal: %d B   Optimized: %d B\nEstimated tokens: %d → %d\nProcessing: %s\n", s.Total.Processed, s.Total.Changed, s.Total.OriginalBytes, s.Total.OptimizedBytes, s.Total.EstimatedOriginalTokens, s.Total.EstimatedOptimizedTokens, time.Duration(s.Total.DurationNS))
		names := make([]string, 0, len(s.ByCompressor))
		for n := range s.ByCompressor {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			t := s.ByCompressor[n]
			fmt.Fprintf(out, "%-18s %d outputs   %d → %d B\n", n, t.Processed, t.OriginalBytes, t.OptimizedBytes)
		}
		return nil
	case "cache":
		if len(args) < 2 {
			return fmt.Errorf("usage: cache inspect REF [--metadata]|clear|prune")
		}
		store := cache.Store{Dir: filepath.Join(home, "cache"), MaxBytes: int64(c.Limits.MaxInputMB) << 20}
		switch args[1] {
		case "inspect":
			if len(args) < 3 || len(args) > 4 {
				return fmt.Errorf("usage: cache inspect REF [--metadata]")
			}
			b, r, e := store.Get(args[2])
			if e != nil {
				return e
			}
			if len(args) == 4 {
				if args[3] != "--metadata" {
					return fmt.Errorf("unknown inspect flag")
				}
				return json.NewEncoder(out).Encode(r)
			}
			_, e = out.Write(b)
			return e
		case "clear":
			if len(args) != 2 {
				return fmt.Errorf("usage: cache clear")
			}
			return store.Clear()
		case "prune":
			if len(args) != 2 {
				return fmt.Errorf("usage: cache prune")
			}
			store.MaxBytes = int64(c.Cache.MaxSizeMB) << 20
			d, _ := config.Retention(c.Cache.Retention)
			n, e := store.Prune(d, "")
			if e == nil {
				fmt.Fprintf(out, "Pruned %d originals\n", n)
			}
			return e
		}
		return fmt.Errorf("unknown cache command")
	case "optimize", "benchmark":
		fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
		mode := fs.String("mode", c.Mode, "off, safe or smart")
		command := fs.String("command", "", "original command for classification")
		asJSON := fs.Bool("json", false, "benchmark JSON")
		if e := fs.Parse(args[1:]); e != nil {
			return e
		}
		if fs.NArg() != 1 {
			return fmt.Errorf("usage: %s [flags] FILE|-", args[0])
		}
		c.Mode = *mode
		if e = c.Validate(); e != nil {
			return e
		}
		reader := in
		if fs.Arg(0) != "-" {
			f, e := os.Open(fs.Arg(0))
			if e != nil {
				return e
			}
			defer f.Close()
			reader = f
		}
		limit := int64(c.Limits.MaxInputMB) << 20
		b, e := io.ReadAll(io.LimitReader(reader, limit+1))
		if e != nil {
			return e
		}
		if int64(len(b)) > limit {
			if args[0] == "benchmark" {
				return fmt.Errorf("input exceeds %d MiB limit", c.Limits.MaxInputMB)
			}
			if _, e = out.Write(b); e != nil {
				return e
			}
			_, e = io.Copy(out, reader)
			return e
		}
		r := (engine.Engine{Config: c, Home: home}).Process(engine.Request{Stdout: string(b), Command: *command, Original: b, Format: "text"}, args[0] == "benchmark")
		if args[0] == "optimize" {
			_, e = io.WriteString(out, r.Stdout)
			return e
		}
		if *asJSON {
			return json.NewEncoder(out).Encode(r.Metrics)
		}
		m := r.Metrics
		reduction := 0.0
		if m.OriginalBytes > 0 {
			reduction = 100 * (1 - float64(m.OptimizedBytes)/float64(m.OriginalBytes))
		}
		fmt.Fprintf(out, "TokenSlim Benchmark\nType: %s   Mode: %s\nOriginal: %d B / %d lines / ~%d tokens\nOptimized: %d B / %d lines / ~%d tokens\nByte reduction: %.1f%%\nProcessing: %s\nResult: %s\n", m.Compressor, m.Mode, m.OriginalBytes, m.OriginalLines, m.EstimatedOriginalTokens, m.OptimizedBytes, m.OptimizedLines, m.EstimatedOptimizedTokens, reduction, time.Duration(m.DurationNS), m.Reason)
		return nil
	}
	return fmt.Errorf("unknown command %q (try help)", strings.Join(args, " "))
}
