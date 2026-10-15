package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"cod4monitor/internal/logtail"
	"cod4monitor/internal/store"
)

// Imports runs the imports of old log files uploaded from the dashboard, one at a time, in the background, while
// the live logs keep being read.
type Imports struct {
	Store *store.Store
	Dir   string                              // where the uploaded files wait for their import
	Max   int64                               // biggest file accepted, in bytes
	Loc   func(serverID int64) *time.Location // the server's clock (dates of enterleave.log)
	wake  chan struct{}
}

func NewImports(st *store.Store, dir string, max int64, loc func(int64) *time.Location) *Imports {
	return &Imports{Store: st, Dir: dir, Max: max, Loc: loc, wake: make(chan struct{}, 1)}
}

func (im *Imports) poke() {
	select {
	case im.wake <- struct{}{}:
	default:
	}
}

// Run imports the waiting files until ctx ends; imports interrupted by a restart start again.
func (im *Imports) Run(ctx context.Context) {
	if err := im.Store.RequeueImports(); err != nil {
		slog.Error("imports: requeue", "err", err)
	}
	for {
		for {
			j, found, err := im.Store.NextImportJob()
			if err != nil {
				slog.Error("imports: next", "err", err)
				break
			}
			if !found {
				break
			}
			im.process(ctx, j)
			if ctx.Err() != nil {
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-im.wake:
		case <-time.After(time.Minute):
		}
	}
}

func (im *Imports) process(ctx context.Context, j store.ImportJob) {
	log := slog.With("import", j.ID, "server_id", j.ServerID, "file", j.FileName)
	j.State = store.ImportRunning
	im.Store.SaveImportJob(j)
	loc := time.UTC
	if im.Loc != nil {
		loc = im.Loc(j.ServerID)
	}
	open := func() (io.ReadCloser, error) { return os.Open(j.Path) }
	var p logtail.ImportProgress
	last := time.Now()
	report := func() {
		j.Kind, j.Lines, j.Imported, j.Skipped, j.Undated = p.Kind, p.Lines, p.Imported, p.Skipped, p.Undated
		if time.Since(last) > 2*time.Second {
			im.Store.SaveImportJob(j)
			last = time.Now()
		}
	}
	err := func() error {
		f, err := open()
		if err != nil {
			return err
		}
		p.Kind = logtail.DetectKind(f, loc)
		f.Close()
		key := logtail.ImportKey(j.ID)
		switch p.Kind {
		case "enterleave":
			f, err := open()
			if err != nil {
				return err
			}
			defer f.Close()
			return logtail.ImportEnterLeave(ctx, im.Store, j.ServerID, key, loc, f, &p, report)
		case "games":
			return logtail.ImportGames(ctx, im.Store, j.ServerID, key, open, &p, report)
		}
		return logtail.ErrUnknownLog
	}()
	if ctx.Err() != nil {
		return // the monitor stops: the import starts again at the next start
	}
	report()
	j.State, j.FinishedAt = store.ImportDone, time.Now().Unix()
	if err != nil {
		j.State, j.Error = store.ImportFailed, err.Error()
		log.Warn("imports: failed", "err", err)
	} else {
		log.Info("imports: done", "kind", j.Kind, "lines", j.Lines, "imported", j.Imported, "skipped", j.Skipped, "undated", j.Undated)
	}
	if err := im.Store.SaveImportJob(j); err != nil {
		log.Error("imports: save", "err", err)
	}
	os.Remove(j.Path)
}

// uploadOrigin protects the upload (a raw body, not JSON) from other websites: the browser must say where the
// request comes from, and it must be this site; without a login, a local Host only.
func uploadOrigin(c *gin.Context) {
	origin := c.GetHeader("Origin")
	if origin == "" || origin != "http://"+c.Request.Host && origin != "https://"+c.Request.Host ||
		c.GetHeader("Content-Type") != "application/octet-stream" || !c.GetBool("loginmode") && !localHost(c) {
		c.AbortWithStatusJSON(403, gin.H{"error": "forbidden"})
	}
}

// importRoutes: upload an old log file of a server (its owner) and follow its import.
func importRoutes(r *gin.Engine, im *Imports) {
	sid := func(c *gin.Context) (int64, bool) {
		id, err := strconv.ParseInt(c.Param("sid"), 10, 64)
		if err != nil || id <= 0 {
			c.JSON(400, gin.H{"error": "serveur invalide"})
			return 0, false
		}
		return id, true
	}
	r.GET("/api/serverconf/:sid/imports", func(c *gin.Context) {
		id, ok := sid(c)
		if !ok {
			return
		}
		if im == nil {
			c.JSON(200, gin.H{"enabled": false, "imports": []any{}})
			return
		}
		js, err := im.Store.ImportJobs(id, 20)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"enabled": true, "max_bytes": im.Max, "imports": js})
	})

	r.PUT("/api/serverconf/:sid/imports", uploadOrigin, func(c *gin.Context) {
		id, ok := sid(c)
		if !ok {
			return
		}
		if im == nil {
			c.JSON(503, gin.H{"error": "import indisponible sur ce monitor"})
			return
		}
		name := filepath.Base(strings.TrimSpace(c.Query("name")))
		if name == "" || name == "." || len(name) > 120 {
			name = "log"
		}
		conf, found, err := im.Store.ServerConf(id)
		if err != nil || !found {
			c.JSON(404, gin.H{"error": "serveur introuvable"})
			return
		}
		if err := os.MkdirAll(im.Dir, 0o700); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		f, err := os.CreateTemp(im.Dir, fmt.Sprintf("server%d-*.log", id))
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		n, err := io.Copy(f, http.MaxBytesReader(c.Writer, c.Request.Body, im.Max))
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(f.Name())
			var tooBig *http.MaxBytesError
			if errors.As(err, &tooBig) {
				c.JSON(413, gin.H{"error": fmt.Sprintf("fichier trop gros (%d Mo au plus)", im.Max>>20)})
				return
			}
			c.JSON(400, gin.H{"error": "envoi interrompu : " + err.Error()})
			return
		}
		if n == 0 {
			os.Remove(f.Name())
			c.JSON(400, gin.H{"error": "fichier vide"})
			return
		}
		jid, err := im.Store.AddImportJob(id, name, f.Name(), n, c.GetString("user"), time.Now())
		if err != nil {
			os.Remove(f.Name())
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		im.poke()
		c.Set("audit_server", id)
		c.Set("audit", fmt.Sprintf("%s : %s (%d Mo)", conf.Name, name, n>>20))
		c.JSON(200, gin.H{"id": jid})
	})
}
