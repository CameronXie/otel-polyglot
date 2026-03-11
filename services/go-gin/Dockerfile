FROM golang:1.26-trixie AS dev

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
      tree \
      unzip \
      vim \
      zip \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

ARG GOLANGCILINT_VERSION=2.10.1

ENV PATH="/root/.local/bin:$PATH"
WORKDIR /tmp/build
RUN set -eux \
    # install Claude Code
    && curl -fsSL https://claude.ai/install.sh | bash \
    # install golangci-lint
    && curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | \
        sh -s -- -b $(go env GOPATH)/bin v${GOLANGCILINT_VERSION} \
    && go install golang.org/x/vuln/cmd/govulncheck@latest \
    # clean up build directory
    && rm -rf /tmp/build

FROM dev AS builder

WORKDIR /usr/code
COPY . .

RUN make build

FROM gcr.io/distroless/static-debian13:nonroot AS runtime

COPY --from=builder /usr/code/dist/go-gin /bin/go-gin

ENTRYPOINT ["/bin/go-gin"]
