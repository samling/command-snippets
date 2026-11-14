package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/samling/command-snippets/internal/library"
	"github.com/samling/command-snippets/internal/models"
	"github.com/samling/command-snippets/internal/search"
	"github.com/samling/command-snippets/internal/templating"
)

func performanceFixture(t *testing.T, count int) string {
	t.Helper()
	dir := t.TempDir()
	main := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(main, []byte("settings:\n  sources: ['source-*.yaml']\n  project_source: false\nsnippets: []\n"), 0600); err != nil {
		t.Fatal(err)
	}
	sources := make([]strings.Builder, 20)
	for i := range sources {
		sources[i].WriteString("snippets:\n")
	}
	for i := 0; i < count; i++ {
		title := fmt.Sprintf("Inspect resource in workspace %05d, current status", i%9000)
		if i%97 == 0 {
			title = fmt.Sprintf("日本語 café resource %05d", i%9000)
		}
		description := strings.Repeat("Inspect resource status in a namespace. ", 4)[:120]
		command := "echo '$0 ${HOME}'; " + strings.Repeat("x", 494)
		if i%100 == 0 {
			command += strings.Repeat("x", 4096-len(command))
		}
		tags := []string{}
		if i%10 != 0 {
			for j := 0; j < 1+i%4; j++ {
				tags = append(tags, fmt.Sprintf("Tag %02d", (i+j)%100))
			}
		}
		fmt.Fprintf(&sources[i%20], "  - name: %q\n    description: %q\n    tags: [%s]\n    command: %q\n", title, description, strings.Join(tags, ", "), command)
	}
	for i := range sources {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("source-%02d.yaml", i)), []byte(sources[i].String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return main
}
func TestPerformanceReceipt(t *testing.T) {
	if testing.Short() || os.Getenv("CS_PERFORMANCE_TEST") != "1" {
		t.Skip("set CS_PERFORMANCE_TEST=1 for development-host acceptance budgets")
	}
	path := performanceFixture(t, 10000)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	started := time.Now()
	lib := library.Load(path, filepath.Dir(path))
	loadDuration := time.Since(started)
	if err := lib.Error(); err != nil {
		t.Fatal(err)
	}
	started = time.Now()
	index := search.New(lib.Entries)
	build := time.Since(started)
	m, err := New(lib, Options{NoColor: true})
	if err != nil {
		t.Fatal(err)
	}
	m.Width = 120
	m.Height = 30
	// Production retains one index. Reuse the separately timed build rather than
	// count a second characterization-only index in the heap receipt.
	m.Index = index
	runtime.GC()
	runtime.ReadMemStats(&after)
	heap := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	queries := []string{"", "Inspect", "irws", "namespace", "Tag 11", "echo", "no-match-ever", "日本語"}
	samples := []time.Duration{}
	for i := 0; i < 240; i++ {
		tags := []string{}
		for j := 0; j < i%4; j++ {
			tags = append(tags, fmt.Sprintf("Tag %02d", 11+j))
		}
		started = time.Now()
		m.Matches = index.Query(queries[i%len(queries)], tags, i%11 == 0)
		m.Selected = 0
		_ = m.View()
		samples = append(samples, time.Since(started))
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	p95 := samples[len(samples)*95/100]
	maximum := samples[len(samples)-1]
	t.Logf("Go=%s CPU=%d terminal=120x30 load=%s index=%s incremental_heap=%.2f MiB query+view p95=%s max=%s samples=%d", runtime.Version(), runtime.NumCPU(), loadDuration, build, float64(heap)/(1<<20), p95, maximum, len(samples))
	if build > 200*time.Millisecond || heap > 64<<20 || p95 > 50*time.Millisecond || maximum > 100*time.Millisecond {
		t.Fatal("normal-workload performance budget exceeded")
	}
	runtime.KeepAlive(m)
	runtime.KeepAlive(index)
	s := models.Snippet{Name: "Preview workload", Command: "echo {{out_0}}", Expressions: map[string]string{}}
	for i := 0; i < 20; i++ {
		s.Inputs = append(s.Inputs, models.Input{Name: fmt.Sprintf("input_%d", i), Default: "value"})
	}
	for i := 0; i < 8; i++ {
		s.Expressions[fmt.Sprintf("out_%d", i)] = fmt.Sprintf("quote(inputs.input_%d)", i)
	}
	compiled, err := templating.Compile(s)
	if err != nil {
		t.Fatal(err)
	}
	samples = nil
	for i := 0; i < 240; i++ {
		started = time.Now()
		result := compiled.Preview(nil)
		if !result.Valid() {
			t.Fatal(result.Error())
		}
		samples = append(samples, time.Since(started))
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	p95 = samples[len(samples)*95/100]
	t.Logf("shared render 20 inputs/8 expressions p95=%s", p95)
	if p95 > 20*time.Millisecond {
		t.Fatal("render budget exceeded")
	}
	stress := performanceFixture(t, 50000)
	started = time.Now()
	large := library.Load(stress, filepath.Dir(stress))
	if err := large.Error(); err != nil {
		t.Fatal(err)
	}
	largeIndex := search.New(large.Entries)
	results := largeIndex.Query("Inspect", nil, false)
	t.Logf("50,000 load/index/query=%s entries=%d results=%d", time.Since(started), len(large.Entries), len(results))
	runtime.KeepAlive(largeIndex)
}
