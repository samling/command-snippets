# CS - Command Snippets

CS is a terminal command library. Search friendly names, browse tag categories, fill inputs with a live preview, and insert the finished command into your shell. CS does not execute commands unless you explicitly use `exec --run` or `exec --prompt`.

**Incompatible format change:** libraries now use a YAML sequence with `name`, `command`, tags and optional inputs. Legacy keyed snippets, variables, transforms, computed blocks, selectors and `${...}` interpolation are no longer supported. CS reports invalid files without rewriting them. Existing installed files are not replaced automatically; see [the authoring guide](SNIPPET_GUIDE.md) for the new format.

## Install and start

```sh
go install github.com/samling/command-snippets/cmd/cs@latest
cs init
cs
```

Build from source with `go build -o /tmp/cs ./cmd/cs`, or use the repository's `make install` when installation is intended. Go 1.24 or later is required.

`cs init` creates `$XDG_CONFIG_HOME/cs/config.yaml`, or `~/.config/cs/config.yaml`, and copies editable examples into the adjacent `snippets/` directory. Existing files are skipped. `cs init --missing` restores absent examples; `cs init --force` explicitly overwrites defaults. These options do not convert old libraries.

The smallest library is:

```yaml
snippets:
  - name: List pods
    tags: [Kubernetes]
    command: kubectl get pods
```

## Find, fill, insert

Search starts focused. Names, descriptions, tags and literal commands are searchable. Multiple selected tags intersect; All clears category filters and Untagged finds uncategorized commands. Duplicate names remain separate entries distinguished by source. Tags are trimmed and case-folded for matching.

| Key | Library action |
| --- | --- |
| Tab / Shift-Tab | Switch search, categories, results and detail |
| Up / Down | Select a command, or a category in its pane |
| Enter | Fill the selected command's inputs, then confirm insertion |
| Ctrl-L / Ctrl-R | Clear query and filters / reload files |
| Ctrl-N / Ctrl-E / Ctrl-O | New command / edit selected command / settings |
| Ctrl-D | Toggle detail on a medium-width terminal |
| F1 | Contextual help |
| Esc / Ctrl-C | Cancel with no command output |

In the input view, arrows cycle choices, Tab navigates, and Enter advances or submits the last field; Ctrl-S submits from any input field. Esc returns to the previous search and filters. Required and invalid inputs block insertion. Repeat inputs have separate item rows; spaces within an item are retained. No preview executes a command.

In the editor, Ctrl-Left/Right switches Basics, Inputs, Advanced and Test. Alt-Enter inserts a command newline. Ctrl-P marks a start and end cursor position and opens guided input controls. Ctrl-S validates and saves; dirty cancellation asks before discarding. Settings controls manage source order, project discovery, destination and color.

## CLI

```sh
cs add
cs edit 'List pods'
cs list --query pods --tag Kubernetes
cs list --json
cs describe 'List pods'
cs validate
cs render 'Pod resource usage, sorted' --set namespace=all --set sort_by=Memory
cs exec 'List pods' --set namespace=team-a
```

`render` never opens a terminal or executes anything. It applies defaults and the same validation as the preview. `exec` opens input entry when values are incomplete; `--run` and `--prompt` are explicit, mutually exclusive execution modes. NAME lookup is exact and case-sensitive. Ambiguous names require workspace selection or `--id UUID`. `list --json` and `describe` expose persisted IDs for automation; ordinary panes hide them. ID-less definitions are valid, and saving one assigns an ID only to that entry. Renaming retains its ID.

Use `--config PATH` for another main file and `--no-color` to override color. Interactive paths require a terminal on stderr; their input comes from the controlling terminal even when stdout is captured. UI, diagnostics and save messages go to stderr. Cancellation exits 0 with empty stdout; fatal errors exit nonzero without a partial command. Help, list, describe and config generation intentionally print their requested data.

## Ctrl-S shell integration

Source [zshrc-snippet.sh](zshrc-snippet.sh) from Zsh or [bashrc-snippet.sh](bashrc-snippet.sh) from Bash. Both release Ctrl-S with `stty -ixon`, call bare `cs`, check its exit status separately, and insert only a nonempty successful result. Neither executes the result or presses Enter.

Zsh appends to `LBUFFER`, retaining text to the right of the cursor, and redisplays after success, failure or cancellation. Bash splices at `READLINE_POINT`. The Bash example uses character offsets; Readline/Bash versions reporting byte offsets in multibyte locales need an adapted binding. Command substitution strips trailing newlines. Zsh is recommended for Unicode-heavy command lines.

## Files and safe saves

The main configuration owns settings; included files and current-directory `.csnippets` own only snippets. No parent-directory project search occurs. See [SNIPPET_GUIDE.md](SNIPPET_GUIDE.md) for source settings and input syntax.

Edits save only the owning source. Unrelated snippets and surrounding bytes remain unchanged. Unsupported YAML layouts fail rather than trigger a lossy rewrite. Save conflicts keep the draft. Symlink sources are readable but read-only. A sibling `.cs.lock` coordinates CS writers; inspect the owner before manually removing a crash-stale lock. CS checks content, identity and permissions before replacement, but an external editor can still race between the final check and rename. A directory-sync error means bytes were saved but durability is uncertain; CS reloads instead of blindly retrying.

Commands, choice outputs, special mappings and expressions are trusted authored shell syntax. CS quotes normal text, flag values and repeated values, but it does not make an authored command safe to execute.

[Testing and scratch verification](TESTING.md)
