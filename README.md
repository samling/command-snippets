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

# Build and install binary + setup config directory
make install
```

**Note:** The `snippets/` directory in this repository contains example snippet files that you can use as reference for creating your own templates.

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

## Shell Integration

TplKit is designed to integrate seamlessly with your shell workflow. The default behavior outputs clean commands to stdout, making it perfect for shell functions and keybindings.

### Execution Modes

```bash
# Print command only (default - perfect for shell integration)
tplkit exec kubectl-get-pods

# Execute automatically without prompting
tplkit exec kubectl-get-pods --run

# Prompt before executing (classic behavior)
tplkit exec kubectl-get-pods --prompt
```

### Zsh Keybinding Integration

Create a zsh function to invoke TplKit with a keybinding (e.g., Ctrl-S) that inserts the generated command directly into your command line:

**Setup:**

Add this to your `~/.zshrc`:

```zsh
# TplKit integration - Ctrl-S to invoke template selector
function tplkit-select() {
  RBUFFER=$(tplkit exec)  # Uses default config: ~/.config/tplkit/config.yaml
  CURSOR=$#BUFFER
  zle redisplay
}

# Register the function as a zle widget
zle -N tplkit-select

# Disable terminal flow control (frees up Ctrl-S)
stty -ixon

# Bind Ctrl-S to our function
bindkey '^s' tplkit-select
```

**Usage:**

1. **Press Ctrl-S** → TplKit opens with your configured selector (e.g., fzf)
2. **Select a template** → Interactive prompts appear for variables
3. **Fill in variables** → Validation ensures correct input
4. **Command appears** → Generated command is inserted at your cursor position

**Example workflow:**
```bash
❯ kubectl get pods  # Your existing partial command
# Press Ctrl-S, select "port-forward", fill variables
❯ kubectl get pods kubectl port-forward svc/my-app 8080:8080 -n production
#                  ↑ New command inserted at cursor
```

### External Selector Configuration

TplKit supports external selectors like fzf, rofi, or dmenu for better template selection:

```yaml
# In your ~/.config/tplkit/config.yaml
settings:
  selector:
    command: "fzf"
    options: "--height 40% --reverse --border --header='Select template:'"
```

**Popular selector configurations:**

```yaml
# fzf (recommended)
selector:
  command: "fzf"
  options: "--height 40% --reverse --border --preview 'echo {}' --header='Select template:'"

# rofi
selector:
  command: "rofi"
  options: "-dmenu -i -p 'Template' -theme-str 'window {width: 50%;} listview {lines: 10;}'"

# dmenu
selector:
  command: "dmenu"
  options: "-l 10 -p 'Select template:' -fn 'monospace-12'"
```

### Bash Integration

For bash users, you can create a similar function:

```bash
# Add to ~/.bashrc
tplkit-select() {
  local cmd=$(tplkit exec)  # Uses default config: ~/.config/tplkit/config.yaml
  if [[ -n "$cmd" ]]; then
    READLINE_LINE="${READLINE_LINE:0:$READLINE_POINT}$cmd${READLINE_LINE:$READLINE_POINT}"
    READLINE_POINT=$((READLINE_POINT + ${#cmd}))
  fi
}

# Bind to Ctrl-S
bind -x '"\C-s": tplkit-select'
```

### Pipeline Integration

TplKit's clean stdout makes it perfect for pipelines:

```bash
# Save command to file
tplkit exec kubectl-get-pods > my-command.sh

# Execute directly
tplkit exec kubectl-get-pods | sh

# Modify and execute
tplkit exec kubectl-get-pods | sed 's/kubectl/sudo kubectl/' | sh

# Copy to clipboard (with xclip or pbcopy)
tplkit exec kubectl-get-pods | xclip -selection clipboard
```

## Configuration Organization

TplKit supports modular configuration to help organize your templates:

### Single Config File (Default)

The simplest approach - everything in one file:

```yaml
# ~/.config/tplkit/config.yaml
transform_templates:
  k8s-namespace:
    # ... transform rules
    
variable_types:
  port:
    # ... validation rules
    
snippets:
  kubectl-get-pods:
    # ... your templates

settings:
  # ... settings
```

### Modular Configuration with Additional Snippets

For better organization, split snippets into separate files:

```yaml
# ~/.config/tplkit/config.yaml
transform_templates:
  # Shared transform templates
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

variable_types:
  # Shared variable types
  port:
    description: "Network port"
    validation:
      range: [1, 65535]
    default: "8080"

snippets:
  # Core snippets can still go here
  
settings:
  # Load additional snippet files
  additional_snippets:
    - "snippets/kubernetes.yaml"
    - "snippets/docker.yaml"
    - "snippets/git.yaml"
    - "~/my-custom-snippets.yaml"  # Absolute paths work too
```

Then organize your snippets by topic:

```yaml
# ~/.config/tplkit/snippets/kubernetes.yaml
snippets:
  kubectl-describe-pod:
    id: "kubectl-describe-pod"
    description: "Describe a specific pod"
    command: "kubectl describe pod <pod_name> <namespace>"
    variables:
      - name: "pod_name"
        description: "Pod name to describe"
        required: true
      - name: "namespace"
        transformTemplate: "k8s-namespace"  # References main config
    tags: ["kubernetes", "describe"]
```

### Benefits of Modular Organization

- **Team Sharing**: Share topic-specific snippet files across team members
- **Maintainability**: Easier to manage large collections of templates
- **Flexibility**: Mix and match snippet collections for different projects
- **Version Control**: Track changes to specific command categories separately

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
