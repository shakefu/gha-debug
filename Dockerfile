FROM ghcr.io/actions/actions-runner:latest

COPY start.sh /start.sh
COPY end.sh /end.sh

ENV ACTIONS_RUNNER_HOOK_JOB_STARTED=/start.sh
ENV ACTIONS_RUNNER_HOOK_JOB_COMPLETED=/end.sh
