//go:build webui

package webui

import (
	"embed"
	"io/fs"
)

//go:embed dist
var embeddedDist embed.FS

// FS returns the embedded web/dist filesystem for production single-image
// builds. pnpm web:embed syncs web/dist here before compiling with -tags
// webui.
func FS() fs.FS {
	dist, err := fs.Sub(embeddedDist, "dist")
	if err != nil {
		return nil
	}
	return dist
}
