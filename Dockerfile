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


USER root
# TODO: Some of these dependencies are probably better installed via Homebrew,
# but Homebrew Linux is fussy at best...

ENV DEBIAN_FRONTEND=noninteractive

# Dependencies that definitely should be in here... not sure why they're missing
RUN apt-get update -yqq && apt-get install -yqq \
    acl \
    apt-transport-https \
    awscli \
    build-essential \
    ca-certificates \
    curl \
    git \
    jq \
    tree \
    uidmap \
    unzip \
    wget \
    xz-utils \
    zip \
    && rm -rf /var/lib/apt/lists/*

# Dependencies that CI uses
RUN apt-get update -yqq && apt-get install -yqq \
    shellcheck \
    && rm -rf /var/lib/apt/lists/*

# Kubectl apt repository
RUN curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.28/deb/Release.key | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg && \
    echo 'deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.28/deb/ /' > /etc/apt/sources.list.d/kubernetes.list

# Docker apt repository
RUN	curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker-apt-keyring.gpg  && \
    echo "deb [signed-by=/etc/apt/keyrings/docker-apt-keyring.gpg] https://download.docker.com/linux/ubuntu jammy stable" > /etc/apt/sources.list.d/docker.list

# Terraform apt repository
RUN curl -fsSL https://apt.releases.hashicorp.com/gpg | gpg --dearmor -o /etc/apt/keyrings/hashicorp-apt-keyring.gpg && \
    echo "deb [signed-by=/etc/apt/keyrings/hashicorp-apt-keyring.gpg] https://apt.releases.hashicorp.com jammy main" > /etc/apt/sources.list.d/hashicorp.list

# 3rd party apt repositories installs
RUN apt-get update -yqq && apt-get install -yqq \
    docker-ce \
    kubectl \
    terraform \
    && rm -rf /var/lib/apt/lists/*

# Non-apt installs
RUN bash -c 'bash <(curl -fsSL https://raw.githubusercontent.com/rhysd/actionlint/main/scripts/download-actionlint.bash)' && \
    mv actionlint /usr/local/bin/actionlint && \
    chmod 755 /usr/local/bin/actionlint

USER runner
COPY hook.sh /start.sh
COPY hook.sh /end.sh
COPY --from=build /build/gha-debug /usr/local/bin/gha-debug

ENV ACTIONS_RUNNER_HOOK_JOB_STARTED=/start.sh
ENV ACTIONS_RUNNER_HOOK_JOB_COMPLETED=/end.sh
