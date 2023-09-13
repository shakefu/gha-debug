# shellcheck shell=bash

echo
echo "debug.sh v1.3.0"
echo

function gha_debug {
    echo
    echo "gha_debug"
    echo
    echo "Script: $0"
    echo
    echo "Args: $*"
    echo
    echo "Environment:"
    for name in $(declare -F | awk '{print $3}' | grep 'gha_'); do
        unset "$name"
    done
    env | sort

    # This doesn't have state/success/failure info
    # if [[ -f "$GITHUB_EVENT_PATH" ]]; then
    #     echo
    #     echo "Event:"
    #     echo
    #     cat "$GITHUB_EVENT_PATH"
    # fi

    # This appears to be the writable state
    # if [[ -f "$GITHUB_STATE" ]]; then
    #     echo
    #     echo "State:"
    #     echo
    #     cat "$GITHUB_STATE"
    # fi

    # This appears to be the writable human readable step summary
    # if [[ -f "$GITHUB_STEP_SUMMARY" ]]; then
    #     echo
    #     echo "Step summary:"
    #     echo
    #     cat "$GITHUB_STEP_SUMMARY"
    # fi

    echo "Debug finished!"

    # Sleep to allow for debug checking via the API
    if [[ "$0" == "/end.sh" ]]; then
        echo "Sleeping ..."
        sleep 120
        # echo "Getting run state, run_id=$GITHUB_RUN_ID"
        # [[ -n "$GITHUB_TOKEN" ]] || echo "GITHUB_TOKEN not set, trying anyway"
        # gh api "/repos/turo/github-actions-runner-deployments/actions/runs/$GITHUB_RUN_ID/jobs" | jq .
    fi
}
export -f gha_debug
