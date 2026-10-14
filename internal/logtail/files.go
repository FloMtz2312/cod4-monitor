package logtail

import (
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
)

// Entry is a file of a folder of the game server.
type Entry struct {
	Name string
	Size int64
	Mod  time.Time
}

// Files reads and deletes files of the game server (the demos), on its FTP server or on this machine.
type Files interface {
	// Entries lists the files of a folder (not the sub-folders). A missing folder is no error: no files.
	Entries(dir string) ([]Entry, error)
	Open(p string) (io.ReadCloser, error)
	Remove(p string) error
	Close()
}

// Connect opens the files of the server whose logs are in dir (an ftp:// or file:// folder, as for the logs).
func Connect(ctx context.Context, dir *url.URL) (Files, error) {
	s, err := dial(ctx, dir)
	if err != nil {
		return nil, err
	}
	return s.(Files), nil
}

func (s ftpSource) Entries(dir string) ([]Entry, error) {
	es, err := s.c.List(dir)
	if err != nil {
		if strings.Contains(err.Error(), "550") { // no such folder
			return nil, nil
		}
		return nil, err
	}
	var out []Entry
	for _, e := range es {
		if e.Type == ftp.EntryTypeFile && !strings.HasPrefix(e.Name, ".") {
			out = append(out, Entry{Name: path.Base(e.Name), Size: int64(e.Size), Mod: e.Time})
		}
	}
	return out, nil
}

func (s ftpSource) Open(p string) (io.ReadCloser, error) { return s.c.Retr(p) }

func (s ftpSource) Remove(p string) error { return s.c.Delete(p) }

func (localSource) Entries(dir string) ([]Entry, error) {
	f, err := openLocal(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	es, err := f.ReadDir(-1)
	var out []Entry
	for _, e := range es {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if fi, ierr := e.Info(); ierr == nil {
			out = append(out, Entry{Name: e.Name(), Size: fi.Size(), Mod: fi.ModTime()})
		}
	}
	return out, err
}

func (localSource) Open(p string) (io.ReadCloser, error) { return openLocal(p) }

func (localSource) Remove(p string) error {
	if !LocalOK(p) {
		return errOutside
	}
	root, err := os.OpenRoot(LocalRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	return root.Remove(strings.TrimPrefix(strings.TrimPrefix(path.Clean(p), strings.TrimSuffix(LocalRoot, "/")), "/"))
}
