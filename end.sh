#!/bin/bash

echo
echo "end.sh"
echo

# shellcheck disable=SC1090
source <(wget -qO- https://raw.githubusercontent.com/shakefu/gha-debug/main/debug.sh)

gha_debug "$@"
