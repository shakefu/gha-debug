#!/bin/bash

echo
echo "start.sh"
echo

# shellcheck disable=SC1090
source <(wget -qO- https://raw.githubusercontent.com/shakefu/gha-debug/main/debug.sh)

env | sort

declare -F

gha_debug "$@"
