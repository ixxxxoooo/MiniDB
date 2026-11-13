package ui

import (
	"os"
	"path/filepath"

	"github.com/egoist/mygo"

	"minidb/internal/app"
)

// host serves the application layer's dialogs and shell calls with MyGo.
type host struct {
	win func() *mygo.Window
}

func filters(in []app.FileFilter) []mygo.FileFilter {
	out := make([]mygo.FileFilter, 0, len(in))
	for _, f := range in {
		out = append(out, mygo.FileFilter{Name: f.Name, Extensions: f.Exts})
	}
	return out
}

func (h host) OpenFile(o app.OpenOptions) (string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent: h.win(), Title: o.Title, DefaultPath: o.Directory,
		ShowHiddenFiles: o.ShowHidden, Filters: filters(o.Filters),
	})
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

func (h host) SaveFile(o app.SaveOptions) (string, error) {
	def := o.Filename
	if dir, err := mygo.App.Path(mygo.PathDownloads); err == nil && dir != "" {
		def = filepath.Join(dir, o.Filename)
	} else if home, err := os.UserHomeDir(); err == nil {
		def = filepath.Join(home, o.Filename)
	}
	return mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent: h.win(), Title: o.Title, DefaultPath: def, Filters: filters(o.Filters),
	})
}

func (host) OpenPath(path string) error { return mygo.Shell.OpenPath(path) }
