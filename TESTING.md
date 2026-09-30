# Testing CS

Run source, rendering, workspace, CLI, widget and provisioning tests:

```sh
go test ./...
go test -race ./...
go build -v ./...
go vet ./...
gofmt -l internal cmd
```

Tests create their own files and source snapshots. Command integration changes to a temporary working directory; widget tests stub CS and shell editor registration. Never run verification against installed libraries, shell startup files, the ignored root `config.yaml`/`.csnippets`, or the ignored root binary `cs`.

## Test ownership

- `internal/models`: canonical identities/types and shipped-example render cases.
- `internal/library`: strict loading, provenance, IDs, supported YAML, guarded source-span writes, byte preservation and fault injection.
- `internal/templating`: placeholder grammar, typed pure expressions, defaults/presets, validation, hidden effective values and exact shell-text preservation.
- `internal/search`: field ranking, deterministic identity ordering, normalized tag intersections and generated benchmarks.
- `internal/template`: rune-aware editing, paste, controls, visibility caches and preview/submission parity.
- `internal/workspace`: library/editor/settings transitions, marking, round-trip fields, recovery, cancellation and resize.
- `internal/cmd` / `internal/integration_test.go`: init no-overwrite behavior, CLI output safety and guarded Zsh/Bash widgets.
- `internal/defaults`: exact byte equality between embedded and top-level examples.

`testdata/config.yaml` and `testdata/test_snippets.yaml` use only the canonical schema. Legacy fixtures/tests were replaced, not retained as a compatibility promise.

## Performance receipts

Normal-workload acceptance budgets apply to a documented development Linux host, not arbitrary CI timing. Enable the explicit percentile/heap receipt:

```sh
CS_PERFORMANCE_TEST=1 go test -run TestPerformanceReceipt -v ./internal/workspace
go test -run '^$' -bench . -benchmem ./internal/search ./internal/workspace ./internal/templating
```

The receipt creates 10,000 entries across 20 source files, 100 tags, 10% untagged commands, repeated/Unicode titles, 120-character descriptions, 512-byte median commands and a 4-KiB tail. It measures index build, incremental retained heap, 240 query/filter-plus-view updates, 20-input/8-expression preview, and a 50,000-entry stress load/index/query. Normal gates are 200 ms index build, 64 MiB retained library/search heap, 50 ms query/view p95 with no sample above 100 ms, and 20 ms preview p95. Use an uninstrumented build for those timing gates; race checks run separately.

## Real terminal checks

```sh
SCRATCH=$(mktemp -d /tmp/cs-check.XXXXXX)
go build -o "$SCRATCH/cs" ./cmd/cs
python3 scripts/verify_pty.py "$SCRATCH/cs"
```

The harness uses temporary HOME, XDG_CONFIG_HOME and cwd, captured command stdout, and a real controlling PTY on stderr with stdin redirected. It asserts search/use without execution, empty cancellation, invalid all-preset input followed by valid insertion, guided creation, source-local rename/ID stability, tiny-terminal resize and guided pod authoring through marking, flag/special mappings, mapped choices and explicit duplicate linking. It measures 20 warm 10,000-entry launches; p95 must be at most 500 ms. Commands that refer to Docker/Kubernetes/filesystem operations are rendered, never executed.

The automated terminal transcript is a functional receipt, not a human visual approval. Review the library/editor/settings at 140×40, 90×24, 60×18 and 40×10 when judging spacing and readability. Model resize tests also cover 120×20 and 20×5.

## Portability

Compile `./cmd/cs` with `CGO_ENABLED=0` for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 and windows/amd64 into scratch outputs. Preserve the release version linker symbol `github.com/samling/command-snippets/internal/cmd.version`. Compilation does not prove non-Linux filesystem/terminal behavior. Windows saves use replace-file semantics and explicitly report unavailable portable directory-sync durability.

Saves coordinate CS writers with an exclusive sibling lock and reject changed file identity/content/permissions. External editors retain the narrow final-check/replacement race documented in the README. Failure tests compare original bytes and metadata, not just decoded YAML values.
