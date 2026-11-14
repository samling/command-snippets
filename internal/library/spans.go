package library

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type byteSpan struct{ start, end int }

func sourceLines(data []byte) ([]string, []int) {
	lines := strings.SplitAfter(string(data), "\n")
	offsets := make([]int, len(lines)+1)
	for i, line := range lines {
		offsets[i+1] = offsets[i] + len(line)
	}
	return lines, offsets
}
func indentation(line string) int { return len(line) - len(strings.TrimLeft(line, " ")) }
func trimSeparators(lines []string, start, end int, indent int) int {
	for end > start {
		line := strings.TrimSuffix(strings.TrimSuffix(lines[end-1], "\n"), "\r")
		if strings.TrimSpace(line) == "" || (indentation(line) <= indent && strings.HasPrefix(strings.TrimSpace(line), "#")) {
			end--
		} else {
			break
		}
	}
	return end
}

// Keep-chomp blank lines belong to the scalar, not to entry separators.
// YAML coordinates identify the header; indentation identifies its true extent.
func keepScalarEnd(node *yaml.Node, lines []string, boundary int) int {
	end := 0
	if node.Kind == yaml.ScalarNode && node.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 && node.Line > 0 && node.Line <= len(lines) {
		header := lines[node.Line-1]
		indicator := header[node.Column-1:]
		indicator, _, _ = strings.Cut(indicator, "#")
		if strings.Contains(indicator, "+") {
			base := indentation(header)
			if strings.HasPrefix(strings.TrimLeft(header, " "), "- ") {
				base += 2
			}
			contentIndent := base + 1
			indentKnown := false
			for _, r := range indicator {
				if r >= '1' && r <= '9' {
					contentIndent = base + int(r-'0')
					indentKnown = true
					break
				}
			}
			for i := node.Line; i < boundary; i++ {
				if strings.TrimSpace(lines[i]) != "" {
					if indentation(lines[i]) < contentIndent {
						break
					}
					// Without an explicit indicator, the first content line sets indentation.
					if !indentKnown {
						contentIndent = indentation(lines[i])
						indentKnown = true
					}
				}
				end = i + 1
			}
		}
	}
	for _, child := range node.Content {
		end = max(end, keepScalarEnd(child, lines, boundary))
	}
	return end
}
func documentEnd(lines []string, start, end int) int {
	for i := start; i < end; i++ {
		line := strings.TrimRight(lines[i], "\r\n")
		if line == "..." || strings.HasPrefix(line, "... ") || strings.HasPrefix(line, "...\t") {
			return i
		}
	}
	return end
}
func snippetSpan(source *Source, ordinal int) (byteSpan, error) {
	sequence := mapValue(source.Root, "snippets")
	if sequence == nil || sequence.Style&yaml.FlowStyle != 0 || ordinal < 0 || ordinal >= len(sequence.Content) {
		return byteSpan{}, fmt.Errorf("cannot identify a safe block snippet span")
	}
	lines, offsets := sourceLines(source.Bytes)
	start := sequence.Content[ordinal].Line - 1
	if start < 0 || start >= len(lines) {
		return byteSpan{}, fmt.Errorf("invalid source coordinates")
	}
	indent := sequence.Column - 1
	line := strings.TrimLeft(lines[start], " ")
	if !strings.HasPrefix(line, "- ") && strings.TrimSpace(line) != "-" {
		return byteSpan{}, fmt.Errorf("line %d: unsupported snippet layout", start+1)
	}
	end := len(lines)
	if ordinal+1 < len(sequence.Content) {
		end = sequence.Content[ordinal+1].Line - 1
	} else {
		for i := start + 1; i < len(lines); i++ {
			trim := strings.TrimSpace(lines[i])
			if trim == "" || strings.HasPrefix(trim, "#") {
				continue
			}
			if indentation(lines[i]) <= indent && !strings.HasPrefix(trim, "- ") {
				end = i
				break
			}
		}
	}
	end = documentEnd(lines, start, end)
	end = max(trimSeparators(lines, start, end, indent), keepScalarEnd(sequence.Content[ordinal], lines, end))
	if end <= start {
		return byteSpan{}, fmt.Errorf("ambiguous snippet boundary")
	}
	return byteSpan{offsets[start], offsets[end]}, nil
}
func fieldSpan(source *Source, key string) (byteSpan, error) {
	root := source.Root.Content[0]
	lines, offsets := sourceLines(source.Bytes)
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value != key {
			continue
		}
		start := root.Content[i].Line - 1
		end := len(lines)
		if i+2 < len(root.Content) {
			end = root.Content[i+2].Line - 1
		}
		end = documentEnd(lines, start, end)
		end = trimSeparators(lines, start, end, root.Content[i].Column-1)
		if start < 0 || end <= start || end >= len(offsets) {
			return byteSpan{}, fmt.Errorf("ambiguous %s source boundary", key)
		}
		startOffset := offsets[start]
		if startOffset == 0 && bytes.HasPrefix(source.Bytes, []byte{0xef, 0xbb, 0xbf}) {
			startOffset = 3
		}
		return byteSpan{startOffset, offsets[end]}, nil
	}
	return byteSpan{}, fmt.Errorf("field %s not present", key)
}
func encodeNode(node *yaml.Node, indent int, newline string) ([]byte, error) {
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	raw := b.String()
	prefix := strings.Repeat(" ", indent)
	lines := strings.SplitAfter(raw, "\n")
	var out strings.Builder
	for _, line := range lines {
		if line == "" {
			continue
		}
		out.WriteString(prefix)
		out.WriteString(line)
	}
	if newline == "\r\n" {
		return []byte(strings.ReplaceAll(out.String(), "\n", "\r\n")), nil
	}
	return []byte(out.String()), nil
}
func newlineFor(source *Source) string {
	if bytes.Contains(source.Bytes, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}
func retainedComment(comment string, original []byte) string {
	result := []string{}
	for _, line := range strings.Split(comment, "\n") {
		if line != "" && bytes.Contains(original, []byte(line)) {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}
func transferComments(old, new *yaml.Node, source *Source, span byteSpan) {
	original := source.Bytes[span.start:span.end]
	removed := []*yaml.Node{}
	transferNodeComments(old, new, original, &removed)
	lines, _ := sourceLines(original)
	firstLine := bytes.Count(source.Bytes[:span.start], []byte("\n"))
	comments := map[int]string{}
	for _, node := range removed {
		collectRemovedComments(node, lines, firstLine, comments)
	}
	// Physical occurrences, not comment text, determine ownership and order.
	// Identical notes on surviving fields must not be copied again.
	for i := range lines {
		if comment := comments[i]; comment != "" {
			if new.FootComment != "" {
				new.FootComment += "\n"
			}
			new.FootComment += comment
		}
	}
}
func lastNodeLine(node *yaml.Node) int {
	line := node.Line
	for _, child := range node.Content {
		line = max(line, lastNodeLine(child))
	}
	return line
}
func collectRemovedComments(node *yaml.Node, lines []string, firstLine int, comments map[int]string) int {
	lastLine := node.Line - firstLine - 1
	for _, child := range node.Content {
		lastLine = max(lastLine, collectRemovedComments(child, lines, firstLine, comments))
	}
	collect := func(comment string, start, step int) {
		parts := strings.Split(comment, "\n")
		if step < 0 {
			for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
				parts[i], parts[j] = parts[j], parts[i]
			}
		}
		for _, part := range parts {
			if part == "" {
				continue
			}
			for start >= 0 && start < len(lines) {
				i := start
				start += step
				if strings.TrimSpace(lines[i]) == part {
					// A footer cannot be a comment-shaped line in the last scalar's body.
					if step > 0 && node.FootComment == comment && indentation(lines[i]) > indentation(lines[max(0, lastNodeLine(node)-firstLine-1)]) {
						continue
					}
					comments[i] = part
					lastLine = max(lastLine, i)
					break
				}
			}
		}
	}
	collect(node.HeadComment, node.Line-firstLine-2, -1)
	line := node.Line - firstLine - 1
	if node.LineComment != "" && line >= 0 && line < len(lines) && strings.HasSuffix(strings.TrimRight(lines[line], "\r\n"), node.LineComment) {
		comments[line] = node.LineComment
	}
	collect(node.FootComment, lastLine+1, 1)
	return lastLine
}
func transferNodeComments(old, new *yaml.Node, original []byte, removed *[]*yaml.Node) {
	if old == nil || new == nil {
		return
	}
	new.HeadComment = retainedComment(old.HeadComment, original)
	new.LineComment = retainedComment(old.LineComment, original)
	new.FootComment = retainedComment(old.FootComment, original)
	if old.Kind == yaml.MappingNode && new.Kind == yaml.MappingNode {
		oldFields := map[string][2]*yaml.Node{}
		for i := 0; i+1 < len(old.Content); i += 2 {
			oldFields[old.Content[i].Value] = [2]*yaml.Node{old.Content[i], old.Content[i+1]}
		}
		for i := 0; i+1 < len(new.Content); i += 2 {
			if pair, exists := oldFields[new.Content[i].Value]; exists {
				transferNodeComments(pair[0], new.Content[i], original, removed)
				transferNodeComments(pair[1], new.Content[i+1], original, removed)
				delete(oldFields, new.Content[i].Value)
			}
		}
		for _, pair := range oldFields {
			for _, node := range pair {
				*removed = append(*removed, node)
			}
		}
	} else if old.Kind == yaml.SequenceNode && new.Kind == yaml.SequenceNode {
		matched := map[*yaml.Node]bool{}
		for i, node := range new.Content {
			var match *yaml.Node
			name := mapValue(node, "name")
			if name != nil {
				for _, candidate := range old.Content {
					oldName := mapValue(candidate, "name")
					if oldName != nil && oldName.Value == name.Value {
						match = candidate
						break
					}
				}
			} else if i < len(old.Content) {
				match = old.Content[i]
			}
			transferNodeComments(match, node, original, removed)
			matched[match] = true
		}
		for _, node := range old.Content {
			if !matched[node] {
				*removed = append(*removed, node)
			}
		}
	}
}
func splice(data []byte, span byteSpan, replacement []byte) []byte {
	result := make([]byte, 0, len(data)-(span.end-span.start)+len(replacement))
	result = append(result, data[:span.start]...)
	result = append(result, replacement...)
	return append(result, data[span.end:]...)
}
func entryBytes(source *Source, ordinal int, value any) ([]byte, error) {
	var node yaml.Node
	if err := node.Encode(value); err != nil {
		return nil, err
	}
	sequence := mapValue(source.Root, "snippets")
	if ordinal >= 0 {
		span, err := snippetSpan(source, ordinal)
		if err != nil {
			return nil, err
		}
		transferComments(source.Nodes[ordinal], &node, source, span)
		container := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{&node}}
		encoded, err := encodeNode(container, sequence.Column-1, newlineFor(source))
		if err != nil {
			return nil, err
		}
		return splice(source.Bytes, span, encoded), nil
	}
	if sequence == nil {
		encoded, err := encodeNode(&yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{{Kind: yaml.ScalarNode, Tag: "!!str", Value: "snippets"}, {Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{&node}}}}, 0, newlineFor(source))
		if err != nil {
			return nil, err
		}
		body := append([]byte{}, source.Bytes...)
		if len(body) > 0 && body[len(body)-1] != '\n' {
			body = append(body, []byte(newlineFor(source))...)
		}
		return append(body, encoded...), nil
	}
	if len(sequence.Content) == 0 {
		span, err := fieldSpan(source, "snippets")
		if err != nil {
			return nil, err
		}
		mapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{{Kind: yaml.ScalarNode, Tag: "!!str", Value: "snippets"}, {Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{&node}}}}
		transferComments(sequence, mapping.Content[1], source, span)
		mapping.Content[0].LineComment = mapping.Content[1].LineComment
		mapping.Content[1].LineComment = ""
		encoded, err := encodeNode(mapping, 0, newlineFor(source))
		if err != nil {
			return nil, err
		}
		return splice(source.Bytes, span, encoded), nil
	}
	span, err := snippetSpan(source, len(sequence.Content)-1)
	if err != nil {
		return nil, err
	}
	encoded, err := encodeNode(&yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: []*yaml.Node{&node}}, sequence.Column-1, newlineFor(source))
	if err != nil {
		return nil, err
	}
	if span.end > 0 && source.Bytes[span.end-1] != '\n' {
		encoded = append([]byte(newlineFor(source)), encoded...)
	}
	return splice(source.Bytes, byteSpan{span.end, span.end}, encoded), nil
}
func settingsBytes(source *Source, value any) ([]byte, error) {
	var node yaml.Node
	if err := node.Encode(value); err != nil {
		return nil, err
	}
	span, err := fieldSpan(source, "settings")
	if err == nil {
		transferComments(mapValue(source.Root, "settings"), &node, source, span)
	}
	mapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{{Kind: yaml.ScalarNode, Tag: "!!str", Value: "settings"}, &node}}
	mapping.Content[0].LineComment = node.LineComment
	node.LineComment = ""
	encoded, encodeErr := encodeNode(mapping, 0, newlineFor(source))
	if encodeErr != nil {
		return nil, encodeErr
	}
	if err == nil {
		return splice(source.Bytes, span, encoded), nil
	}
	body := append([]byte{}, source.Bytes...)
	if len(body) > 0 && body[len(body)-1] != '\n' {
		body = append(body, []byte(newlineFor(source))...)
	}
	return append(body, encoded...), nil
}
