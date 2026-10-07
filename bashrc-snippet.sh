#!/usr/bin/env bash
# Source this file from ~/.bashrc after installing CS.
stty -ixon 2>/dev/null
cs-insert-widget() {
    local command exit_status
    command=$(cs)
    exit_status=$?
    if [[ $exit_status -eq 0 && -n "$command" ]]; then
        READLINE_LINE="${READLINE_LINE:0:READLINE_POINT}${command}${READLINE_LINE:READLINE_POINT}"
        READLINE_POINT=$((READLINE_POINT + ${#command}))
    fi
}
bind -x '"\C-s":cs-insert-widget'
