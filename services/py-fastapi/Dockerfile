FROM python:3.14-trixie AS dev

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

ARG UV_VERSION=0.10.9

ENV PATH="/root/.local/bin:$PATH"
WORKDIR /tmp/build
RUN set -eux \
    # install Claude Code
    && curl -fsSL https://claude.ai/install.sh | bash \
    # install uv \
    && curl -LsSf https://astral.sh/uv/${UV_VERSION}/install.sh | bash \
    # clean up build directory
    && rm -rf /tmp/build

FROM dev AS deps

WORKDIR /usr/code
COPY pyproject.toml uv.lock ./

ENV PATH="/usr/code/.venv/bin:$PATH"
RUN uv sync --all-groups --frozen

FROM deps AS builder

COPY . .
RUN make build

FROM python:3.14-slim AS runtime

WORKDIR /app

# Create non-root user
RUN groupadd -r appgroup && useradd -r -g appgroup appuser

# Copy built package from builder
COPY --from=builder /usr/code/dist /app/dist

# Install the package
RUN pip install --no-cache-dir /app/dist/*.whl

# Switch to non-root user
USER appuser

# Set environment variables
ENV PYTHONUNBUFFERED=1

EXPOSE 8080

ENTRYPOINT ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8080"]
