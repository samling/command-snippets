package library

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/samling/command-snippets/internal/models"
	"github.com/samling/command-snippets/internal/templating"
	"gopkg.in/yaml.v3"
)

type Source struct {
	Path        string
	DisplayPath string
	Kind        string
	Bytes       []byte
	Root        *yaml.Node
	Config      models.Config
	Nodes       []*yaml.Node
	Info        os.FileInfo
	Hash        [32]byte
	Symlink     bool
}
type Entry struct {
	Key      string
	Snippet  models.Snippet
	Source   *Source
	Ordinal  int
	Line     int
	Template *templating.Template
}
type Library struct {
	ConfigPath  string
	CWD         string
	Settings    models.Settings
	Sources     []*Source
	Entries     []*Entry
	Diagnostics []string
	Warnings    []string
	saveHook    func(stage, path string) error
}

func (l *Library) Error() error {
	if len(l.Diagnostics) == 0 {
		return nil
	}
	return errors.New(strings.Join(l.Diagnostics, "\n"))
}
func (l *Library) Find(name, id string) (*Entry, error) {
	if name != "" && id != "" {
		return nil, fmt.Errorf("NAME and --id are mutually exclusive")
	}
	if id != "" && !models.ValidID(id) {
		return nil, fmt.Errorf("--id must be a persisted UUID v4")
	}
	var matches []*Entry
	for _, e := range l.Entries {
		if (id != "" && e.Snippet.ID == id) || (id == "" && e.Snippet.Name == strings.TrimSpace(name)) {
			matches = append(matches, e)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("command not found: %s%s", name, id)
	}
	paths := []string{}
	for _, e := range matches {
		paths = append(paths, fmt.Sprintf("%s:%d", e.Source.DisplayPath, e.Line))
	}
	return nil, fmt.Errorf("name %q is ambiguous (%s); use the workspace or --id", name, strings.Join(paths, ", "))
}
func mapValue(n *yaml.Node, key string) *yaml.Node {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		n = n.Content[0]
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}
func checkNodes(n *yaml.Node) error {
	if n.Anchor != "" || n.Kind == yaml.AliasNode {
		return fmt.Errorf("line %d: anchors and aliases are not supported", n.Line)
	}
	if n.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			key := n.Content[i]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
				return fmt.Errorf("line %d: mapping keys must be strings", key.Line)
			}
			if seen[key.Value] {
				return fmt.Errorf("line %d: duplicate key %q", key.Line, key.Value)
			}
			seen[key.Value] = true
		}
	}
	switch n.Tag {
	case "", "!!map", "!!seq", "!!str", "!!int", "!!bool", "!!null", "!!float":
	default:
		return fmt.Errorf("line %d: unsupported YAML tag %s", n.Line, n.Tag)
	}
	for _, child := range n.Content {
		if err := checkNodes(child); err != nil {
			return err
		}
	}
	return nil
}
func parseSource(path, kind string, data []byte, info os.FileInfo) (*Source, error) {
	source := &Source{Path: path, DisplayPath: path, Kind: kind, Bytes: data, Info: info, Hash: sha256.Sum256(data)}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var root yaml.Node
	if err := dec.Decode(&root); err != nil {
		if err != io.EOF {
			return source, err
		}
		root = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return source, fmt.Errorf("only one YAML document is supported")
		}
		return source, err
	}
	if len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return source, fmt.Errorf("root must be a mapping; example: snippets: [{name: List pods, command: kubectl get pods}]")
	}
	if err := checkNodes(&root); err != nil {
		return source, err
	}
	if root.Content[0].Style&yaml.FlowStyle != 0 {
		return source, fmt.Errorf("use a block-style root mapping for safe source-local editing")
	}
	if err := validateDocumentTypes(&root); err != nil {
		return source, err
	}
	source.Root = &root
	if kind != "main" && mapValue(&root, "settings") != nil {
		return source, fmt.Errorf("included/project sources may contain only snippets, not settings")
	}
	seq := mapValue(&root, "snippets")
	if seq != nil {
		if seq.Kind != yaml.SequenceNode {
			return source, fmt.Errorf("line %d: snippets must be a sequence, not legacy keyed entries; example: snippets: [{name: List pods, command: kubectl get pods}]", seq.Line)
		}
		if len(seq.Content) > 0 && seq.Style&yaml.FlowStyle != 0 {
			return source, fmt.Errorf("line %d: use block-style snippets for safe source-local editing", seq.Line)
		}
		for _, node := range seq.Content {
			if node.Kind != yaml.MappingNode || node.Style&yaml.FlowStyle != 0 {
				return source, fmt.Errorf("line %d: snippets must use block mappings", node.Line)
			}
		}
		source.Nodes = seq.Content
	}
	cfg := models.Config{Settings: models.DefaultSettings()}
	if err := root.Decode(&cfg); err != nil {
		return source, err
	}
	if kind == "main" {
		if err := cfg.Settings.Validate(); err != nil {
			return source, err
		}
	}
	for i := range cfg.Snippets {
		if err := cfg.Snippets[i].Validate(); err != nil {
			return source, fmt.Errorf("line %d: %w", seq.Content[i].Line, err)
		}
	}
	source.Config = cfg
	return source, nil
}
