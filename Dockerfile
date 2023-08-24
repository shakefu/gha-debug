FROM ghcr.io/actions/actions-runner:latest

ENV DEBIAN_FRONTEND=noninteractive

USER root
RUN apt-get update -yqq && apt-get install -yqq \
    curl \
    && rm -rf /var/lib/apt/lists/*

USER runner
COPY hook.sh /start.sh
COPY hook.sh /end.sh

ENV ACTIONS_RUNNER_HOOK_JOB_STARTED=/start.sh
ENV ACTIONS_RUNNER_HOOK_JOB_COMPLETED=/end.sh
