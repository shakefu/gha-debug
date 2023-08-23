#!/usr/bin/env bash

echo
echo "start.sh"
echo

DEBUG_SCRIPT="https://raw.githubusercontent.com/shakefu/gha-debug/main/debug.sh"

# shellcheck disable=SC1090
source <(wget -qO- "$DEBUG_SCRIPT")

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
