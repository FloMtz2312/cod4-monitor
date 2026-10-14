package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"cod4monitor/internal/demos"
	"cod4monitor/internal/logtail"
	"cod4monitor/internal/store"
)

// DemoStatusOf: the recording status of a running server (ok=false: the server isn't running).
type DemoStatusOf func(id int64) (demos.Status, bool)

// LinkDemos keeps the demos that cover the moderation items of a server (reports, suspects, watched players, ban
// evasions, B3 bans): they stay DemoLinkedDays instead of the normal keep time. Run by the demo recorder.
func LinkDemos(st *store.Store, id int64, now time.Time) error {
	items, _, err := inbox(st.DB, st, id, now, true)
	if err != nil {
		return err
	}
	for _, it := range items {
		key, _ := it["key"].(string)
		ts, _ := it["ts"].(int64)
		typ, _ := it["type"].(string)
		if _, err := st.LinkDemos(id, key, ts, typ); err != nil {
			return err
		}
	}
	// players banned from the site
	bs, err := rows(st.DB, `SELECT guid, ts FROM bans WHERE server_id=? AND guid IS NOT NULL AND ts>=?`, id, now.AddDate(0, 0, -7).Unix())
	if err != nil {
		return err
	}
	for _, b := range bs {
		g, _ := b["guid"].(string)
		ts, _ := b["ts"].(int64)
		if _, err := st.LinkDemos(id, g, ts, "ban"); err != nil {
			return err
		}
	}
	return nil
}

// demoSettingsRoutes: the demo settings of a server (owner), with the estimate that guides the keep time.
func demoSettingsRoutes(r *gin.Engine, st *store.Store, rcons RconOf, status DemoStatusOf) {
	statusOf := func(id int64) demos.Status {
		if status == nil {
			return demos.Status{}
		}
		s, _ := status(id)
		return s
	}
	view := func(c *gin.Context, id int64, s store.DemoSettings) {
		now := time.Now()
		ds := statusOf(id)
		u, err := st.DemoUsage(id, now)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		e, err := demos.Calc(st, id, s.MinPlayers, s.KeepHours, s.ReserveGB, ds, now)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		conf, _, _ := st.ServerConf(id)
		c.JSON(200, gin.H{"settings": s, "status": ds, "usage": u, "estimate": e, "default_dir": store.DefaultDemoDir,
			"has_rcon": conf.RconPass != "", "has_logs": conf.LogURL != "", "local": strings.HasPrefix(conf.LogURL, "file:")})
	}
	sid := func(c *gin.Context) (int64, bool) {
		id, err := strconv.ParseInt(c.Param("sid"), 10, 64)
		if err != nil || id <= 0 {
			c.JSON(400, gin.H{"error": "serveur invalide"})
			return 0, false
		}
		return id, true
	}

	r.GET("/api/serverconf/:sid/demos", func(c *gin.Context) {
		id, ok := sid(c)
		if !ok {
			return
		}
		s, err := st.DemoSettings(id)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		view(c, id, s)
	})

	// live estimate while the settings are edited (nothing saved)
	r.GET("/api/serverconf/:sid/demos/estimate", func(c *gin.Context) {
		id, ok := sid(c)
		if !ok {
			return
		}
		minP, _ := strconv.Atoi(c.DefaultQuery("min", "2"))
		keep, _ := strconv.Atoi(c.DefaultQuery("keep", "48"))
		reserve, _ := strconv.ParseFloat(c.DefaultQuery("reserve", "0"), 64)
		e, err := demos.Calc(st, id, min(max(minP, 1), 64), min(max(keep, 1), 24*365), max(reserve, 0), statusOf(id), time.Now())
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, e)
	})

	r.PUT("/api/serverconf/:sid/demos", sameOrigin, func(c *gin.Context) {
		id, ok := sid(c)
		if !ok {
			return
		}
		var s store.DemoSettings
		if c.ShouldBindJSON(&s) != nil {
			c.JSON(400, gin.H{"error": "réglages invalides"})
			return
		}
		s.Dir = strings.TrimSpace(s.Dir)
		switch {
		case s.MinPlayers < 1 || s.MinPlayers > 64:
			c.JSON(400, gin.H{"error": "nombre de joueurs invalide (1 à 64)"})
			return
		case s.KeepHours < 1 || s.KeepHours > 24*365:
			c.JSON(400, gin.H{"error": "durée de conservation invalide (1 heure à 1 an)"})
			return
		case s.ReserveGB < 0 || s.ReserveGB > 100000:
			c.JSON(400, gin.H{"error": "place réservée invalide"})
			return
		case len(s.Dir) > 300 || strings.ContainsAny(s.Dir, "\x00\r\n"):
			c.JSON(400, gin.H{"error": "dossier des démos invalide"})
			return
		}
		conf, found, err := st.ServerConf(id)
		if err != nil || !found {
			c.JSON(404, gin.H{"error": "serveur introuvable"})
			return
		}
		if _, _, err := demos.Folder(conf.LogURL, s); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if s.Enabled && conf.RconPass == "" {
			c.JSON(400, gin.H{"error": "l'enregistrement automatique a besoin du mot de passe rcon du serveur"})
			return
		}
		before, err := st.DemoSettings(id)
		if err == nil {
			err = st.SaveDemoSettings(id, s)
		}
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if before.Enabled && !s.Enabled { // stop at once, even when the recorder doesn't know it was recording
			demos.Stop(rcons.of(id))
		}
		c.Set("audit_server", id)
		c.Set("audit", fmt.Sprintf("%s : démos %s, dès %d joueurs, gardées %d h", conf.Name,
			map[bool]string{true: "activées", false: "désactivées"}[s.Enabled], s.MinPlayers, s.KeepHours))
		view(c, id, s)
	})
}

// demoViewerRoutes serves the cod4-demo viewer (github.com/TiPSYSPiT/cod4-demo, GPL-3.0) from its folder: plain HTML
// and JavaScript that parses the demo in the browser. It opens a server demo with ?demo=<url of the file>.
func demoViewerRoutes(r *gin.Engine, dir string) {
	r.GET("/api/demo-viewer", func(c *gin.Context) { c.JSON(200, gin.H{"installed": viewerInstalled(dir)}) })
	if dir == "" {
		return
	}
	files := http.FileServer(http.Dir(dir))
	r.GET("/demo-viewer/*file", func(c *gin.Context) {
		p := path.Clean("/" + c.Param("file"))
		if fi, err := os.Stat(filepath.Join(dir, filepath.FromSlash(p))); err != nil || fi.IsDir() && p != "/" {
			c.Status(404)
			return
		}
		if p == "/" {
			c.Redirect(302, "/demo-viewer/index.html")
			return
		}
		c.Header("Cache-Control", "public, max-age=3600")
		c.Request.URL.Path = p
		files.ServeHTTP(c.Writer, c.Request)
	})
}

func viewerInstalled(dir string) bool {
	if dir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, "index.html"))
	return err == nil
}

// demoRoutes: the demos of a server (staff with the private data permission): list, file, keep.
func demoRoutes(g *gin.RouterGroup, st *store.Store) {
	g.GET("/demos", func(c *gin.Context) {
		f := store.DemoFilter{Player: c.Query("player")}
		f.From, _ = strconv.ParseInt(c.Query("from"), 10, 64)
		f.To, _ = strconv.ParseInt(c.Query("to"), 10, 64)
		f.Limit, _ = strconv.Atoi(c.Query("limit"))
		ds, err := st.Demos(mustID(c), f)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"demos": ds})
	})

	// the file itself, read from the game server (FTP or this machine) for the demo viewer; the last part of the
	// path is the file name, which the viewer shows and uses for its exports
	g.GET("/demos/:did/file/:name", func(c *gin.Context) {
		id := mustID(c)
		did, _ := strconv.ParseInt(c.Param("did"), 10, 64)
		d, found, err := st.Demo(id, did)
		if err != nil || !found || d.Deleted {
			c.JSON(404, gin.H{"error": "démo introuvable (supprimée ?)"})
			return
		}
		conf, _, err := st.ServerConf(id)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		s, err := st.DemoSettings(id)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		base, dir, err := demos.Folder(conf.LogURL, s)
		if err != nil || base == nil || strings.ContainsAny(d.File, "/\\") {
			c.JSON(404, gin.H{"error": "dossier des démos inconnu"})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Minute)
		defer cancel()
		fs, err := logtail.Connect(ctx, base)
		if err != nil {
			c.JSON(502, gin.H{"error": "serveur de jeu injoignable : " + err.Error()})
			return
		}
		defer fs.Close()
		rd, err := fs.Open(path.Join(dir, d.File))
		if err != nil {
			c.JSON(404, gin.H{"error": "fichier introuvable : " + err.Error()})
			return
		}
		defer rd.Close()
		c.Header("Content-Type", "application/octet-stream")
		c.Header("Content-Disposition", `attachment; filename="`+d.File+`"`)
		c.Header("Cache-Control", "private, no-store")
		if d.Size > 0 && time.Now().Unix()-d.LastWrite > 120 {
			c.Header("Content-Length", strconv.FormatInt(d.Size, 10))
		}
		c.Status(200)
		io.Copy(c.Writer, rd)
	})

	// keep a demo by hand (beyond the keep time), or let it go back to the normal keep time
	g.POST("/demos/:did/keep", sameOrigin, func(c *gin.Context) {
		id := mustID(c)
		did, _ := strconv.ParseInt(c.Param("did"), 10, 64)
		var b struct{ Keep bool }
		if c.ShouldBindJSON(&b) != nil {
			c.JSON(400, gin.H{"error": "requête invalide"})
			return
		}
		d, found, err := st.Demo(id, did)
		if err != nil || !found {
			c.JSON(404, gin.H{"error": "démo introuvable"})
			return
		}
		level, reason := 0, ""
		if b.Keep {
			level, reason = store.DemoManual, "manual"
		}
		if err := st.KeepDemo(id, did, level, reason); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.Set("audit", map[bool]string{true: "démo gardée : ", false: "démo non gardée : "}[b.Keep]+d.File)
		c.JSON(200, gin.H{"ok": true})
	})
}
