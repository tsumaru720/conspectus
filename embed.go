package conspectus

import (
	"embed"
	"io/fs"
)

// Templates and assets are deliberately NOT embedded: they ship next to the
// binary (see CONSPECTUS_WEB_DIR) so they can be edited in place. Only the
// migration chain is baked in.
//
//go:embed migrations
var embedded embed.FS

func EmbeddedMigrations() (fs.FS, error) {
	return fs.Sub(embedded, "migrations")
}
