#!/bin/bash

echo
echo "start.sh"
echo

DEBUG_SCRIPT="https://raw.githubusercontent.com/shakefu/gha-debug/4d4a94752f7d3bb4bde16cb1c0520b315d7ce290/debug.sh"

# shellcheck disable=SC1090
source <(wget -qO- "$DEBUG_SCRIPT")

# echo "Environment:"
# env | sort
# echo
main() {
    echo
    echo "main"
    echo

    echo "Functions:"
    declare -F
    echo

    echo "Functions with -F:"
    typeset -F
    echo

    wget -qO- "$DEBUG_SCRIPT"

    gha_debug "$@"
}

main "$@"
