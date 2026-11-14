# Creating commands in CS

Use `cs add` for guided authoring, or edit YAML directly. A snippet needs only a friendly `name` and literal `command`:

```yaml
snippets:
  - name: Git status
    tags: [Git]
    command: git status --short
```

Snippet fields are `id`, `name`, `description`, `tags`, `command`, `inputs` and `expressions`. IDs are optional lowercase UUID v4 values. Do not copy an existing ID to a different command. Reading never assigns IDs; an explicit save assigns one to an ID-less entry. Tags define categories; there is no group or slug field.

## Guided authoring

The framed workspace contains the title, multiline command, description and one Tags field, with input configuration beside it (stacked at narrower sizes). Tab/Shift-Tab traverses controls; Left/Right changes selections. Type or paste `ls -lah {{folder_name}}`: a complete valid token creates a required text input once, in first-occurrence order. Repeated placeholders reuse it; existing definitions and expressions keep their names, types and settings. Partial or invalid tokens remain editable and produce compile feedback. Removing a token does not delete its definition; use the inline Remove action deliberately. Input outputs already include any required shell quoting.

Tags split on Unicode whitespace and deduplicate using the library's case-folded identity, retaining first spelling/order. Existing multiword tags stay intact while the field is untouched; its help warns that editing splits on whitespace. Saved tags remain a YAML sequence. The read-only Saved to label shows the configured default source for a new command and the owning source for an edit.

The selected input's inline controls contain labels/help, behavior, defaults, requiredness, flags, choice label/output rows, special value/output rows, validation and visibility/required conditions. Expressions and Test preview are discoverable inline actions; no Ctrl-N/Ctrl-P is needed for normal authoring. Test uses the same live preview and controls as command use and never emits or executes a command. Missing required test values are reported as an incomplete representative test; the reusable definition can still be saved, but cannot be inserted until its visible inputs validate.

## Declarative inputs

```yaml
snippets:
  - name: Run a container with options
    tags: [Docker]
    command: docker run {{detach}} {{env}} {{name}} {{image}}
    inputs:
      - {name: detach, kind: toggle, flag: -d, default: false}
      - name: env
        kind: repeat
        flag: -e
        default: []
        validate: {pattern: '^[A-Za-z_][A-Za-z0-9_]*=.*$'}
      - {name: name, kind: flag, flag: --name}
      - {name: image, required: true, default: 'nginx:latest'}
```

| Kind | Raw value | Output |
| --- | --- | --- |
| `text` (default) | String | One argument, quoted if needed; empty omits |
| `flag` | String | Literal flag plus quoted value; empty omits |
| `toggle` | Boolean | Literal flag when true, nothing when false |
| `choice` | Choice label | Selected literal output fragment |
| `repeat` | String list | One flag and independently quoted value per item |

Flag, toggle and repeat require `flag`. Choice requires ordered `choices: [{label: CPU, value: "3"}, {label: Memory, value: "4"}]`; omitted default selects the first label. Invalid explicit labels are errors, not silently replaced. `special: {all: -A, "": ""}` maps exact raw strings to literal fragments before normal output for text, flag and choice. Hidden inputs bypass special mappings.

Every input supports `name`, `label`, `help`, `kind`, `default`, `required`, `flag`, `choices`, `special`, `validate`, `visible_when` and `required_when` where applicable. Names match `[A-Za-z_][A-Za-z0-9_]*` and cannot collide with another input or expression. Omitted defaults are empty text/list, false toggle, or the first choice. An explicit empty preset does not restore a default.

Defaults must have the input's type. Quote numeric text: `default: "8080"`, not `default: 8080`. YAML booleans must be booleans, not strings. Validation can combine `pattern` (Go RE2), inclusive integer `range: [1, 65535]`, and `regex: true` (the entered value must compile as RE2). Optional empty values skip content checks. Required repeat needs at least one item; required toggle needs true. Toggle inputs reject pattern/range/regex content validation; use requiredness and conditions instead. Choice labels remain required/valid even when their output is empty.

Repeated CLI presets keep item boundaries:

```sh
cs render 'Run a container with options' --set env=A=1 --set 'env=B=hello world'
```

The first `=` separates name and value. Scalar, toggle and choice names cannot be supplied twice; repeat names append items. Unknown names and expression presets fail.

## Conditions

```yaml
snippets:
  - name: Get pods in a chosen namespace
    command: kubectl get pods {{mode}} {{namespace}}
    inputs:
      - name: mode
        kind: choice
        choices:
          - {label: Default, value: ""}
          - {label: All, value: -A}
          - {label: Named, value: ""}
      - name: namespace
        kind: flag
        flag: -n
        visible_when: {input: mode, equals: Named}
        required_when: {input: mode, equals: Named}
```

A condition uses exactly one of `{input, equals}`, `{input, not_equals}` or `{expr}`. Comparisons use the controller's raw value and type, not its emitted shell fragment. Dependencies may refer to later inputs; unknown names, self-dependencies and cycles fail validation. Hidden values are cached for restoration, but downstream conditions and expressions see typed zero values. Hidden inputs emit nothing and skip required/content validation.

## Literal command text and expressions

Only `{{input_name}}` and `{{expression_name}}` interpolate. Write `{{{{` to emit a literal `{{`; a stray closing `}}` is literal. `${HOME}`, `${x:-fallback}`, `$0`, pipes, braces, heredocs and newlines remain shell text. Inserted values are never scanned again. Placeholders producing complete shell arguments belong outside existing shell quotes.

For a larger assembled argument, use one named expression:

```yaml
snippets:
  - name: Forward a service port
    command: kubectl port-forward {{resource}} {{ports}}
    inputs:
      - {name: service, required: true}
      - {name: host_port, required: true, default: "8080", validate: {range: [1, 65535]}}
      - {name: target_port, validate: {range: [1, 65535]}}
    expressions:
      resource: 'quote("svc/" + inputs.service)'
      ports: 'quote(inputs.host_port + ":" + default(inputs.target_port, inputs.host_port))'
```

Expressions use typed `inputs.name`, string/boolean literals, string/boolean operators, ternary selection, and pure helpers:

- `quote(text)` always shell-quotes a complete argument.
- `flag(option, text)` omits empty text, otherwise quotes its value.
- `boolFlag(option, enabled)` emits when enabled is true.
- `repeatFlag(option, items)` preserves each string-list item.
- `join(items, separator)` joins nonempty strings.
- `default(text, fallback)` chooses fallback only for empty text.
- `empty(value)` checks a string, boolean or string list for its typed zero.

Named expressions return strings; expression conditions return booleans. They have no environment, filesystem, process, network or clock access. Methods, ranges, comprehensions and collection iteration are rejected. Limits are 4 KiB and 1,000 nodes per expression, and 1 MiB per rendered command. Literal command bytes outside placeholders are not normalized.

## Sources and supported YAML

```yaml
settings:
  sources: [snippets/*.yaml]
  project_source: true
  default_source: snippets/custom.yaml
  color: auto
snippets: []
```

Only the main file may contain settings. Relative source/destination paths resolve next to it; `~/` expands to the home directory. Includes load in declared order, glob matches in lexical order, and duplicate canonical paths load once. An empty glob warns; a missing literal path fails. Enabled project discovery reads only the current directory's `.csnippets`. Duplicate display names are allowed; duplicate persisted IDs are not.

Settings defaults are no includes, project discovery enabled, destination equal to the main file, `color: auto`, and the original `default` theme. Color can be `auto`, `always` or `never`; `--no-color` and nonempty `NO_COLOR` override it. Main settings also accept `theme: catppuccin-mocha` and optional semantic `theme_colors: {focus: '#cba6f7'}` overrides; see [Themes in README](README.md#themes) for every supported role. A new destination must be the main file, enabled `.csnippets`, or match a configured include. Changing the default destination does not create a file.

Use a single document with a block root mapping and a block snippet sequence. Inline lists/maps within fields and literal/folded command scalars are supported. Anchors, aliases, custom tags, merge keys, duplicate keys, unknown fields and flow-style snippet entries fail closed. The old schema has no compatibility layer or automatic migration. A recovery screen lets you retry after external correction, or remove a broken include through settings when the main file is parseable.

Saves preserve unrelated source bytes and guard content/identity/permission conflicts. A failed save retains the editor draft. Included/project snippets never get flattened into the main file. Removing an include never deletes its file. See [README.md](README.md) for locks, durability warnings and the residual external-writer race.
