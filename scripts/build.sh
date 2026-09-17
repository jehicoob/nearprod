#!/bin/sh
# Native developer build. BUILD_UI=1 rebuilds TypeScript first.
set -eu
cd "$(dirname "$0")/.."
command -v go >/dev/null || { echo 'Instala Go para compilar NearProd.' >&2; exit 2; }
VERSION=${VERSION:-$(cat VERSION)}
if ! printf '%s\n' "$VERSION" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$'; then
  echo 'Versión inválida; usa X.Y.Z o X.Y.Z-sufijo.' >&2; exit 2
fi
if [ "${BUILD_UI:-0}" = 1 ]; then npm run build:ui; fi
# Preserve the existing acceptance fixtures and private resource identities.
cp -R examples/. internal/acceptanceassets/examples/
mkdir -p bin
TARGET_OS=${GOOS:-$(go env GOOS)}
TARGET_ARCH=${GOARCH:-$(go env GOARCH)}
case "$TARGET_OS/$TARGET_ARCH" in
  darwin/arm64|darwin/amd64|linux/arm64|linux/amd64) ;;
  *) echo "Plataforma no soportada: $TARGET_OS/$TARGET_ARCH" >&2; exit 2 ;;
esac
MODULE=$(go list -m)
CGO_ENABLED=0 GOOS="$TARGET_OS" GOARCH="$TARGET_ARCH" go build \
  -trimpath -buildvcs=false \
  -ldflags="-s -w -buildid= -X $MODULE/internal/nearprod.Version=$VERSION" \
  -o "bin/nearprod-$TARGET_OS-$TARGET_ARCH" ./cmd/nearprod
printf 'Compilado con: '; go version
printf 'Binario: bin/nearprod-%s-%s (%s)\n' "$TARGET_OS" "$TARGET_ARCH" "$VERSION"
