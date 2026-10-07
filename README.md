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
| / | Jump to search from any pane |
| Up / Down | Select a command, or a category in its pane; scroll the focused preview |
| PgUp / PgDn | Page the focused preview |
| Enter | Fill the selected command's inputs, then confirm insertion |
| Ctrl-L / Ctrl-R | Clear query and filters / reload files |
| F2 (or Ctrl-N) / Ctrl-E / Ctrl-O | New command / edit selected command / settings |
| Ctrl-D | Toggle detail on a medium-width terminal |
| F1 | Contextual help |
| Esc / Ctrl-C | Cancel with no command output |

The input form frames **Inputs** and **Live preview** side by side on wide terminals and stacks them when narrow. Each field is one aligned row with its value in an input well; the focused row has an accent bar, and its description and any validation error appear in a hint at the bottom of the Inputs panel. An empty required field simply reads `required` in its well — no asterisks or repeated error text. The live preview names the inputs a missing part still needs (`awk ‹pattern› ‹logfile›`), even when the command uses an expression such as `{{awk_script}}`, and underlines the part of the command the focused field controls. Pressing Enter on the last field or Ctrl-S on an incomplete form moves focus to the first field that needs attention. Small terminals prioritize the focused value and keyboard actions over frames. The input form keeps the original workflow: Tab or Up/Down navigates fields, Left/Right cycles choices or moves the text cursor, and Enter advances or submits the last field. The default theme keeps magenta focused fields and cyan selected choices; text uses a block cursor. Named themes also color the input form. Ctrl-S remains an optional submit alias, not a required step. Esc returns to the previous search and filters. Required and invalid inputs block insertion. Repeat inputs have separate item rows; spaces within an item are retained. No preview executes a command.

The framed editor keeps command details and input configuration in one workspace. Tab/Shift-Tab or Up/Down move between controls (in a multiline Command, Up/Down move between lines first); Left/Right move the cursor or change a choice. Enter on Configure inputs, Computed values or Test preview opens and focuses that pane; Shift-Tab or Up from the pane's first row returns to the action that opened it. The Inputs pane lists each input with its type, flag and requiredness; Enter opens its details, and Esc or Back to inputs returns to the list. Type `ls -lah {{folder_name}}` to create a required text input automatically. Repeated tokens reuse a definition; removing a token keeps its configuration.

**Tags** opens a filtering checklist of every tag already in your library, with counts. Type to filter, Up/Down to pick, Enter to toggle a tag on or off, or Enter on `+ create “word”` to add a new one. Backspace only edits what you typed; untick a tag with Enter. Tab or Esc leaves. Chosen tags show as `a · b` and are stored as a YAML sequence. Merely visiting the field never edits it, so untouched multiword tags are preserved.

**Computed values** (`expressions:` in YAML) build part of a command from inputs when a plain `{{input}}` isn't enough — for example `awk_script: quote("/" + inputs.pattern + "/ {print $0}")` placed with `{{awk_script}}`. Each value has a Name and a Formula; beneath it the pane shows which inputs it uses and its current result from the Test preview values, or a reminder to add `{{name}}` to Command. `+ Add computed value` starts a fresh name with a starter formula and focuses the formula. Formulas read `inputs.<name>`, join text with `+`, choose with `a ? b : c`, and use the helpers `quote`, `flag`, `boolFlag`, `repeatFlag`, `join`, `default` and `empty`; see SNIPPET_GUIDE.md.

Alt-Enter inserts a command newline. Ctrl-S validates and saves without emitting a command; dirty cancellation asks before discarding. New commands use the configured default source, shown as a read-only **Saved to** label; edits retain their owning file.

**Shell use.** Ctrl-S (the zsh widget) inserts the finished command at your prompt. Typing `cs` or `cs exec [NAME] [--set ...]` and pressing Enter does the same with the dotfiles' accept-line hook: the finished command replaces that line, ready to edit or run, and `cs exec` is not added to history. Pipes and captures (`cs exec | pbcopy`, `$(cs exec)`) print the command; `--run` and `--prompt` execute it explicitly.

## Themes

Settings offers `default` and `catppuccin-mocha`. Omitting `theme` (or leaving it empty) preserves the original mixed library/input-form palette. Named themes cover search, panes and selection surfaces, inputs, editor, Settings, help and recovery. Custom colors belong to the main YAML config and are preserved when changing presets in Settings:

```yaml
settings:
  sources: [snippets/current/*.yaml]
  project_source: false
  default_source: snippets/current/custom.yaml
  color: auto
  theme: catppuccin-mocha
  theme_colors:
    focus: '#cba6f7'
    preview: '#89b4fa'
    background: transparent
    panel: transparent
snippets: []
```

Supported `theme_colors` roles: `text`, `muted`, `focus`, `preview`, `filled`, `unfilled`, `error`, `border`, `selection`, `inactive_selection`, `background`, `panel`. Colors must be quoted `#RRGGBB` values, except `background` and `panel`, which also accept `transparent` to show the terminal background without changing foregrounds or selection highlights. Transparency is opt-in: omitted overrides keep each theme's existing surfaces (Mocha stays opaque). Settings exposes separate **Transparent canvas** and **Transparent panels** toggles; turning them off uses the preset or existing custom opaque override. A no-op save preserves custom colors. Unknown presets, roles or malformed colors enter the existing configuration recovery path. Overrides apply over the chosen preset, including `default`; they are edited in YAML, not a separate theme file. The Mocha colors follow the [official Catppuccin palette](https://catppuccin.com/palette) ([Mocha](https://catppucc.in/mocha/)).

`color: auto` detects stderr's terminal; `always` forces colors, and `never` disables them. `--no-color` and a nonempty `NO_COLOR` take precedence over presets and `always`. Focus markers and a text cursor remain available without ANSI. Saving Settings reloads the appearance. Create an empty `snippets: []` custom source if desired, or let the first save create it when it matches an included glob.

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
