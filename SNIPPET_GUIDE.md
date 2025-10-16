# Snippet Creation Guide

This guide provides comprehensive documentation for creating command snippets in CS (Command Snippets). Learn how to build powerful, reusable command templates with intelligent variable substitution, transformations, and validation.

## Table of Contents

- [Quick Start](#quick-start)
- [Snippet Structure](#snippet-structure)
- [Variables](#variables)
  - [Variable Fields](#variable-fields)
  - [Variable Types](#variable-types)
  - [Validation](#validation)
  - [Default Values](#default-values)
- [Transformations](#transformations)
  - [Inline Transformations](#inline-transformations)
  - [Transform Templates](#transform-templates)
  - [Computed Variables](#computed-variables)
- [Variable Types (Reusable Definitions)](#variable-types-reusable-definitions)
- [Advanced Examples](#advanced-examples)
- [Best Practices](#best-practices)

## Quick Start

The simplest snippet consists of a command template with variables:

```yaml
snippets:
  hello-world:
    id: "hello-world"
    name: "hello-world"
    description: "Say hello to someone"
    command: "echo 'Hello, <name>!'"
    variables:
      - name: "name"
        description: "Name to greet"
        required: true
    tags: ["example", "simple"]
    created_at: "2025-01-01T00:00:00Z"
    updated_at: "2025-01-01T00:00:00Z"
```

## Snippet Structure

Every snippet consists of these top-level fields:

### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique identifier for the snippet (usually same as the YAML key) |
| `name` | string | Display name for the snippet (usually same as id) |
| `description` | string | Human-readable description of what the command does |
| `command` | string | The command template with `<variable>` placeholders |
| `created_at` | timestamp | ISO 8601 timestamp when snippet was created |
| `updated_at` | timestamp | ISO 8601 timestamp when snippet was last updated |

### Optional Fields

| Field | Type | Description |
|-------|------|-------------|
| `variables` | array | List of variable definitions (see [Variables](#variables)) |
| `tags` | array | Tags for organizing and searching snippets |

### Example: Complete Snippet Structure

```yaml
snippets:
  kubectl-get-pods:
    id: "kubectl-get-pods"
    name: "kubectl-get-pods"
    description: "Get Kubernetes pods with namespace selection"
    command: "kubectl get pods <namespace>"
    variables:
      - name: "namespace"
        description: "Kubernetes namespace"
        type: "namespace"
        transformTemplate: "k8s-namespace"
    tags: ["kubernetes", "pods", "kubectl"]
    created_at: "2025-01-01T00:00:00Z"
    updated_at: "2025-01-01T00:00:00Z"
```

## Variables

Variables are placeholders in your command template denoted by `<variable_name>`. Each variable used in the command **must** be explicitly defined in the `variables` array.

### Variable Fields

#### Required Field

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Variable name (must match the placeholder in the command) |

#### Optional Fields

| Field | Type | Description |
|-------|------|-------------|
| `description` | string | Help text shown to user during input |
| `required` | boolean | If true, user must provide a value (default: false) |
| `default` | string | Default value if user provides no input |
| `type` | string | Variable type (see [Variable Types](#variable-types)) |
| `validation` | object | Validation rules (see [Validation](#validation)) |
| `transform` | object | Inline transformation rules (see [Transformations](#transformations)) |
| `transformTemplate` | string | Reference to a reusable transform template |
| `computed` | boolean | If true, value is computed from other variables (default: false) |

### Variable Types

The `type` field can be:

#### Built-in Types
- `string` (default): Any text input
- `boolean`: True/false value (shown as `<true>` / `<false>` selector)
- `regex`: Regular expression pattern (validated on input)

#### Custom Types
You can define custom types in the `variable_types` section (see [Variable Types (Reusable Definitions)](#variable-types-reusable-definitions)):
- `port`: Network port (1-65535)
- `namespace`: Kubernetes namespace
- Any custom type you define

### Validation

Validation ensures user input meets specific criteria:

#### Pattern Validation

Use regular expressions to validate input format:

```yaml
variables:
  - name: "email"
    description: "Email address"
    validation:
      pattern: "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"
```

#### Enum Validation

Restrict input to a specific set of values:

```yaml
variables:
  - name: "log_level"
    description: "Logging level"
    validation:
      enum: ["debug", "info", "warn", "error"]
```

Users will see a selector with arrow keys to choose from the options.

#### Range Validation

For numeric inputs, specify min and max values:

```yaml
variables:
  - name: "port"
    description: "Port number"
    type: "port"
    validation:
      range: [1, 65535]
```

### Default Values

Provide sensible defaults to speed up command entry:

```yaml
variables:
  - name: "branch"
    description: "Git branch"
    default: "main"
  - name: "remote"
    description: "Git remote"
    default: "origin"
```

If a user presses Enter without typing, the default value is used.

## Transformations

Transformations modify how variable values appear in the final command. This is powerful for handling optional flags, conditional logic, and complex formatting.

### Inline Transformations

Define transformation logic directly in the variable:

#### Empty Value Transformation

Replace empty input with a specific value:

```yaml
variables:
  - name: "namespace"
    description: "Kubernetes namespace (empty for all)"
    transform:
      empty_value: "-A"  # When empty, use "-A" flag
```

**Usage:**
```bash
namespace: [Enter]
# Result: kubectl get pods -A
```

#### Value Pattern Transformation

Format non-empty values with a pattern:

```yaml
variables:
  - name: "port"
    description: "Port to expose"
    transform:
      empty_value: ""  # No flag when empty
      value_pattern: "-p {{.Value}}:{{.Value}}"  # Format when provided
```

**Usage:**
```bash
port: 8080
# Result: docker run -p 8080:8080 nginx
```

The `{{.Value}}` placeholder is replaced with the user's input.

#### Boolean Transformations

Convert boolean values to command flags:

```yaml
variables:
  - name: "follow"
    description: "Follow logs"
    type: "boolean"
    default: "false"
    transform:
      true_value: "-f"
      false_value: ""
```

**Usage:**
```bash
follow: <true>
# Result: kubectl logs pod-name -f

follow: <false>
# Result: kubectl logs pod-name
```

#### Advanced Value Patterns with Go Templates

Use Go template syntax for complex transformations:

```yaml
variables:
  - name: "output"
    description: "Output format"
    transform:
      empty_value: ""
      value_pattern: |
        {{- if eq .Value "json" -}}
          -o json
        {{- else if eq .Value "yaml" -}}
          -o yaml
        {{- else -}}
          -o wide
        {{- end -}}
```

### Transform Templates

For reusable transformation logic, define templates in the `transform_templates` section at the config root:

#### Defining Transform Templates

```yaml
transform_templates:
  k8s-namespace:
    description: "Kubernetes namespace: empty=none, 'all'=-A, name=-n <name>"
    transform:
      empty_value: ""
      value_pattern: |
        {{- if eq .Value "all" -}}
          -A
        {{- else -}}
          -n {{.Value}}
        {{- end -}}

  docker-port:
    description: "Docker port mapping (hostport:targetport)"
    transform:
      empty_value: ""
      value_pattern: "-p {{.Value}}:{{.Value}}"

  follow-flag:
    description: "Follow logs flag"
    transform:
      true_value: "-f"
      false_value: ""
```

#### Using Transform Templates

Reference templates by name in your variables:

```yaml
snippets:
  kubectl-get-pods:
    command: "kubectl get pods <namespace>"
    variables:
      - name: "namespace"
        description: "Kubernetes namespace"
        transformTemplate: "k8s-namespace"  # Reference the template

  docker-run:
    command: "docker run <port> <image>"
    variables:
      - name: "port"
        description: "Port mapping"
        transformTemplate: "docker-port"  # Reuse across snippets
```

**Benefits:**
- Define once, use everywhere
- Consistent behavior across commands
- Easier to maintain and update

### Computed Variables

Computed variables are calculated from other variables using the `compose` transformation. They don't prompt the user; instead, they combine values from other fields.

#### Basic Composition

```yaml
variables:
  - name: "resource_type"
    description: "Resource type"
    validation:
      enum: ["pod", "svc", "deployment"]
  
  - name: "resource_name"
    description: "Resource name"
    required: true
  
  - name: "resource"
    description: "Full resource reference"
    computed: true
    transform:
      compose: "{{.resource_type}}/{{.resource_name}}"
```

**Command:**
```yaml
command: "kubectl describe <resource>"
```

**Usage:**
```bash
resource_type: <pod>
resource_name: my-pod
# Result: kubectl describe pod/my-pod
```

#### Complex Composition with Conditionals

```yaml
variables:
  - name: "host_port"
    description: "Host port"
    required: true
  
  - name: "target_port"
    description: "Target port (empty to use host port)"
    default: ""
  
  - name: "port_mapping"
    description: "Complete port mapping"
    computed: true
    transform:
      compose: |
        {{- .host_port -}}:
        {{- if .target_port -}}
          {{- .target_port -}}
        {{- else -}}
          {{- .host_port -}}
        {{- end -}}
```

**Usage:**
```bash
host_port: 8080
target_port: [Enter]
# Result: port_mapping = "8080:8080"

host_port: 8080
target_port: 9090
# Result: port_mapping = "8080:9090"
```

The `compose` field receives all variable values as a map accessible via `{{.variable_name}}`.

## Variable Types (Reusable Definitions)

Define reusable variable configurations in the `variable_types` section. These can specify default validation rules, defaults, and transformations.

### Defining Variable Types

```yaml
variable_types:
  port:
    description: "Network port number"
    validation:
      range: [1, 65535]
    default: "8080"
  
  namespace:
    description: "Kubernetes namespace"
    default: "default"
  
  email:
    description: "Email address"
    validation:
      pattern: "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"
```

### Using Variable Types

```yaml
variables:
  - name: "http_port"
    description: "HTTP port"
    type: "port"  # Inherits validation and default from type definition
  
  - name: "https_port"
    description: "HTTPS port"
    type: "port"
    default: "443"  # Override the type's default
  
  - name: "kube_namespace"
    description: "Kubernetes namespace"
    type: "namespace"
    transformTemplate: "k8s-namespace"  # Combine type with transform
```

**Benefits:**
- Consistent validation across commands
- Reduce duplication
- Easier to update validation rules globally

## Advanced Examples

### Example 1: Git Branch with Remote

Create a snippet that constructs a full branch reference:

```yaml
snippets:
  git-checkout-remote:
    id: "git-checkout-remote"
    name: "git-checkout-remote"
    description: "Checkout remote branch"
    command: "git checkout <branch_ref>"
    variables:
      - name: "remote"
        description: "Remote name"
        default: "origin"
      - name: "branch"
        description: "Branch name"
        required: true
      - name: "branch_ref"
        description: "Full branch reference"
        computed: true
        transform:
          compose: "{{.remote}}/{{.branch}}"
    tags: ["git", "branch"]
    created_at: "2025-01-01T00:00:00Z"
    updated_at: "2025-01-01T00:00:00Z"
```

### Example 2: Docker Run with Optional Flags

Handle multiple optional Docker flags elegantly:

```yaml
snippets:
  docker-run-advanced:
    id: "docker-run-advanced"
    name: "docker-run-advanced"
    description: "Run Docker container with optional flags"
    command: "docker run <detach> <port> <volume> <name> <image>"
    variables:
      - name: "detach"
        description: "Run in detached mode"
        type: "boolean"
        default: "false"
        transform:
          true_value: "-d"
          false_value: ""
      
      - name: "port"
        description: "Port mapping (empty for none)"
        transform:
          empty_value: ""
          value_pattern: "-p {{.Value}}:{{.Value}}"
      
      - name: "volume"
        description: "Volume mount (empty for none)"
        transform:
          empty_value: ""
          value_pattern: "-v {{.Value}}"
      
      - name: "container_name"
        description: "Container name (empty for auto)"
        
      - name: "name"
        description: "Name flag"
        computed: true
        transform:
          compose: |
            {{- if .container_name -}}
              --name {{.container_name}}
            {{- end -}}
      
      - name: "image"
        description: "Docker image"
        required: true
    tags: ["docker", "container"]
    created_at: "2025-01-01T00:00:00Z"
    updated_at: "2025-01-01T00:00:00Z"
```

**Usage:**
```bash
detach: <true>
port: 8080
volume: [Enter]
container_name: my-app
image: nginx:latest
# Result: docker run -d -p 8080:8080 --name my-app nginx:latest
```

### Example 3: File Backup with Boolean Transform

Create a snippet that optionally adds a backup extension:

```yaml
snippets:
  sed-edit-file:
    id: "sed-edit-file"
    name: "sed-edit-file"
    description: "Edit file with sed"
    command: "sed -i<backup> 's/<search>/<replace>/g' <file>"
    variables:
      - name: "backup"
        description: "Create file backup"
        type: "boolean"
        default: "false"
        transform:
          true_value: ".bak"
          false_value: ""
      
      - name: "search"
        description: "Text to search for"
        required: true
      
      - name: "replace"
        description: "Replacement text"
        required: true
      
      - name: "file"
        description: "File to edit"
        required: true
    tags: ["sed", "edit", "file"]
    created_at: "2025-01-01T00:00:00Z"
    updated_at: "2025-01-01T00:00:00Z"
```

**Usage:**
```bash
backup: <true>
# Result: sed -i.bak 's/foo/bar/g' file.txt

backup: <false>
# Result: sed -i 's/foo/bar/g' file.txt
```

### Example 4: Complex Kubernetes Port Forward

Combine multiple computed variables:

```yaml
snippets:
  kubectl-port-forward:
    id: "kubectl-port-forward"
    name: "kubectl-port-forward"
    description: "Forward local port to pod or service"
    command: "kubectl port-forward <resource> <port_mapping> <namespace>"
    variables:
      - name: "resource_type"
        description: "Resource type"
        required: true
        default: "svc"
        validation:
          enum: ["pod", "svc"]
      
      - name: "resource_name"
        description: "Resource name"
        required: true
      
      - name: "resource"
        description: "Resource reference"
        computed: true
        transform:
          compose: "{{.resource_type}}/{{.resource_name}}"
      
      - name: "host_port"
        description: "Host port"
        required: true
        type: "port"
      
      - name: "target_port"
        description: "Target port (empty to use host port)"
        default: ""
        type: "port"
      
      - name: "port_mapping"
        description: "Port mapping"
        computed: true
        transform:
          compose: |
            {{- .host_port -}}:
            {{- if .target_port -}}
              {{- .target_port -}}
            {{- else -}}
              {{- .host_port -}}
            {{- end -}}
      
      - name: "namespace"
        description: "Kubernetes namespace"
        type: "namespace"
        transformTemplate: "k8s-namespace"
    tags: ["kubernetes", "port-forward", "networking"]
    created_at: "2025-01-01T00:00:00Z"
    updated_at: "2025-01-01T00:00:00Z"
```

## Best Practices

### 1. Use Descriptive Names

Good variable names make snippets self-documenting:

```yaml
# Good
variables:
  - name: "source_file"
  - name: "destination_dir"
  - name: "create_backup"

# Bad
variables:
  - name: "src"
  - name: "dst"
  - name: "bak"
```

### 2. Provide Helpful Descriptions

Descriptions guide users during input:

```yaml
variables:
  - name: "namespace"
    description: "Kubernetes namespace (empty=none, 'all'=all namespaces, or specific name)"
```

### 3. Set Sensible Defaults

Defaults speed up common use cases:

```yaml
variables:
  - name: "branch"
    default: "main"
  - name: "remote"
    default: "origin"
  - name: "log_level"
    default: "info"
```

### 4. Use Transform Templates for Reusability

Don't repeat transformation logic:

```yaml
# Good: Define once
transform_templates:
  follow-flag:
    transform:
      true_value: "-f"
      false_value: ""

# Use everywhere
variables:
  - name: "follow"
    transformTemplate: "follow-flag"

# Bad: Repeat everywhere
variables:
  - name: "follow"
    transform:
      true_value: "-f"
      false_value: ""
```

### 5. Use Variable Types for Common Patterns

Create types for frequently used validation:

```yaml
variable_types:
  port:
    validation:
      range: [1, 65535]
  email:
    validation:
      pattern: "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"
```

### 6. Leverage Computed Variables

Reduce user input by computing values:

```yaml
# User only enters: resource_type, resource_name
# System computes: resource = "pod/my-pod"
variables:
  - name: "resource"
    computed: true
    transform:
      compose: "{{.resource_type}}/{{.resource_name}}"
```

### 7. Use Enums for Fixed Choices

Provide a selector instead of free text:

```yaml
variables:
  - name: "log_level"
    validation:
      enum: ["debug", "info", "warn", "error"]
```

### 8. Add Meaningful Tags

Tags help organize and discover snippets:

```yaml
tags: ["kubernetes", "pods", "kubectl", "networking"]
```

### 9. Keep Commands Simple

Break complex commands into multiple snippets:

```yaml
# Good: Focused snippets
snippets:
  docker-build:
    command: "docker build -t <image>:<tag> ."
  docker-push:
    command: "docker push <image>:<tag>"

# Bad: One giant snippet
snippets:
  docker-build-and-push:
    command: "docker build -t <image>:<tag> . && docker push <image>:<tag>"
```

### 10. Test Your Snippets

Use the interactive preview to ensure transformations work correctly:

```bash
cs exec your-snippet
# Check the command preview at the top
# Verify variables transform as expected
```

## Configuration Organization

### Single File

For small collections, keep everything in one file:

```yaml
# ~/.config/cs/config.yaml
transform_templates:
  # Your templates

variable_types:
  # Your types

snippets:
  # Your snippets

settings:
  # Your settings
```

### Multiple Files

For larger collections, split by topic:

```yaml
# ~/.config/cs/config.yaml
transform_templates:
  # Shared templates

variable_types:
  # Shared types

settings:
  additional_configs:
    - "snippets/kubernetes.yaml"
    - "snippets/docker.yaml"
    - "snippets/git.yaml"
```

```yaml
# ~/.config/cs/snippets/kubernetes.yaml
snippets:
  kubectl-get-pods:
    # Kubernetes snippets
```

### Project-Specific Snippets

Add `.csnippets` files in project directories:

```yaml
# .csnippets (in project root)
snippets:
  dev-build:
    description: "Build this project"
    command: "go build -o ./bin/<name> ."
    variables:
      - name: "name"
        default: "myapp"
```

## Troubleshooting

### Variable Not Found

**Error:** `variable <name> not defined`

**Solution:** Ensure every `<variable>` in the command has a matching entry in `variables`:

```yaml
command: "kubectl get pods <namespace>"
variables:
  - name: "namespace"  # Must match <namespace> in command
```

### Transform Template Not Found

**Error:** `transform template 'xyz' not found`

**Solution:** Check that the template is defined in `transform_templates`:

```yaml
transform_templates:
  xyz:  # Must exist
    transform:
      # ...
```

### Validation Failing

**Error:** `variable <name> does not match required format`

**Solution:** Check your validation pattern and test with expected input:

```yaml
validation:
  pattern: "^[a-z0-9-]+$"  # Only lowercase, numbers, hyphens
```

### Compose Template Errors

**Error:** Template execution errors

**Solution:** Use `{{- ... -}}` to control whitespace and test incrementally:

```yaml
compose: |
  {{- .var1 -}}
  {{- if .var2 -}}
    /{{- .var2 -}}
  {{- end -}}
```

## Further Reading

- [README.md](README.md) - Main project documentation
- [TESTING.md](TESTING.md) - Testing guidelines
- [snippets/](snippets/) - Example snippet files for reference

## Getting Help

If you have questions or need help:

1. Check example snippets in the `snippets/` directory
2. Use `cs describe <snippet-id>` to inspect existing snippets
3. Use `cs show transforms` and `cs show types` to see available building blocks
4. Review the example files in this guide

Happy snippet creation! 🚀

