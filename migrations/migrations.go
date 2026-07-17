// Package migrations embeds the ordered SQL migration files for the DDAG
// metadata database so they can be applied by the migration runner without any
// external tooling.
package migrations

import "embed"

// FS holds every *.sql migration, applied in lexical (numeric) filename order.
//
//go:embed *.sql
var FS embed.FS
