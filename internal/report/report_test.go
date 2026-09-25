package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"tokenslim/internal/metrics"
)

func TestReportWeightsAndEscaping(t *testing.T) {
	source := metrics.Summary{Total: metrics.Total{Processed: 2, Changed: 1, OriginalBytes: 1000, OptimizedBytes: 900, EstimatedOriginalTokens: 250, EstimatedOptimizedTokens: 225}, ByCompressor: map[string]metrics.Total{"z": {Processed: 1, OriginalBytes: 100, OptimizedBytes: 0}, "<script>alert(1)</script>": {Processed: 1, OriginalBytes: 900, OptimizedBytes: 900}}, ByRule: map[string]metrics.RuleTotal{"health": {Records: 1, Groups: 2, Lines: 20}}, ByReason: map[string]int{"compressed": 1, "unchanged": 1}}
	snapshot := Build(source, "</script><img src=x onerror=alert(1)>")
	if snapshot.Total.ReductionPercent != 10 || snapshot.Total.EstimatedSavedTokens != 25 || snapshot.Tools[0].Name != "<script>alert(1)</script>" {
		t.Fatal(snapshot)
	}
	for _, format := range []string{"text", "json", "html"} {
		var a, b bytes.Buffer
		if err := Render(&a, snapshot, format); err != nil {
			t.Fatal(err)
		}
		_ = Render(&b, snapshot, format)
		if a.String() != b.String() {
			t.Fatal("unstable snapshot")
		}
		if format == "json" && !json.Valid(a.Bytes()) {
			t.Fatal("invalid JSON")
		}
		if format == "html" && (strings.Contains(a.String(), "<script>alert(1)</script>") || strings.Contains(a.String(), "<img src=x")) {
			t.Fatal("unescaped HTML")
		}
	}
	var out bytes.Buffer
	if err := Render(&out, Build(metrics.Summary{}, ""), "html"); err != nil || !strings.Contains(out.String(), "No metrics recorded") {
		t.Fatal(out.String(), err)
	}
	if err := Render(&out, snapshot, "unknown"); err == nil {
		t.Fatal("unknown format")
	}
}
