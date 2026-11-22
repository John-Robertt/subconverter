//go:build !webui

package webui

import "io/fs"

// FS 在常规构建中返回 nil，这样 CLI/API 二进制不需要前端构建产物。
func FS() fs.FS {
	return nil
}
