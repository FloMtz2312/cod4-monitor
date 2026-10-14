package main

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"cod4monitor/internal/api"
	"cod4monitor/internal/b3"
	"cod4monitor/internal/logtail"
	"cod4monitor/internal/mail"
	"cod4monitor/internal/push"
	"cod4monitor/internal/store"
	"cod4monitor/internal/watch"
	"cod4monitor/ui"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func dur(k, def string) time.Duration {
	d, err := time.ParseDuration(env(k, def))
	if err != nil {
		slog.Error("invalid duration", "var", k, "err", err)
		os.Exit(1)
	}
	return d
}

// backups writes one copy of the database per day (monitor-YYYY-MM-DD.db) and keeps the last `keep`.
func backups(ctx context.Context, st *store.Store, dir string, keep int) {
	keep = max(1, keep)
	for {
		day := time.Now().Format("2006-01-02")
		p := filepath.Join(dir, "monitor-"+day+".db")
		if st.Meta("backup_verified_day") != day {
			err := st.VerifyBackup(p)
			if errors.Is(err, os.ErrNotExist) {
				err = st.Backup(p)
			}
			if err != nil {
				slog.Error("backup restore verification failed", "err", err)
				st.SetMeta("backup_error", "restore verification failed")
			} else {
				st.SetMeta("backup_verified_day", day)
				st.SetMeta("backup_verified_at", strconv.FormatInt(time.Now().Unix(), 10))
				st.SetMeta("backup_error", "")
				slog.Info("backup restored and verified", "path", p)
				if old, _ := filepath.Glob(filepath.Join(dir, "monitor-????-??-??.db")); len(old) > keep {
					for _, f := range old[:len(old)-keep] {
						if e := os.Remove(f); e != nil {
							slog.Error("backup retention", "err", e)
						}
					}
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Hour):
		}
	}
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	gin.SetMode(gin.ReleaseMode)

	interval := dur("POLL_INTERVAL", "15s")
	if interval < 5*time.Second {
		slog.Warn("POLL_INTERVAL below 5s, CoD4X rate-limits queries; using 5s")
		interval = 5 * time.Second
	}
	timeout := dur("QUERY_TIMEOUT", "2s")
	retention, _ := strconv.Atoi(env("RETENTION_DAYS", "90"))
	threshold, _ := strconv.Atoi(env("DISCORD_PLAYER_THRESHOLD", "0"))
	// A map change doesn't answer for ~20-40s: only declare offline after this long without reply.
	offlineAfter := int((dur("OFFLINE_AFTER", "45s") + interval - 1) / interval)

	st, err := store.Open(env("DB_PATH", "monitor.db"))
	if err != nil {
		slog.Error("open db", "err", err)
		os.Exit(1)
	}
	defer st.DB.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	hub := api.NewHub()

	serverLoc, err := time.LoadLocation(env("SERVER_LOG_TZ", "UTC"))
	if err != nil {
		slog.Error("invalid SERVER_LOG_TZ", "err", err)
		os.Exit(1)
	}
	logtail.SetLocalRoot(os.Getenv("LOCAL_LOGS_ROOT")) // logs read on this machine: only inside this folder
	b3.SetRoot(os.Getenv("B3_ROOT"))                   // B3 databases: only inside this folder

	// Emails (SMTP_HOST + MAIL_TO): VPS alerts, weekly summary, player alerts
	mailer := mail.FromEnv()
	siteURL := ""
	if d := os.Getenv("DOMAIN"); d != "" {
		siteURL = "https://" + d + "/"
	}
	go watch.Started(st, mailer, siteURL, time.Now())
	players := &watch.Players{Mail: mailer, DB: st.DB, SiteURL: siteURL}

	// Push notifications of the staff (v2 app): the VAPID key is created the first time and kept in the database.
	// The contact given to the push services: the dashboard's address, else MAIL_FROM.
	subject := strings.TrimSuffix(siteURL, "/")
	if subject == "" {
		subject = "mailto:" + env("MAIL_FROM", env("SMTP_USER", "monitor@localhost"))
	}
	pusher, err := push.New(st, subject)
	if err != nil {
		slog.Error("push notifications off", "err", err)
		pusher = nil
	} else {
		pusher.Access = api.Access(st.DB, os.Getenv("DASHBOARD_USER"), os.Getenv("DASHBOARD_PASSWORD") == "")
	}

	// The servers are managed from the dashboard (Admin tab). SERVERS is only imported the first time.
	if has, err := st.HasManagedServers(); err != nil {
		slog.Error("read servers", "err", err)
		os.Exit(1)
	} else if !has {
		if err := importServers(st, os.Getenv("SERVERS")); err != nil {
			slog.Error("import SERVERS", "err", err)
			os.Exit(1)
		}
	} else if os.Getenv("SERVERS") != "" {
		slog.Info("SERVERS is ignored: the servers are managed from the dashboard (Admin tab)")
	}
	fl := &fleet{ctx: ctx, st: st, hub: hub, mailer: mailer, players: players, push: pusher, interval: interval, timeout: timeout,
		logEvery: dur("LOG_POLL_INTERVAL", "2s"), offlineAfter: offlineAfter, threshold: threshold, loc: serverLoc,
		discord: os.Getenv("DISCORD_WEBHOOK_URL"), siteURL: siteURL, run: map[int64]*running{}}
	confs, err := st.ServerConfs()
	if err != nil {
		slog.Error("read servers", "err", err)
		os.Exit(1)
	}
	for _, c := range confs {
		if c.Managed && c.Enabled {
			if err := fl.Apply(c.ID); err != nil {
				slog.Error("start server", "server", c.Name, "err", err) // the others keep running; fixable from the dashboard
			}
		}
	}

	if mailer != nil {
		pct := func(k string, def float64) float64 {
			if v, err := strconv.ParseFloat(os.Getenv(k), 64); err == nil && v > 0 {
				return v
			}
			return def
		}
		dir := filepath.Dir(env("DB_PATH", "monitor.db"))
		go (&watch.VPS{Mail: mailer, Dir: dir, DiskPct: pct("VPS_DISK_ALERT", 90), MemPct: pct("VPS_MEM_ALERT", 90), Load: pct("VPS_LOAD_ALERT", 2), SiteURL: siteURL}).Run(ctx)
		if os.Getenv("WEEKLY_MAIL") != "0" {
			go (&watch.Weekly{Mail: mailer, Store: st, List: fl.Servers, Loc: time.Local, SiteURL: siteURL}).Run(ctx)
		}
		slog.Info("emails on", "to", mailer.To)
	}

	if retention > 0 {
		go func() {
			for {
				if err := st.Purge(time.Now().AddDate(0, 0, -retention)); err != nil {
					slog.Error("purge", "err", err)
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(24 * time.Hour):
					st.Optimize()
				}
			}
		}()
	}

	go api.RunSchedules(ctx, st, fl.Rcon)
	if pusher != nil {
		go api.RunPushScan(ctx, st, hub, pusher)
	}

	if dir := os.Getenv("BACKUP_DIR"); dir != "" {
		keep, _ := strconv.Atoi(env("BACKUP_KEEP", "14"))
		go backups(ctx, st, dir, keep)
	}

	if os.Getenv("DASHBOARD_PASSWORD") == "" {
		slog.Warn("DASHBOARD_PASSWORD is empty: anyone who can reach the port sees the dashboard (commands stay limited to localhost)")
	}

	proxies := strings.FieldsFunc(env("TRUSTED_PROXIES", "127.0.0.1/32,::1/128"), func(r rune) bool { return r == ',' || r == ' ' })
	if os.Getenv("TRUSTED_PROXIES") == "none" {
		proxies = nil
	}
	uiDist, err := fs.Sub(ui.Dist, "dist") // the interface, its fonts and map images
	if err != nil {
		slog.Error("interface", "err", err)
		os.Exit(1)
	}
	handler := api.Router(api.Config{
		Store: st, Hub: hub, UI: uiDist, PollInterval: interval, Rcon: fl.Rcon, ServerLoc: serverLoc, Servers: fl, Push: pusher,
		DemoStatus: fl.DemoStatus, DemoViewer: os.Getenv("DEMO_VIEWER_DIR"),
		TrustedProxies: proxies,
		User:           os.Getenv("DASHBOARD_USER"), Password: os.Getenv("DASHBOARD_PASSWORD"),
		RecentLogs: func(id int64) []string {
			if t := fl.Tail(id); t != nil {
				return t.Recent()
			}
			return nil
		},
		VPS: func() any { return watch.Read(filepath.Dir(env("DB_PATH", "monitor.db"))) },
		GameDirs: func(ctx context.Context, id int64) ([]string, []string, error) {
			if t := fl.Tail(id); t != nil {
				return t.GameDirs(ctx)
			}
			return nil, nil, nil
		},
	})
	bind := os.Getenv("HTTP_BIND")
	if os.Getenv("DASHBOARD_PASSWORD") == "" {
		bind = "127.0.0.1"
	}
	if os.Getenv("DASHBOARD_PASSWORD") != "" && strings.TrimSpace(os.Getenv("DASHBOARD_USER")) == "" {
		slog.Error("DASHBOARD_USER is required with DASHBOARD_PASSWORD")
		return
	}
	srv := &http.Server{Addr: net.JoinHostPort(bind, env("HTTP_PORT", "8080")), Handler: handler,
		ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 1 << 20,
		BaseContext: func(net.Listener) context.Context { return ctx }} // cancels SSE streams on SIGTERM
	go func() {
		slog.Info("listening", "addr", srv.Addr, "servers", fl.Count(), "interval", interval)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(sctx)
	fl.Wait()
}
