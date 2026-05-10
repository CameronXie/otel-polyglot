FROM mcr.microsoft.com/dotnet/sdk:10.0 AS dev

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

ENV PATH="/root/.local/bin:$PATH"
WORKDIR /tmp/build
RUN set -eux \
    # install Claude Code
    && curl -fsSL https://claude.ai/install.sh | bash \
    && rm -rf /tmp/build

FROM dev AS builder

ARG VERSION=dev

WORKDIR /usr/code
COPY src/CsAspNet/*.csproj ./src/CsAspNet/
RUN dotnet restore src/CsAspNet/CsAspNet.csproj
COPY . .
RUN dotnet publish src/CsAspNet/CsAspNet.csproj -c Release -o artifacts/publish -p:VersionPrefix=${VERSION}

FROM mcr.microsoft.com/dotnet/aspnet:10.0 AS runtime

WORKDIR /app
COPY --from=builder /usr/code/artifacts/publish .

ENTRYPOINT ["dotnet", "CsAspNet.dll"]
