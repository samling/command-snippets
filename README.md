# TplKit - Advanced Command Template Toolkit

**TplKit** is a powerful CLI tool for managing command templates with intelligent variable substitution. It goes beyond simple snippet storage by providing conditional transformations, reusable template patterns, and smart variable processing.

## Features

- **Intelligent Template Engine**: Variable transformations with conditional logic
- **Reusable Transformation Patterns**: Define transformation rules once, use across multiple commands
- **Interactive Execution**: Smart prompting with validation and defaults
- **Flexible Configuration**: YAML-based configuration with inheritance
- **Tag-based Organization**: Organize and search templates by tags
- **Shell Integration**: Execute commands directly or copy to clipboard

## Quick Start

### Installation

```bash
# Build from source
git clone <repository>
cd tplkit
go build -o tplkit
sudo mv tplkit /usr/local/bin/
```

### Basic Usage

```bash
# Add your first command template
tplkit add

# List all templates
tplkit list

# Execute a template interactively
tplkit exec kubectl-get-pods

# Search for templates
tplkit search kubernetes
```

## Core Concepts

### Transform Templates
Transform templates define reusable transformation logic that can be referenced by multiple variables. They contain the transformation rules for how variables should behave.

### Variables  
Variables in commands are denoted with `<variable_name>` and must be **explicitly defined** in each snippet. Each variable can have:
- **Transform templates**: Reference to reusable transformation logic
- **Inline transforms**: Custom transformation defined directly in the variable
- **Default values**: Used when no input provided
- **Validation**: Ensure input meets criteria
- **Types**: Boolean, enum, string, etc.

### Explicit Configuration
**No magic linking** - every variable is explicitly configured. You can see exactly what each variable does by looking at the snippet definition.

## Examples

### Kubernetes Namespace Pattern
A common pattern where an empty namespace should default to all namespaces, but a specific namespace should be properly formatted:

```yaml
# Define reusable transform template
transform_templates:
  k8s-namespace:
    description: "Kubernetes namespace with -A default"
    transform:
      empty_value: "-A"
      value_pattern: "-n {{.Value}}"

# Use the template in a snippet
snippets:
  kubectl-get-pods:
    description: "Get pods with optional namespace"
    command: "kubectl get pods <namespace>"
    variables:
      - name: "namespace"
        description: "Kubernetes namespace"
        transformTemplate: "k8s-namespace"  # Reference the template
```

**Usage:**
```bash
$ tplkit exec kubectl-get-pods
namespace (Kubernetes namespace): [Enter]
Executing: kubectl get pods -A

$ tplkit exec kubectl-get-pods  
namespace (Kubernetes namespace): kube-system
Executing: kubectl get pods -n kube-system
```

This demonstrates explicit variable configuration with reusable transformation templates.

## CLI Commands

### `tplkit add`
Add a new command template interactively:
```bash
tplkit add    # Interactive template creation with explicit variable configuration
```

During creation, you'll be prompted to configure each variable found in your command template. You can choose:
- **No transformation**: Simple variable substitution
- **Inline transform**: Custom transformation defined directly
- **Transform template**: Reference to reusable transformation logic

### `tplkit list`
List and filter templates:
```bash
tplkit list                  # List all templates
tplkit list --tags kubernetes # Filter by tags
tplkit list --verbose        # Show detailed info
```

### `tplkit exec`
Execute templates with interactive prompting:
```bash
tplkit exec kubectl-get-pods # Execute specific template
tplkit exec                  # Interactive selection
```

### `tplkit search`
Search through templates:
```bash
tplkit search kubectl        # Find templates containing "kubectl"
tplkit search "get pods"     # Multi-word search
```

### `tplkit edit`
Edit templates or configuration:
```bash
tplkit edit kubectl-get-pods # Edit specific template
tplkit edit --config         # Edit configuration file
```

## ⚙️ Configuration

TplKit uses a YAML configuration file (default: `~/.config/tplkit/tplkit.yaml`) with three main sections:

### Transform Templates
Define reusable transformation logic:
```yaml
transform_templates:
  my-transform:
    description: "Description of this transformation"
    transform:
      empty_value: "default"
      value_pattern: "--flag {{.Value}}"
```

### Variable Types
Define reusable variable configurations:
```yaml
variable_types:
  port:
    description: "Network port"
    validation:
      range: [1, 65535]
    default: "8080"
```

### Command Templates (Snippets)
Your actual command templates with explicit variable definitions:
```yaml
snippets:
  my-command:
    description: "What this command does"
    command: "command <var1> <var2>"
    variables:
      - name: "var1"
        description: "First variable"
        transformTemplate: "my-transform"  # Reference transform template
        required: true
      - name: "var2"
        description: "Second variable"
        transform:                         # Inline transform
          empty_value: "default"
          value_pattern: "--{{.Value}}"
    tags: ["tag1", "tag2"]
```

## Advanced Examples

### Boolean Flags with Transform Templates
Handle optional flags elegantly with reusable templates:

```yaml
transform_templates:
  follow-flag:
    description: "Follow logs flag"
    transform:
      true_value: "-f"
      false_value: ""

snippets:
  kubectl-logs:
    command: "kubectl logs <pod> <follow>"
    variables:
      - name: "pod"
        description: "Pod name"
        required: true
      - name: "follow"
        description: "Follow logs"
        type: "boolean"
        transformTemplate: "follow-flag"
```

### Complex Compositions
Combine multiple variables with computed values:

```yaml
snippets:
  git-checkout:
    command: "git checkout <branch_ref>"
    variables:
      - name: "branch"
        description: "Branch name"
        required: true
      - name: "remote"
        description: "Remote name"
        default: "origin"
      - name: "branch_ref"
        description: "Full branch reference"
        computed: true
        transform:
          compose: "{{.remote}}/{{.branch}}"
```

### Inline Transforms
Custom transformation logic defined directly:

```yaml
snippets:
  docker-run:
    command: "docker run <port> <image>"
    variables:
      - name: "port"
        description: "Port mapping"
        transform:
          empty_value: ""  # No port flag if empty
          value_pattern: "-p {{.Value}}:{{.Value}}"  # Map port
      - name: "image"
        description: "Docker image"
        required: true
```

## Why TplKit?

Compared to simple snippet managers, TplKit provides:

1. **Smart Defaults**: Intelligent handling of empty vs specified values
2. **Conditional Logic**: Variables behave differently based on input
3. **Reusable Patterns**: Define transformation rules once, use everywhere
4. **Type Safety**: Validation ensures correct input
5. **Composability**: Build complex commands from simple patterns

## Project Structure

```
tplkit/
├── main.go                    # Entry point
├── cmd/                       # CLI commands
│   ├── root.go               # Root command and config loading
│   ├── add.go                # Add new templates
│   ├── list.go               # List templates
│   ├── exec.go               # Execute templates
│   ├── search.go             # Search templates
│   └── edit.go               # Edit templates/config
├── internal/
│   ├── models/               # Data structures
│   │   └── snippet.go        # Core models and processing
│   └── template/             # Template processing
│       └── processor.go      # Variable substitution engine
├── tplkit.yaml               # Example configuration
└── README.md                 # This file
```

## Example Workflow

1. **Define a transform template:**
   ```yaml
   transform_templates:
     docker-port:
       description: "Docker port mapping"
       transform:
         empty_value: ""
         value_pattern: "-p {{.Value}}:{{.Value}}"
   ```

2. **Create snippets with explicit variables:**
   ```bash
   tplkit add
   # You'll be prompted to configure each variable explicitly
   # You can choose to use transform templates or inline transforms
   ```

3. **Execute with smart prompting:**
   ```bash
   tplkit exec docker-run
   port_mapping (Port mapping (empty for none)): 8080
   image_name (Docker image name): nginx
   Executing: docker run -p 8080:8080 nginx
   ```

**TplKit** - Where intelligent templating meets practical command management!
