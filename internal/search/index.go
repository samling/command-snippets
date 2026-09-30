package search

import (
	"sort"
	"strings"
	"unicode"

	"github.com/samling/command-snippets/internal/library"
	"github.com/samling/command-snippets/internal/models"
)

type Category struct {
	Key, Label string
	Count      int
}
type document struct {
	entry                       *library.Entry
	title, description, command string
	tags                        []string
	source                      int
	ordinal                     int
}
type Index struct {
	documents  []document
	Categories []Category
}
type Match struct {
	Entry *library.Entry
	Score int
}

func New(entries []*library.Entry) *Index {
	index := &Index{}
	categories := map[string]*Category{}
	sourceOrder := map[string]int{}
	for _, entry := range entries {
		path := entry.Source.Path
		if _, ok := sourceOrder[path]; !ok {
			sourceOrder[path] = len(sourceOrder)
		}
		doc := document{entry: entry, title: strings.ToLower(entry.Snippet.Name), description: strings.ToLower(entry.Snippet.Description), command: strings.ToLower(entry.Snippet.Command), source: sourceOrder[path], ordinal: entry.Ordinal}
		for _, tag := range entry.Snippet.Tags {
			key := models.TagKey(tag)
			doc.tags = append(doc.tags, key)
			category := categories[key]
			if category == nil {
				category = &Category{Key: key, Label: tag}
				categories[key] = category
			}
			category.Count++
		}
		index.documents = append(index.documents, doc)
	}
	for _, category := range categories {
		index.Categories = append(index.Categories, *category)
	}
	sort.Slice(index.Categories, func(i, j int) bool { return index.Categories[i].Key < index.Categories[j].Key })
	return index
}
func (i *Index) Query(query string, tags []string, untagged bool) []Match {
	tokens := strings.Fields(strings.ToLower(query))
	keys := map[string]bool{}
	for _, tag := range tags {
		keys[models.TagKey(tag)] = true
	}
	matches := make([]Match, 0, len(i.documents))
	docs := map[*library.Entry]document{}
	for _, doc := range i.documents {
		if untagged && len(doc.tags) > 0 {
			continue
		}
		matchesTags := true
		for key := range keys {
			found := false
			for _, tag := range doc.tags {
				if tag == key {
					found = true
					break
				}
			}
			if !found {
				matchesTags = false
				break
			}
		}
		if !matchesTags {
			continue
		}
		score := 0
		valid := true
		for _, token := range tokens {
			best := titleScore(doc.title, token)
			if best < 120 {
				best = max(best, fieldScore(doc.description, token, 120, 90))
			}
			if best < 100 {
				for _, tag := range doc.tags {
					best = max(best, fieldScore(tag, token, 100, 80))
				}
			}
			if best < 60 {
				best = max(best, fieldScore(doc.command, token, 60, 40))
			}
			if best == 0 {
				valid = false
				break
			}
			score += best
		}
		if valid {
			matches = append(matches, Match{doc.entry, score})
			docs[doc.entry] = doc
		}
	}
	sort.SliceStable(matches, func(a, b int) bool {
		left, right := matches[a], matches[b]
		if left.Score != right.Score {
			return left.Score > right.Score
		}
		ld, rd := docs[left.Entry], docs[right.Entry]
		if ld.title != rd.title {
			return ld.title < rd.title
		}
		if ld.source != rd.source {
			return ld.source < rd.source
		}
		if ld.ordinal != rd.ordinal {
			return ld.ordinal < rd.ordinal
		}
		return left.Entry.Key < right.Entry.Key
	})
	return matches
}
func fieldScore(field, query string, substring, fuzzy int) int {
	if strings.Contains(field, query) {
		return substring
	}
	if titleScore(field, query) > 0 {
		return fuzzy
	}
	return 0
}
func titleScore(title, query string) int {
	if title == query {
		return 1000
	}
	if strings.HasPrefix(title, query) {
		return 800
	}
	if strings.Contains(title, query) {
		return 600
	}
	// Rune-aware subsequence matching rewards boundaries and contiguous runs.
	needles := []rune(query)
	if len(needles) == 0 {
		return 0
	}
	position := 0
	bonus := 0
	previousMatch := false
	previous := rune(' ')
	for _, r := range title {
		matched := position < len(needles) && r == needles[position]
		if matched {
			position++
			if previousMatch {
				bonus += 3
			}
			if unicode.IsSpace(previous) || strings.ContainsRune("-/_.", previous) {
				bonus += 5
			}
		}
		previousMatch = matched
		previous = r
	}
	if position == len(needles) {
		return 200 + min(bonus, 150)
	}
	return 0
}
