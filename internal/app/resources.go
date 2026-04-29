package app

import "io/fs"

type EmbeddedResources struct {
	Assets fs.FS
}
