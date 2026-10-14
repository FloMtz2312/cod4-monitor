package logtail

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/textproto"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
)

// LocalRoot is the only local folder the logs can be read from (LOCAL_LOGS_ROOT): the game server's files when it runs
// on the same machine. With Docker, docker-compose.local-logs.yml mounts them there.
var LocalRoot = "/gamelogs/"

// SetLocalRoot changes LocalRoot ("" keeps the default).
func SetLocalRoot(p string) {
	if p = strings.TrimSpace(p); p != "" {
		LocalRoot = strings.TrimSuffix(path.Clean(p), "/") + "/"
	}
}

// source is where the logs are read: an FTP server, or a folder on this machine.
type source interface {
	// Size returns the size of a file, ok=false when it does not exist.
	Size(p string) (size int64, ok bool, err error)
	ReadAt(p string, off, n int64) ([]byte, error)
	// List returns the names of the folders and of the files of a folder.
	List(p string) (dirs, files []string, err error)
	Close()
}

// IsLocal tells whether a log URL is a folder on this machine (file:///<folder>/).
func IsLocal(u *url.URL) bool { return u.Scheme == "file" }

// LocalOK tells whether a local log folder is allowed: inside LocalRoot.
func LocalOK(p string) bool {
	return path.IsAbs(p) && strings.HasPrefix(strings.TrimSuffix(path.Clean(p), "/")+"/", LocalRoot)
}

func (t *Tail) connect(ctx context.Context) (source, error) { return dial(ctx, t.Dir) }

// dial opens the log folder dir: an FTP server, or a folder on this machine inside LocalRoot.
func dial(ctx context.Context, dir *url.URL) (source, error) {
	if IsLocal(dir) {
		if !LocalOK(dir.Path) {
			return nil, errors.New("dossier local refusé : il doit être dans " + LocalRoot)
		}
		if fi, err := os.Stat(dir.Path); err != nil || !fi.IsDir() {
			return nil, errors.New("dossier introuvable sur le VPS : " + dir.Path)
		}
		return localSource{}, nil
	}
	host := dir.Host
	if dir.Port() == "" {
		host += ":21"
	}
	options := []ftp.DialOption{ftp.DialWithContext(ctx), ftp.DialWithTimeout(10 * time.Second)}
	if dir.Scheme == "ftps" {
		options = append(options, ftp.DialWithExplicitTLS(&tls.Config{MinVersion: tls.VersionTLS12, ServerName: dir.Hostname()}))
	}
	c, err := ftp.Dial(host, options...)
	if err != nil {
		return nil, err
	}
	pass, _ := dir.User.Password()
	if err := c.Login(dir.User.Username(), pass); err != nil {
		c.Quit()
		return nil, err
	}
	return ftpSource{c}, nil
}

type ftpSource struct{ c *ftp.ServerConn }

func (s ftpSource) Size(p string) (int64, bool, error) {
	size, err := s.c.FileSize(p)
	var te *textproto.Error
	if errors.As(err, &te) && te.Code == ftp.StatusFileUnavailable {
		return 0, false, nil
	}
	return size, err == nil, err
}

func (s ftpSource) ReadAt(p string, off, n int64) ([]byte, error) {
	r, err := s.c.RetrFrom(p, uint64(off))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(io.LimitReader(r, n))
}

func (s ftpSource) List(p string) (dirs, files []string, err error) {
	entries, err := s.c.List(p)
	for _, e := range entries {
		switch {
		case strings.HasPrefix(e.Name, "."):
		case e.Type == ftp.EntryTypeFolder:
			dirs = append(dirs, e.Name)
		case e.Type == ftp.EntryTypeFile:
			files = append(files, e.Name)
		}
	}
	return dirs, files, err
}

func (s ftpSource) Close() { s.c.Quit() }

// localSource reads files inside LocalRoot only.
type localSource struct{}

var errOutside = errors.New("chemin refusé : il doit être dans le dossier LOCAL_LOGS_ROOT")

func (localSource) Size(p string) (int64, bool, error) {
	if !LocalOK(p) {
		return 0, false, errOutside
	}
	f, err := openLocal(p)
	if errors.Is(err, os.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if errors.Is(err, os.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return fi.Size(), true, nil
}

func (localSource) ReadAt(p string, off, n int64) ([]byte, error) {
	if !LocalOK(p) {
		return nil, errOutside
	}
	f, err := openLocal(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b := make([]byte, n)
	m, err := f.ReadAt(b, off)
	if errors.Is(err, io.EOF) {
		err = nil
	}
	return b[:m], err
}

func (localSource) List(p string) (dirs, files []string, err error) {
	if !LocalOK(p) {
		return nil, nil, errOutside
	}
	f, err := openLocal(p)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	entries, err := f.ReadDir(-1)
	for _, e := range entries {
		switch {
		case strings.HasPrefix(e.Name(), "."):
		case e.IsDir():
			dirs = append(dirs, e.Name())
		default:
			files = append(files, e.Name())
		}
	}
	return dirs, files, err
}

func (localSource) Close() {}

func openLocal(p string) (*os.File, error) {
	if !LocalOK(p) {
		return nil, errOutside
	}
	root, err := os.OpenRoot(LocalRoot)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	relative := strings.TrimPrefix(path.Clean(p), strings.TrimSuffix(LocalRoot, "/"))
	relative = strings.TrimPrefix(relative, "/")
	if relative == "" {
		relative = "."
	}
	return root.Open(relative)
}
