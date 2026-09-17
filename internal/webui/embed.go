package webui

import "embed"

// Files is the compiled React/TypeScript UI, bundled without a Node runtime.
//
//go:embed all:dist
var Files embed.FS
