FROM debian:trixie-slim

RUN set -eux \
    && apt-get update && apt-get install -y --no-install-recommends \
      apt-transport-https \
      ca-certificates \
      curl \
      git \
      gnupg \
      jq \
      less \
      lsb-release \
      make \
      shellcheck \
      tree \
      unzip \
      vim \
      zip \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

ARG ACTIONLINT_VERSION=1.7.11

ENV PATH="/root/.local/bin:$PATH"
RUN set -eux \
    # install actionlint with arch detection
    && ARCH=$(dpkg --print-architecture) \
    && case "$ARCH" in \
         amd64) ACTIONLINT_ARCH="linux_amd64" ;; \
         arm64) ACTIONLINT_ARCH="linux_arm64" ;; \
         *) echo "Unsupported architecture: $ARCH" >&2 && exit 1 ;; \
       esac \
    && curl -fsSL -o actionlint.tar.gz "https://github.com/rhysd/actionlint/releases/download/v${ACTIONLINT_VERSION}/actionlint_${ACTIONLINT_VERSION}_${ACTIONLINT_ARCH}.tar.gz" \
    && tar -xzf actionlint.tar.gz \
    && mv actionlint /usr/local/bin \
    && rm actionlint.tar.gz \
    # install Claude Code
    && curl -fsSL https://claude.ai/install.sh | bash
