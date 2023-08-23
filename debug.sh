# shellcheck shell=bash

function gha_debug {
    echo
    echo "_gha_debug"
    echo "Args: $*"
    echo
    echo "Environment:"
    env | sort
}
export -f gha_debug
