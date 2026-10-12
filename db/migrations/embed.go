package migrations

import "embed"

// FS exposes versioned SQL migrations to the application and migration tooling.
//
//go:embed *.sql
var FS embed.FS
