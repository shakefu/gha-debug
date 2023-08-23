FROM ghcr.io/actions/actions-runner:latest

COPY hook.sh /start.sh
COPY hook.sh /end.sh

ENV ACTIONS_RUNNER_HOOK_JOB_STARTED=/start.sh
ENV ACTIONS_RUNNER_HOOK_JOB_COMPLETED=/end.sh
