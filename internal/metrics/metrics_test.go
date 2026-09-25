package metrics

import (
	"sync"
	"testing"
	"tokenslim/internal/rules"
)

func TestConcurrentTotals(t *testing.T) {
	home := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if e := Write(home, Record{SessionID: "a", Compressor: "generic", OriginalBytes: 100, OptimizedBytes: 50, Changed: true}); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	s, e := Read(home, "a")
	if e != nil || s.Total.Processed != 20 || s.Total.OriginalBytes != 2000 || s.Total.OptimizedBytes != 1000 {
		t.Fatal(s, e)
	}
	s, e = Read(home, "other")
	if e != nil || s.Total.Processed != 0 {
		t.Fatal(s, e)
	}
}

func TestRuleAndReasonTotals(t *testing.T) {
	home := t.TempDir()
	for _, record := range []Record{
		{SessionID: "a", Compressor: "generic", Changed: true, Reason: "compressed", Rules: map[string]rules.Effect{"health": {Groups: 2, Lines: 20}}},
		{SessionID: "a", Compressor: "generic", Reason: "cache disabled; preserving original", Rules: map[string]rules.Effect{"health": {Groups: 99, Lines: 999}}},
		{SessionID: "b", Compressor: "generic", Changed: true, Reason: "compressed", Rules: map[string]rules.Effect{"health": {Groups: 1, Lines: 10}}},
	} {
		if err := Write(home, record); err != nil {
			t.Fatal(err)
		}
	}
	summary, err := Read(home, "a")
	if err != nil {
		t.Fatal(err)
	}
	if summary.ByRule["health"] != (RuleTotal{Records: 1, Groups: 2, Lines: 20}) || summary.ByReason["compressed"] != 1 || summary.Total.Processed != 2 {
		t.Fatal(summary)
	}
}
