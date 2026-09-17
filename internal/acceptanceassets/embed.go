package acceptanceassets

import "embed"

// Files are isolated application fixtures, never executable host hooks.
//
//go:embed all:examples
var Files embed.FS
