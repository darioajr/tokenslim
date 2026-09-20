package metrics

import (
	"sync"
	"testing"
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
