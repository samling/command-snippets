# Code Style & Conventions

## Go Conventions
- Standard Go style (gofmt)
- Package-level comments on exported types (e.g. `// Snippet represents a command template`)
- Struct tags use `yaml:"field_name"` with `omitempty` where appropriate
- Error handling follows standard Go patterns (return error, check with `if err != nil`)
- Methods use pointer receivers (e.g. `func (s *Snippet) ProcessTemplate(...)`)

## Project Patterns
- **Cobra commands** in `internal/cmd/` - each subcommand in its own file
- **Models** in `internal/models/` - core data structures and business logic
- **Template processing** in `internal/template/` - template engine separated from models
- **Tests**: table-driven tests with subtests, test fixtures in `testdata/`
- **YAML config**: snake_case for YAML field names, camelCase for Go-specific fields (e.g. `transformTemplate`)

## Testing
- Table-driven tests with descriptive names
- Test data in `testdata/` directory  
- Integration tests in `internal/integration_test.go`
- See TESTING.md for full test documentation

## Task Completion Checklist
When a coding task is completed:
1. Run `make fmt` to format code
2. Run `make test` to ensure all tests pass
3. Run `make lint` if golangci-lint is available
4. Run `make build` to verify compilation
