# shellcheck shell=bash

_gha_debug() {
    echo
    echo "_gha_debug"
    echo "Args: $*"
    echo
    echo "Environment:"
    env | sort
}
