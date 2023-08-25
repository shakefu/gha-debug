# shellcheck shell=bash

echo
echo "debug.sh v1.0.0"
echo

function gha_debug {
    echo
    echo "gha_debug"
    echo
    echo "Script: $0"
    echo
    echo "Args: $*"
    echo
    echo "Environment:"
    for name in $(declare -F | awk '{print $3}' | grep 'gha_'); do
        unset "$name"
    done
    env | sort
    echo
    echo "Event:"
    echo
    if [[ -f "$GITHUB_EVENT_PATH" ]]; then
        cat "$GITHUB_EVENT_PATH"
    fi
    echo "Debug finished!"
    # env | sort
    # env | awk 'sub(/}/,"}\n")||$1=$1' ORS=''
}
export -f gha_debug
