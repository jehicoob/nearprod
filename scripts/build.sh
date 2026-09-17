#!/bin/sh
# UI already compiled in the source delivery. Set BUILD_UI=1 after editing TS.
set -eu
cd "$(dirname "$0")/.."
command -v go >/dev/null || { echo 'Instala una versión actualmente soportada de Go.' >&2; exit 2; }
if [ "${BUILD_UI:-0}" = 1 ]; then npm run build:ui; fi
# Keep embedded real-acceptance fixtures identical to the visible examples.
cp -R examples/. internal/acceptanceassets/examples/
mkdir -p bin
TARGET_OS=${GOOS:-$(go env GOOS)}
TARGET_ARCH=${GOARCH:-$(go env GOARCH)}
CGO_ENABLED=0 GOOS="$TARGET_OS" GOARCH="$TARGET_ARCH" go build -trimpath -ldflags='-s -w' -o "bin/nearprod-$TARGET_OS-$TARGET_ARCH" ./cmd/nearprod
printf 'Compilado con: '; go version
printf 'Binario: bin/nearprod-%s-%s\n' "$TARGET_OS" "$TARGET_ARCH"
