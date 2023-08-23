# shellcheck shell=bash

echo
echo "debug.sh"
echo

function gha_debug {
    echo
    echo "_gha_debug"
    echo "Args: $*"
    echo
    echo "Environment:"
    env | sort
}
export -f gha_debug

echo "debug.sh Functions:"
declare -F
echo
