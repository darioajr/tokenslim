// Package report renders a stable snapshot of existing local metrics.
package report

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"sort"
	"text/tabwriter"
	"tokenslim/internal/metrics"
)

type Totals struct {
	Processed                int     `json:"processed"`
	Changed                  int     `json:"changed"`
	OriginalBytes            int     `json:"original_bytes"`
	OptimizedBytes           int     `json:"optimized_bytes"`
	SavedBytes               int     `json:"saved_bytes"`
	ReductionPercent         float64 `json:"reduction_percent"`
	EstimatedOriginalTokens  int     `json:"estimated_original_tokens"`
	EstimatedOptimizedTokens int     `json:"estimated_optimized_tokens"`
	EstimatedSavedTokens     int     `json:"estimated_saved_tokens"`
}
type Tool struct {
	Name string `json:"name"`
	Totals
}
type Rule struct {
	Name    string `json:"name"`
	Records int    `json:"records"`
	Groups  int    `json:"groups"`
	Lines   int    `json:"lines"`
}
type Reason struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}
type Snapshot struct {
	Session string   `json:"session,omitempty"`
	Total   Totals   `json:"total"`
	Tools   []Tool   `json:"tools"`
	Rules   []Rule   `json:"rules"`
	Reasons []Reason `json:"reasons"`
}

func totals(t metrics.Total) Totals {
	reduction := 0.0
	if t.OriginalBytes > 0 {
		reduction = 100 * float64(t.OriginalBytes-t.OptimizedBytes) / float64(t.OriginalBytes)
	}
	return Totals{t.Processed, t.Changed, t.OriginalBytes, t.OptimizedBytes, t.OriginalBytes - t.OptimizedBytes, reduction, t.EstimatedOriginalTokens, t.EstimatedOptimizedTokens, t.EstimatedOriginalTokens - t.EstimatedOptimizedTokens}
}
func Build(s metrics.Summary, session string) Snapshot {
	result := Snapshot{Session: session, Total: totals(s.Total), Tools: []Tool{}, Rules: []Rule{}, Reasons: []Reason{}}
	for name, t := range s.ByCompressor {
		result.Tools = append(result.Tools, Tool{name, totals(t)})
	}
	for name, r := range s.ByRule {
		result.Rules = append(result.Rules, Rule{name, r.Records, r.Groups, r.Lines})
	}
	for name, count := range s.ByReason {
		result.Reasons = append(result.Reasons, Reason{name, count})
	}
	sort.Slice(result.Tools, func(i, j int) bool { return result.Tools[i].Name < result.Tools[j].Name })
	sort.Slice(result.Rules, func(i, j int) bool { return result.Rules[i].Name < result.Rules[j].Name })
	sort.Slice(result.Reasons, func(i, j int) bool { return result.Reasons[i].Name < result.Reasons[j].Name })
	return result
}

//go:embed report.html
var htmlSource string
var htmlReport = template.Must(template.New("report").Parse(htmlSource))

func Render(out io.Writer, s Snapshot, format string) error {
	switch format {
	case "json":
		e := json.NewEncoder(out)
		e.SetIndent("", "  ")
		return e.Encode(s)
	case "html":
		return htmlReport.Execute(out, s)
	case "text":
		w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "TokenSlim report — local metrics; tokens estimated")
		if s.Session != "" {
			fmt.Fprintf(w, "Session: %q\n", s.Session)
		}
		fmt.Fprintf(w, "Processed: %d | Compressed: %d | Saved: %d B (%.1f%%) | Estimated tokens saved: %d\n\n", s.Total.Processed, s.Total.Changed, s.Total.SavedBytes, s.Total.ReductionPercent, s.Total.EstimatedSavedTokens)
		fmt.Fprintln(w, "Tool family\tOutputs\tCompressed\tOriginal B\tOptimized B\tSaved B\tReduction\tEstimated tokens saved")
		for _, t := range s.Tools {
			fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%d\t%.1f%%\t%d\n", t.Name, t.Processed, t.Changed, t.OriginalBytes, t.OptimizedBytes, t.SavedBytes, t.ReductionPercent, t.EstimatedSavedTokens)
		}
		fmt.Fprintln(w, "\nRule\tOutputs affected\tGroups\tLines grouped (includes retained first line)")
		for _, r := range s.Rules {
			fmt.Fprintf(w, "%s\t%d\t%d\t%d\n", r.Name, r.Records, r.Groups, r.Lines)
		}
		fmt.Fprintln(w, "\nOutcome\tOutputs")
		for _, r := range s.Reasons {
			fmt.Fprintf(w, "%s\t%d\n", r.Name, r.Count)
		}
		return w.Flush()
	default:
		return fmt.Errorf("invalid report format %q (use text, json or html)", format)
	}
}
