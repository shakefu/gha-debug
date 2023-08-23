#!/bin/bash

echo
echo "start.sh"
echo

# shellcheck disable=SC1090
source <(wget -qO- https://raw.githubusercontent.com/shakefu/gha-debug/main/debug.sh)

echo "Environment:"
env | sort
echo

echo "Functions:"
declare -F
echo

echo "Functions with -F:"
typeset -F
echo

gha_debug "$@"
