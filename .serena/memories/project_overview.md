# Project Overview: Command Snippets (CS)

## Purpose
CS is a CLI tool for managing command templates with intelligent variable substitution. 
It provides conditional transformations, reusable template patterns, and smart variable processing.

## Tech Stack
- **Language**: Go 1.24
- **CLI Framework**: spf13/cobra
- **TUI**: charmbracelet/bubbletea, charmbracelet/lipgloss
- **Prompts**: AlecAivazis/survey/v2
- **Config**: gopkg.in/yaml.v3
- **Terminal**: muesli/termenv, golang.org/x/term

## Repository Structure
```
cmd/cs/main.go          - Entry point
internal/
  cmd/                  - Cobra command implementations
    root.go             - Root command, config loading
    exec.go             - Execute snippets
    add.go              - Add new snippets
    list.go             - List snippets
    search.go           - Search snippets
    show.go             - Show config components
    describe.go         - Describe a specific snippet
    edit.go             - Edit snippets/config
    selector.go         - External selector support (fzf, etc.)
  models/
    snippet.go          - Core data models (Snippet, Variable, Transform, Config)
    snippet_test.go     - Model tests
    config_test.go      - Config loading tests
  template/
    processor.go        - Template processing engine
    processor_test.go   - Processor tests
    form.go             - Form/input handling
    confirm.go          - Confirmation dialogs
  regex/
    regex.go            - Regex utilities
  integration_test.go   - End-to-end tests
snippets/               - Example YAML snippet files (kubernetes, docker, git, gnu)
testdata/               - Test fixtures
```

## Config Location
- Default: `~/.config/cs/config.yaml`
- Supports additional configs via glob patterns
- Local project snippets via `.csnippets` file
