# shellcheck shell=bash

echo
echo "debug.sh"
echo

function gha_debug {
    echo
    echo "gha_debug"
    echo
    echo "Args: $*"
    echo
    echo "Environment:"
    env | sort
}
export -f gha_debug
