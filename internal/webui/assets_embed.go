//go:build webui

package webui

import (
	"embed"
	"io/fs"
)

//go:embed dist
var embeddedDist embed.FS

// FS 返回嵌入的 web/dist 文件系统，供生产环境的单镜像构建使用。
// 使用 -tags webui 编译前，pnpm web:embed 会把 web/dist 同步到这里。
func FS() fs.FS {
	dist, err := fs.Sub(embeddedDist, "dist")
	if err != nil {
		return nil
	}
	return dist
}
