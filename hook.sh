#!/usr/bin/env bash

echo
echo "start.sh"
echo

DEBUG_SHA="main"
DEBUG_SCRIPT="https://raw.githubusercontent.com/shakefu/gha-debug/$DEBUG_SHA/debug.sh"

# shellcheck disable=SC1090
source <(curl -fsSL "$DEBUG_SCRIPT")

main() {
    echo
    echo "main"
    echo

    gha_debug "$@"
}

main "$@"
