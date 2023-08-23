#!/bin/bash

echo
echo "end.sh"
echo

# shellcheck disable=SC1090
source <(curl -fsSL https://raw.githubusercontent.com/shakefu/gha-debug/main/debug.sh)

_gha_debug "$@"
