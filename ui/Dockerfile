# syntax=docker/dockerfile:1
FROM node:20-alpine

RUN apk add --no-cache docker-cli docker-cli-compose git

ARG PNPM_VERSION=10.34.5
ARG PNPM_LINUX_X64_SHA256=69a535476f407fb0f64427913c2b592ca2ed6bf93f92e2cb82a32349e31f36eb
ARG PNPM_LINUX_ARM64_SHA256=22391362c31edf522fd698d0b43d38fe9228e28d5b5e32124ffd0038fb60f9f0
ARG TARGETARCH

RUN set -eux; \
    arch="${TARGETARCH:-$(uname -m)}"; \
    case "$arch" in \
      amd64|x86_64) asset_arch=x64; expected="$PNPM_LINUX_X64_SHA256" ;; \
      arm64|aarch64) asset_arch=arm64; expected="$PNPM_LINUX_ARM64_SHA256" ;; \
      *) echo "Unsupported architecture: $arch" >&2; exit 1 ;; \
    esac; \
    wget -O /usr/local/bin/pnpm "https://github.com/pnpm/pnpm/releases/download/v${PNPM_VERSION}/pnpm-linux-${asset_arch}"; \
    echo "$expected  /usr/local/bin/pnpm" | sha256sum -c -; \
    chmod 0755 /usr/local/bin/pnpm; \
    pnpm --version | grep -Fx "$PNPM_VERSION"

WORKDIR /app

COPY ui/package*.json ./
COPY ui/pnpm-lock.yaml ./
RUN --mount=type=cache,id=pnpm-store,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile

COPY ui ./
RUN pnpm run build

EXPOSE 5055

CMD ["node", "server.mjs"]
