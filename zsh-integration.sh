#!/usr/bin/env zsh
# ZSH integration for command-snippets (cs) tool
# Add this to your .zshrc or source it

# Function to run cs exec with proper terminal handling
cs-exec-widget() {
    # Save current terminal settings
    local saved_stty=$(stty -g 2>/dev/null)
    
    # Disable flow control to prevent Ctrl+S/Ctrl+Q issues
    stty -ixon 2>/dev/null
    
    # Clear the current line
    zle kill-whole-line
    
    # Run cs exec
    BUFFER="cs exec"
    zle accept-line
    
    # Restore terminal settings (this will run after the command completes)
    trap "stty $saved_stty 2>/dev/null" EXIT INT TERM
}

# Create the ZLE widget
zle -N cs-exec-widget

# Bind Ctrl+S to the widget
# First, we need to disable flow control in the shell
stty -ixon 2>/dev/null

# Then bind the key
bindkey '^S' cs-exec-widget

# Alternative: If you want to keep the command in the buffer for editing
cs-exec-buffer() {
    # Disable flow control temporarily
    stty -ixon 2>/dev/null
    
    # Insert the command at the cursor
    LBUFFER="${LBUFFER}cs exec "
}

# Create alternative widget
zle -N cs-exec-buffer

# You could bind this to a different key if preferred
# bindkey '^X^S' cs-exec-buffer

# Optional: Function to run with arguments
cs-quick() {
    local saved_stty=$(stty -g 2>/dev/null)
    stty -ixon 2>/dev/null
    cs exec "$@"
    stty $saved_stty 2>/dev/null
}

# Optional: Alias for quick access
alias cse='cs-quick'
