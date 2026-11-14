package library

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/samling/command-snippets/internal/templating"
)

func ExpandPath(path, base string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[2:])
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	p, err := filepath.Abs(path)
	if err == nil {
		return filepath.Clean(p)
	}
	return filepath.Clean(path)
}

// Load retains a recovery-capable snapshot even when one source is invalid.
// It never writes IDs, config, or snippets as a side effect of reading.
func Load(configPath, cwd string) *Library {
	return load(configPath, cwd, nil)
}

func load(configPath, cwd string, overrides map[string][]byte) *Library {
	configPath = ExpandPath(configPath, cwd)
	lib := &Library{ConfigPath: configPath, CWD: cwd}
	seen := map[string]bool{}
	add := func(path, kind string) *Source {
		display := path
		canonical, err := filepath.EvalSymlinks(path)
		if _, virtual := overrides[path]; virtual && os.IsNotExist(err) {
			canonical, err = path, nil
		}
		if err != nil {
			lib.Diagnostics = append(lib.Diagnostics, fmt.Sprintf("%s: %v", path, err))
			return nil
		}
		canonical = ExpandPath(canonical, cwd)
		if seen[canonical] {
			return nil
		}
		seen[canonical] = true
		data, virtual := overrides[canonical]
		info, err := os.Stat(canonical)
		if virtual && os.IsNotExist(err) {
			err = nil
		}
		if err != nil {
			lib.Diagnostics = append(lib.Diagnostics, fmt.Sprintf("%s: %v", path, err))
			return nil
		}
		if info != nil && !info.Mode().IsRegular() {
			lib.Diagnostics = append(lib.Diagnostics, fmt.Sprintf("%s: source is not a regular file", path))
			return nil
		}
		if !virtual {
			data, err = os.ReadFile(canonical)
		}
		if err != nil {
			lib.Diagnostics = append(lib.Diagnostics, fmt.Sprintf("%s: %v", path, err))
			return nil
		}
		source, err := parseSource(canonical, kind, data, info)
		if source != nil {
			source.DisplayPath = display
			source.Symlink = canonical != display
			lib.Sources = append(lib.Sources, source)
		}
		if err != nil {
			lib.Diagnostics = append(lib.Diagnostics, fmt.Sprintf("%s: %v", display, err))
			return source
		}
		for i, snippet := range source.Config.Snippets {
			compiled, err := templating.Compile(snippet)
			if err != nil {
				lib.Diagnostics = append(lib.Diagnostics, fmt.Sprintf("%s:%d: %v", display, source.Nodes[i].Line, err))
				continue
			}
			key := snippet.ID
			if key == "" {
				key = fmt.Sprintf("%s#%d", canonical, i)
			}
			line := source.Nodes[i].Line
			lib.Entries = append(lib.Entries, &Entry{Key: key, Snippet: snippet, Source: source, Ordinal: i, Line: line, Template: compiled})
		}
		return source
	}
	main := add(configPath, "main")
	if main == nil || main.Root == nil {
		return lib
	}
	lib.Settings = main.Config.Settings
	if main.Kind == "main" && lib.Settings.Color == "" {
		return lib
	} // invalid main settings stay in recovery
	base := filepath.Dir(configPath)
	for _, pattern := range lib.Settings.Sources {
		expanded := ExpandPath(pattern, base)
		matches, err := filepath.Glob(expanded)
		if err != nil {
			lib.Diagnostics = append(lib.Diagnostics, fmt.Sprintf("%s: invalid source pattern %q: %v", configPath, pattern, err))
			continue
		}
		for path := range overrides {
			if matched, _ := filepath.Match(expanded, path); matched {
				found := false
				for _, existing := range matches {
					if existing == path {
						found = true
					}
				}
				if !found {
					matches = append(matches, path)
				}
			}
		}
		if len(matches) == 0 {
			if strings.ContainsAny(pattern, "*?[") {
				lib.Warnings = append(lib.Warnings, fmt.Sprintf("source pattern %q has no matches", pattern))
				continue
			}
			matches = []string{expanded}
		}
		sort.Strings(matches)
		for _, path := range matches {
			add(path, "included")
		}
	}
	if lib.Settings.ProjectSource {
		local := filepath.Join(cwd, ".csnippets")
		_, virtual := overrides[local]
		if _, err := os.Lstat(local); err == nil || virtual {
			add(local, "project")
		} else if !os.IsNotExist(err) {
			lib.Diagnostics = append(lib.Diagnostics, fmt.Sprintf("%s: %v", local, err))
		}
	}
	ids := map[string]*Entry{}
	for _, entry := range lib.Entries {
		id := entry.Snippet.ID
		if id == "" {
			continue
		}
		if first := ids[id]; first != nil {
			lib.Diagnostics = append(lib.Diagnostics, fmt.Sprintf("duplicate id %s at %s:%d and %s:%d", id, first.Source.DisplayPath, first.Line, entry.Source.DisplayPath, entry.Line))
		} else {
			ids[id] = entry
		}
	}
	return lib
}

func (l *Library) Source(path string) *Source {
	path = ExpandPath(path, filepath.Dir(l.ConfigPath))
	for _, s := range l.Sources {
		if s.Path == path || s.DisplayPath == path {
			return s
		}
	}
	return nil
}
func (l *Library) AllowedDestination(path string) bool {
	path = ExpandPath(path, filepath.Dir(l.ConfigPath))
	if l.Source(path) != nil || path == l.ConfigPath {
		return true
	}
	if l.Settings.ProjectSource && path == filepath.Join(l.CWD, ".csnippets") {
		return true
	}
	for _, pattern := range l.Settings.Sources {
		matched, err := filepath.Match(ExpandPath(pattern, filepath.Dir(l.ConfigPath)), path)
		if err == nil && matched {
			return true
		}
	}
	return false
}
func (l *Library) Destinations() []string {
	result := []string{}
	seen := map[string]bool{}
	appendPath := func(path string) {
		if !seen[path] {
			seen[path] = true
			result = append(result, path)
		}
	}
	for _, s := range l.Sources {
		appendPath(s.DisplayPath)
	}
	target := l.Settings.DefaultSource
	if target == "" {
		target = l.ConfigPath
	}
	target = ExpandPath(target, filepath.Dir(l.ConfigPath))
	if l.AllowedDestination(target) {
		appendPath(target)
	}
	if l.Settings.ProjectSource {
		appendPath(filepath.Join(l.CWD, ".csnippets"))
	}
	return result
}
