// Package engine is independent of agent protocols and shell execution.
package engine

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"
	"tokenslim/internal/cache"
	"tokenslim/internal/classifier"
	"tokenslim/internal/compressor"
	"tokenslim/internal/config"
	"tokenslim/internal/metrics"
	"tokenslim/internal/rules"
	"unicode/utf8"
)

type Request struct {
	Stdout, Stderr, Command, SessionID, ToolUseID, Format string
	Original                                              []byte
}
type Result struct {
	Stdout, Stderr, Ref string
	Metrics             metrics.Record
}
type Engine struct {
	Config config.Config
	Home   string
}

func Lines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}
func (e Engine) Process(q Request, dryRun bool) (r Result) {
	start := time.Now()
	c := e.Config
	original := q.Stdout + q.Stderr
	r.Stdout = q.Stdout
	r.Stderr = q.Stderr
	r.Metrics = metrics.Record{SessionID: q.SessionID, Mode: c.Mode, Compressor: classifier.Detect(q.Command, original), OriginalBytes: len(original), OriginalLines: Lines(q.Stdout) + Lines(q.Stderr), Reason: "unchanged"}
	defer func() {
		m := &r.Metrics
		m.OptimizedBytes = len(r.Stdout) + len(r.Stderr)
		m.OptimizedLines = Lines(r.Stdout) + Lines(r.Stderr)
		m.EstimatedOriginalTokens = int(math.Ceil(float64(utf8.RuneCountInString(original)) / c.TokenEstimate.CharactersPerToken))
		m.EstimatedOptimizedTokens = int(math.Ceil(float64(utf8.RuneCountInString(r.Stdout)+utf8.RuneCountInString(r.Stderr)) / c.TokenEstimate.CharactersPerToken))
		m.DurationNS = time.Since(start).Nanoseconds()
		if c.Debug.Enabled && !dryRun {
			_ = metrics.Debug(e.Home, *m)
		}
		if c.Metrics.Enabled && !dryRun {
			_ = metrics.Write(e.Home, *m)
		}
	}()
	if c.Mode == "off" {
		r.Metrics.Reason = "disabled"
		return
	}
	if !utf8.ValidString(original) || strings.ContainsRune(original, 0) {
		r.Metrics.Reason = "binary output"
		return
	}
	if len(original) > c.Limits.MaxInputMB<<20 {
		r.Metrics.Reason = "input limit"
		return
	}
	if len(original) < c.Thresholds.MinimumBytes || r.Metrics.OriginalLines < c.Thresholds.MinimumLines {
		r.Metrics.Reason = "below thresholds"
		return
	}
	opt, ok := c.Compressors[config.Key(r.Metrics.Compressor)]
	if ok && !opt.Enabled {
		r.Metrics.Reason = "compressor disabled"
		return
	}
	mode := c.Mode
	if opt.Mode != "" {
		mode = opt.Mode
	}
	if r.Metrics.Compressor == "terraform" {
		mode = "safe"
	}
	r.Metrics.Mode = mode
	if mode == "off" {
		r.Metrics.Reason = "compressor mode off"
		return
	}
	reducer := compressor.Reducer{Kind: r.Metrics.Compressor}
	ctx := compressor.Context{Command: q.Command, Mode: mode, GroupTimestamps: opt.GroupTimestampVariants, DisableRepeats: !opt.GroupRepeatedLines}
	if mode == "smart" {
		var err error
		ctx.Rules, err = rules.Compile(c.Rules)
		if err != nil {
			r.Metrics.Reason = "invalid custom rules"
			return
		}
	}
	ar := reducer.Compress(ctx, q.Stdout)
	br := reducer.Compress(ctx, q.Stderr)
	a, b := ar.Output, br.Output
	if !compressor.IntactFor(r.Metrics.Compressor, q.Stdout, a) || !compressor.IntactFor(r.Metrics.Compressor, q.Stderr, b) {
		r.Metrics.Reason = "integrity guard"
		return
	}
	if a == q.Stdout && b == q.Stderr {
		return
	}
	if !c.Cache.Enabled {
		r.Metrics.Reason = "cache disabled; preserving original"
		return
	}
	raw := q.Original
	if raw == nil {
		raw = []byte(original)
	}
	ref := cache.Ref(raw)
	// Marker length is part of the budget; iterate to stabilize its final byte count.
	finalSize := len(a) + len(b)
	marker := ""
	for i := 0; i < 5; i++ {
		marker = fmt.Sprintf("\n[TokenSlim ref=%s compressor=%s original=%dB optimized=%dB]\n", ref, r.Metrics.Compressor, len(original), finalSize)
		n := len(a) + len(b) + len(marker)
		if n == finalSize {
			break
		}
		finalSize = n
	}
	reduction := 100 * (1 - float64(finalSize)/float64(len(original)))
	if reduction < c.Thresholds.MinimumReduction || reduction > c.Target.MaxReduction {
		r.Metrics.Reason = "reduction budget"
		return
	}
	if !dryRun {
		store := cache.Store{Dir: filepath.Join(e.Home, "cache"), MaxBytes: int64(c.Cache.MaxSizeMB) << 20}
		_, err := store.Put(raw, cache.Record{SessionID: q.SessionID, ToolUseID: q.ToolUseID, CommandHash: cache.Hash([]byte(q.Command)), OptimizedBytes: finalSize, Compressor: r.Metrics.Compressor, Mode: mode, Format: q.Format})
		if err != nil {
			r.Metrics.Reason = "cache write failed"
			return
		}
		retention, _ := config.Retention(c.Cache.Retention)
		_, _ = store.Prune(retention, ref)
	}
	if a != "" {
		a += marker
	} else {
		b += marker
	}
	r.Stdout = a
	r.Stderr = b
	r.Ref = ref
	r.Metrics.Rules = map[string]rules.Effect{}
	for _, effects := range []map[string]rules.Effect{ar.Rules, br.Rules} {
		for name, effect := range effects {
			current := r.Metrics.Rules[name]
			current.Groups += effect.Groups
			current.Lines += effect.Lines
			r.Metrics.Rules[name] = current
		}
	}
	r.Metrics.Changed = true
	r.Metrics.Reason = "compressed"
	return
}
