# CS - Command Snippets

**CS** (Command Snippets) is a powerful CLI tool for managing command templates with intelligent variable substitution. It goes beyond simple snippet storage by providing conditional transformations, reusable template patterns, and smart variable processing.

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
git clone https://github.com/samling/command-snippets.git
cd command-snippets

# Build and install binary + setup config directory
make install

# Or install directly with Go
go install github.com/samling/command-snippets/cmd/cs@latest
```

**Note:** The `snippets/` directory in this repository contains example snippet files that you can use as reference for creating your own templates.

**Want to create your own snippets?** See the **[Snippet Creation Guide](SNIPPET_GUIDE.md)** for comprehensive documentation on creating command templates with variables, transformations, and validation.

### Basic Usage

```bash
# Add your first command template
cs add

# List all templates
cs list

# Execute a template interactively
cs exec kubectl-get-pods

# Search for templates
cs search kubernetes
```

## Shell Integration

CS is designed to integrate seamlessly with your shell workflow. The default behavior outputs clean commands to stdout, making it perfect for shell functions and keybindings.

### Execution Modes

```bash
# Print final command only
cs exec kubectl-get-pods

# Prompt before executing
cs exec kubectl-get-pods --prompt

# Execute automatically without prompting
cs exec kubectl-get-pods --run
```

### Pre-setting Variables

Like Helm, CS supports pre-populating template variables using `--set`:

```bash
# Set single variable
cs exec kubectl-get-pods --set namespace=kube-system

# Set multiple variables
cs exec docker-run --set port=8080 --set image=nginx --set detach=true

# Mix preset and interactive (only prompts for unset variables)
cs exec kubectl-port-forward --set namespace=default
# Will only prompt for pod_name and ports

# Use with automation/scripting
cs exec kubectl-apply --set file=deployment.yaml --run
```

**Benefits of `--set`:**
- **Automation**: Perfect for CI/CD pipelines and scripts
- **Speed**: Skip interactive prompts for known values
- **Flexibility**: Mix preset and interactive variables
- **Validation**: All `--set` values go through the same validation as interactive input
- **Error Handling**: Clear error messages for invalid preset values

### Zsh Keybinding Integration

Create a zsh function to invoke CS with a keybinding (e.g., Ctrl-S) that inserts the generated command directly into your command line:

**Setup:**

Add this to your `~/.zshrc`:

```zsh
# CS integration - Ctrl-S to invoke template selector
function cs-select() {
  RBUFFER=$(cs exec)  # Uses default config: ~/.config/cs/config.yaml
  CURSOR=$#BUFFER
  zle redisplay
}

# Register the function as a zle widget
zle -N cs-select

# Disable terminal flow control (frees up Ctrl-S)
stty -ixon

# Bind Ctrl-S to our function
bindkey '^s' cs-select
```

**Usage:**

1. **Press Ctrl-S** → CS opens with your configured selector (e.g., fzf)
2. **Select a template** → Interactive prompts appear for variables
3. **Fill in variables** → Validation ensures correct input
4. **Command appears** → Generated command is inserted at your cursor position

### External Selector Configuration

CS supports external selectors like fzf, rofi, or dmenu for better template selection:

```yaml
# In your ~/.config/cs/config.yaml
settings:
  selector:
    command: "fzf"
    options: "--height 40% --reverse --border --header='Select template:'"
```

### Bash Integration

For bash users, you can create a similar function:

```bash
# Add to ~/.bashrc
cs-select() {
  local cmd=$(cs exec)  # Uses default config: ~/.config/cs/config.yaml
  if [[ -n "$cmd" ]]; then
    READLINE_LINE="${READLINE_LINE:0:$READLINE_POINT}$cmd${READLINE_LINE:$READLINE_POINT}"
    READLINE_POINT=$((READLINE_POINT + ${#cmd}))
  fi
}

# Bind to Ctrl-S
bind -x '"\C-s": cs-select'
```

### Pipeline Integration

CS's clean stdout makes it perfect for pipelines:

```bash
# Save command to file
cs exec kubectl-get-pods > my-command.sh

# Execute directly
cs exec kubectl-get-pods | sh

# Modify and execute
cs exec kubectl-get-pods | sed 's/kubectl/sudo kubectl/' | sh

# Copy to clipboard (with xclip or pbcopy)
cs exec kubectl-get-pods | xclip -selection clipboard
```

## Configuration Organization

CS supports modular configuration to help organize your templates:

### Single Config File (Default)

The simplest approach - everything in one file:

```yaml
# ~/.config/cs/config.yaml
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

### Modular Configuration

For better organization, split configuration into separate files:

```yaml
# ~/.config/cs/config.yaml
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
  # Load additional configuration files
  additional_configs:
    - "snippets/*.yaml"  # Glob patterns work too
    - "~/my-custom-snippets.yaml"  # Absolute paths work too
```

Then organize your snippets by topic:

```yaml
# ~/.config/cs/snippets/kubernetes.yaml
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

### Local Project Snippets

CS also supports project-specific snippets via `.csnippets` files:

```yaml
# .csnippets (in your project directory)
snippets:
  dev-build:
    description: "Build this project"
    command: "go build -o ./bin/<project_name> ."
    variables:
      - name: "project_name"
        description: "Project binary name"
        default: "myapp"
    tags: ["development", "build"]
  
  dev-test:
    description: "Run project tests with coverage"
    command: "go test -cover ./..."
    tags: ["development", "test"]
```

**How it works:**
- CS automatically looks for `.csnippets` in your current working directory
- Local snippets are loaded in addition to your global configuration
- Local snippets can override global ones (you'll see a warning)
- Perfect for project-specific build, test, and deployment commands
- Can be committed to share with your team or kept local (ignored by default in `.gitignore`)

### Benefits of Modular Organization

- **Team Sharing**: Share topic-specific snippet files across team members
- **Maintainability**: Easier to manage large collections of templates
- **Flexibility**: Mix and match snippet collections for different projects
- **Version Control**: Track changes to specific command categories separately
- **Project Context**: Local `.csnippets` files provide project-specific commands

## Creating Snippets

For comprehensive documentation on creating snippets, see the **[Snippet Creation Guide](SNIPPET_GUIDE.md)**.

The guide covers:
- How to create snippets and variables
- All variable fields and options
- Transformations and transform templates
- Computed variables
- Validation rules
- Advanced examples and best practices

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
      - name: "namespace" # Reference the variable above
        description: "Kubernetes namespace"
        transformTemplate: "k8s-namespace"  # Reference the template
```

**Usage:**
```bash
$ cs exec kubectl-get-pods
namespace (Kubernetes namespace): [Enter]
Executing: kubectl get pods -A

$ cs exec kubectl-get-pods  
namespace (Kubernetes namespace): kube-system
Executing: kubectl get pods -n kube-system
```

This demonstrates explicit variable configuration with reusable transformation templates.

## CLI Commands

### `cs add`
Add a new command template interactively:
```bash
cs add    # Interactive template creation with explicit variable configuration
```

During creation, you'll be prompted to configure each variable found in your command template. You can choose:
- **No transformation**: Simple variable substitution
- **Inline transform**: Custom transformation defined directly
- **Transform template**: Reference to reusable transformation logic

### `cs list`
List and filter templates:
```bash
cs list                  # List all templates (grouped by source)
cs list --tags kubernetes # Filter by tags
cs list --verbose        # Show detailed info
```

The `list` command automatically groups templates by source:
- **Local (project-specific) templates**: Snippets loaded from `.csnippets` in your current directory
- **Global templates**: Snippets from your main config and additional config files

This makes it easy to see which commands are available globally vs just in the current project.

### `cs exec`
Execute templates with interactive prompting:
```bash
cs exec kubectl-get-pods # Execute specific template
cs exec                  # Interactive selection
```

### `cs search`
Search through templates:
```bash
cs search kubectl        # Find templates containing "kubectl"
cs search "get pods"     # Multi-word search
```

### `cs show`
Display configuration components:
```bash
cs show transforms       # Show all transform templates
cs show types           # Show all variable types
cs show config          # Show configuration summary
```

The `show` command helps you understand what building blocks are available:
- **`cs show transforms`**: Display all transform templates with their patterns and logic
- **`cs show types`**: Show variable types with validation rules and defaults  
- **`cs show config`**: Overview of your entire configuration (templates, types, snippets, settings)

This is especially useful when creating new templates or debugging configuration issues.

### `cs describe`
Show detailed information about a template:
```bash
cs describe kubectl-get-pods      # Show template details and variables
cs describe docker-run            # Show validation rules and defaults
```

The `describe` command shows:
- Template description and command pattern
- All variables with their types, validation rules, and defaults
- Computed variables and their composition logic
- Transform templates being used
- Tags for organization

This is perfect for understanding what variables a template expects before running it, especially useful when using `--set` flags or in automation scenarios.

### `cs edit`
Edit templates or configuration:
```bash
cs edit kubectl-get-pods # Edit specific template
cs edit --config         # Edit configuration file
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
   cs add
   # You'll be prompted to configure each variable explicitly
   # You can choose to use transform templates or inline transforms
   ```

3. **Execute with smart prompting:**
   ```bash
   cs exec docker-run
   port_mapping (Port mapping (empty for none)): 8080
   image_name (Docker image name): nginx
   Executing: docker run -p 8080:8080 nginx
   ```