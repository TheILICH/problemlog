// Package migrations embeds the SQL migration files into the Go binary,
// so the Docker image does not need to ship a separate migrations folder.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
