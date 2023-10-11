# shellcheck shell=bash

echo
echo "debug.sh v2.1.0"
echo

function gha_debug {
    # echo
    # echo "gha_debug"
    # echo
    # echo "Script: $0"
    # echo
    # echo "Args: $*"
    # echo
    # echo "Environment:"
    # for name in $(declare -F | awk '{print $3}' | grep 'gha_'); do
    #     unset "$name"
    # done
    # env | sort

    if [[ "$0" == "/start.sh" ]]; then
        echo "Starting GHA debug!"
        nohup gha-debug start \
            --flag /tmp/gha-debug.flag \
            --new-relic-secret /run/secrets/gha-app/new_relic_license_key \
            --gh-app-id-secret /run/secrets/gha-app/github_app_id \
            --gh-app-install-id-secret /run/secrets/gha-app/github_app_installation_id \
            --gh-app-private-key /run/secrets/gha-app/github_app_private_key \
            --debug &> /tmp/gha-debug.log &
        # Wait 1 second for startup
        sleep 1
        # Check our log output
        echo "Log output:"
        ls -lah /tmp
        cat /tmp/gha-debug.log
        echo "Done!"
    fi

    # Sleep to allow for debug checking via the API
    if [[ "$0" == "/end.sh" ]]; then
        echo "Stopping GHA debug!"
        gha-debug stop \
            --flag /tmp/gha-debug.flag \
            --debug
        # Check our log output
        echo "Log output:"
        ls -lah /tmp
        cat /tmp/gha-debug.log
        echo "Done!"
    fi
}
export -f gha_debug
