# Suggested Commands

## Build
```bash
make build          # Build binary (./cs)
make install        # Build + install to /usr/local/bin + setup config
make dev            # Build and run with local config (./cs --config ./cs.yaml)
```

## Test
```bash
make test           # Run all tests (go test -v ./...)
go test ./...       # Run all tests (shorter output)
go test ./internal/models/...     # Models only
go test ./internal/template/...   # Template processor only
go test ./internal/ -run TestIntegration  # Integration tests
go test ./... -cover              # With coverage
```

## Lint & Format
```bash
make lint           # Run golangci-lint
make fmt            # Format code (go fmt ./...)
```

## Other
```bash
make tidy           # go mod tidy
make clean          # Remove build artifacts
make build-all      # Cross-platform builds (linux/darwin/windows)
```

## Git / System Utils
Standard Linux commands: `git`, `ls`, `grep`, `find`, etc.
