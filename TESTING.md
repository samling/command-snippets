# Testing Documentation

This document describes the test suite for the command-snippets project.

## Test Structure

The test suite is organized into several components:

### Test Fixtures (`testdata/`)

Test fixtures are stored in the `testdata/` directory and include:

- **`config.yaml`** - Test configuration file
- **`transform_templates.yaml`** - Reusable transformation templates for testing
- **`types.yaml`** - Variable type definitions for testing
- **`test_snippets.yaml`** - Comprehensive collection of test snippets

### Test Files

#### Model Tests (`internal/models/`)

- **`snippet_test.go`** - Tests for snippet processing, variable handling, and transformations
  - Template processing without variables
  - Simple variable substitution
  - Default values
  - Boolean transformations
  - Transform templates
  - Value patterns
  - Computed variables (simple and conditional)
  - Complex computed variables
  - Multiple transforms combined
  - Validation (required, enum, range, pattern, regex)
  - Error handling

- **`config_test.go`** - Tests for configuration loading and structure
  - Loading main config
  - Loading transform templates
  - Loading variable types
  - Loading snippets
  - Verifying structure of loaded configurations

#### Processor Tests (`internal/template/`)

- **`processor_test.go`** - Tests for the template processor
  - Processor creation
  - Processing snippets with various variable types
  - All transformation scenarios
  - Error handling

#### Integration Tests (`internal/`)

- **`integration_test.go`** - End-to-end workflow tests
  - Simple snippets
  - Kubernetes-style workflows
  - Docker-style workflows
  - Validation workflows
  - Computed variables workflows
  - Transform workflows
  - Comprehensive feature combinations
  - Error scenarios

## Test Snippets Coverage

The test suite includes 17 different snippet types that exercise all functionality:

1. **simple-no-vars** - Snippet with no variables
2. **simple-with-vars** - Basic variable substitution
3. **snippet-with-defaults** - Default value handling
4. **snippet-with-enum** - Enum validation (using variable types)
5. **snippet-with-range** - Range validation (using variable types)
6. **snippet-with-pattern** - Pattern validation (using variable types)
7. **snippet-with-inline-pattern** - Inline pattern validation
8. **snippet-with-boolean** - Boolean transformations
9. **snippet-with-transform-template** - Using transform templates
10. **snippet-with-value-pattern** - Value pattern transformations
11. **snippet-with-empty-value** - Empty value handling
12. **snippet-with-computed-simple** - Simple computed variables
13. **snippet-with-computed-conditional** - Conditional computed variables
14. **snippet-with-multiple-transforms** - Multiple transform types
15. **snippet-with-complex-computed** - Complex computed variables
16. **snippet-with-regex-type** - Regex type validation
17. **snippet-with-all-features** - Comprehensive feature combination

## Transform Templates

Test transform templates include:

- **test-namespace** - Kubernetes namespace transformation (empty, "all", or specific)
- **test-port-mapping** - Docker port mapping
- **test-boolean-flag** - Boolean flag transformation
- **test-prefix** - Simple prefix pattern

## Variable Types

Test variable types include:

- **test_port** - Port number with range validation (1-65535)
- **test_log_level** - Log level with enum validation (debug, info, warn, error)
- **test_environment** - Environment with enum validation (dev, staging, prod)
- **test_email** - Email with pattern validation
- **test_version** - Semantic version with pattern validation

## Running Tests

### Run All Tests

```bash
go test ./...
```

### Run Tests with Verbose Output

```bash
go test ./... -v
```

### Run Tests for Specific Package

```bash
# Models only
go test ./internal/models/...

# Template processor only
go test ./internal/template/...

# Integration tests only
go test ./internal/integration_test.go
```

### Run Specific Test

```bash
go test ./internal/models -run TestProcessTemplate_BooleanTransform
```

### Run Tests with Coverage

```bash
go test ./... -cover
```

### Generate Coverage Report

```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Test Philosophy

The test suite follows these principles:

1. **Comprehensive Coverage** - All features are tested including edge cases
2. **Real-World Scenarios** - Tests use realistic examples (Kubernetes, Docker)
3. **Independent Test Data** - Test fixtures are separate from production snippets
4. **Clear Test Names** - Test names clearly describe what they test
5. **Subtests** - Related tests are grouped using table-driven tests and subtests
6. **Error Testing** - Both success and failure paths are tested

## Adding New Tests

When adding new features:

1. Add test fixtures to `testdata/` if needed
2. Create test cases in the appropriate test file
3. Include both positive and negative test cases
4. Update this documentation

## Test Output

All tests should pass with output similar to:

```
ok      github.com/samling/command-snippets/internal           0.008s
ok      github.com/samling/command-snippets/internal/models    0.015s
ok      github.com/samling/command-snippets/internal/template  0.013s
```
