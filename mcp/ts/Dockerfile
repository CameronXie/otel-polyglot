FROM node:24-trixie AS dev

RUN set -eux \
    && apt-get update && apt-get install -y --no-install-recommends \
      curl \
      git \
      jq \
      less \
      make \
      tree \
      vim \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

ARG PNPM_VERSION=10.33.0

ENV PATH="/root/.local/share/pnpm:/root/.local/bin:$PATH"

WORKDIR /tmp/build

RUN set -eux \
     # install Claude Code
    && curl -fsSL https://claude.ai/install.sh | bash \
    # install pnpm
    && curl -fsSL https://get.pnpm.io/install.sh | env PNPM_VERSION=${PNPM_VERSION} SHELL="$(which bash)" bash - \
    # clean up build directory
    && rm -rf /tmp/build
