# Go builder
FROM golang:1.20 AS build

WORKDIR /usr/src/app

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy in the main CLI
COPY *.go ./
# Copy in our support packages
COPY pkg ./pkg/

# Make our build directory and build the CLI
RUN mkdir -p /build
RUN go build -v -o /build/ ./...

# Runner image
FROM ghcr.io/actions/actions-runner:latest

ENV DEBIAN_FRONTEND=noninteractive

USER root
RUN apt-get update -yqq && apt-get install -yqq \
    curl \
    && rm -rf /var/lib/apt/lists/*

USER runner
COPY hook.sh /start.sh
COPY hook.sh /end.sh
COPY --from=build /build/gha-debug /usr/local/bin/gha-debug

ENV ACTIONS_RUNNER_HOOK_JOB_STARTED=/start.sh
ENV ACTIONS_RUNNER_HOOK_JOB_COMPLETED=/end.sh
