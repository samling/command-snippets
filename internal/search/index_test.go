package search

import (
	"fmt"
	"testing"

	"github.com/samling/command-snippets/internal/library"
	"github.com/samling/command-snippets/internal/models"
)

func workload(count int) []*library.Entry {
	sources := make([]*library.Source, 20)
	for i := range sources {
		sources[i] = &library.Source{Path: fmt.Sprintf("source-%02d", i)}
	}
	entries := make([]*library.Entry, count)
	for i := range entries {
		tags := []string{}
		if i%10 != 0 {
			for j := 0; j < 1+i%4; j++ {
				tags = append(tags, fmt.Sprintf("Tag %02d", (i+j)%100))
			}
		}
		entries[i] = &library.Entry{Key: fmt.Sprint(i), Ordinal: i / 20, Source: sources[i%20], Snippet: models.Snippet{Name: fmt.Sprintf("Inspect resource in workspace %05d, current status", i%9000), Description: "Find and inspect the current status of a resource; use tags to narrow the command library by service or project namespace.", Tags: tags, Command: fmt.Sprintf("echo %05d %0512d", i, i)}}
	}
	return entries
}
func TestQueryFieldsTagsAndDeterminism(t *testing.T) {
	entries := workload(3)
	entries[0].Snippet.Name = "List pods"
	entries[0].Snippet.Tags = []string{"Kubernetes", "Monitoring"}
	entries[1].Snippet.Name = "List pods"
	entries[1].Snippet.Tags = []string{"kubernetes"}
	entries[2].Snippet.Name = "日本語 café"
	entries[2].Snippet.Description = "descriptionneedle"
	entries[2].Snippet.Command = "commandneedle"
	entries[2].Snippet.Tags = nil
	index := New(entries)
	for _, query := range []string{"lpd", "list pods"} {
		matches := index.Query(query, nil, false)
		if len(matches) != 2 || matches[0].Entry != entries[0] || matches[1].Entry != entries[1] {
			t.Fatalf("query %s: %+v", query, matches)
		}
	}
	for _, query := range []string{"descriptionneedle", "commandneedle", "日本 café"} {
		matches := index.Query(query, nil, false)
		if len(matches) != 1 || matches[0].Entry != entries[2] {
			t.Fatalf("missing field query %s", query)
		}
	}
	if len(index.Query("", []string{"KUBERNETES", "monitoring"}, false)) != 1 {
		t.Fatal("intersection failed")
	}
	if len(index.Query("", nil, true)) != 1 {
		t.Fatal("untagged failed")
	}
	if len(index.Query("no-match-ever", nil, false)) != 0 {
		t.Fatal("unexpected match")
	}
	if index.Categories[0].Label != "Kubernetes" || index.Categories[0].Count != 2 {
		t.Fatalf("tag spelling/count %+v", index.Categories)
	}
}
func BenchmarkQuery(b *testing.B) {
	for _, count := range []int{10000, 50000} {
		index := New(workload(count))
		for _, query := range []string{"", "Inspect", "irws", "description", "Tag 11", "echo", "no-match-ever", "日本語"} {
			b.Run(fmt.Sprintf("%d/%s", count, query), func(b *testing.B) {
				for n := 0; n < b.N; n++ {
					index.Query(query, nil, false)
				}
			})
		}
		for _, tags := range [][]string{{"Tag 11"}, {"Tag 11", "Tag 12"}, {"Tag 11", "Tag 12", "Tag 13"}} {
			b.Run(fmt.Sprintf("%d/tags-%d", count, len(tags)), func(b *testing.B) {
				for n := 0; n < b.N; n++ {
					index.Query("", tags, false)
				}
			})
		}
	}
}
func BenchmarkIndex(b *testing.B) {
	entries := workload(10000)
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		New(entries)
	}
}
