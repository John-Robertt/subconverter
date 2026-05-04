//go:build !webui

package webui

import "io/fs"

// FS returns nil in regular builds so the CLI/API binary does not require
// frontend build artifacts.
func FS() fs.FS {
	return nil
}
