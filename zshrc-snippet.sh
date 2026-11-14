#!/usr/bin/env zsh
# shellcheck disable=SC1071
# Source this file from ~/.zshrc after installing CS.
stty -ixon 2>/dev/null
cs-insert-widget() {
    local command exit_status
    command=$(cs)
    exit_status=$?
    if [[ $exit_status -eq 0 && -n "$command" ]]; then
        LBUFFER="${LBUFFER}${command}"
    fi
    zle redisplay
}
zle -N cs-insert-widget
bindkey '^S' cs-insert-widget
