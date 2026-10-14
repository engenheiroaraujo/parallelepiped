// Package migrations embeds the versioned schema of the index database.
//
// Files are named "<version>_<description>.sql" and are applied in version
// order by platform/database.Open.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
