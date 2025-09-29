#!/usr/bin/env zsh
# Add this to your ~/.zshrc file

# Disable terminal flow control (Ctrl+S/Ctrl+Q)
# This allows Ctrl+S to be used as a keybinding
stty -ixon 2>/dev/null

# Command snippets widget - runs cs exec and captures stdout
cs-exec-widget() {
    # Run cs exec and capture stdout (TUI displays on stderr)
    local cmd=$(cs exec)
    
    # Check if we got a command (not cancelled)
    if [[ $? -eq 0 && -n "$cmd" ]]; then
        # Insert at current position
        LBUFFER="${LBUFFER}${cmd}"
    fi
    
    # Redraw
    zle redisplay
}

# Register the widget
zle -N cs-exec-widget

# Bind Ctrl+S to the widget
bindkey '^S' cs-exec-widget
