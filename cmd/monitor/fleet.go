package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"cod4monitor/internal/api"
	"cod4monitor/internal/demos"
	"cod4monitor/internal/logtail"
	"cod4monitor/internal/mail"
	"cod4monitor/internal/poller"
	"cod4monitor/internal/push"
	"cod4monitor/internal/query"
	"cod4monitor/internal/store"
	"cod4monitor/internal/watch"
)

// fleet runs the monitoring of every enabled server (status polls, logs, crash reports) and restarts one when its
// settings change from the dashboard, without restarting the monitor.
type fleet struct {
	ctx     context.Context
	st      *store.Store
	hub     *api.Hub
	mailer  *mail.Mailer
	players *watch.Players
	push    *push.Sender

	interval, timeout, logEvery time.Duration
	ftpSession                  time.Duration // FTP sessions renewed after this long (LOG_FTP_SESSION)
	historyFTPMax               int64         // HISTORY_FTP_MAX_MB in bytes, <0 = no history over FTP
	offlineAfter, threshold     int
	loc                         *time.Location
	discord, siteURL            string

	mu  sync.RWMutex
	run map[int64]*running
	wg  sync.WaitGroup
}

type running struct {
	name   string
	cancel context.CancelFunc
	done   chan struct{} // closed when the poller and the log tail have stopped
	rcon   func(string) (string, error)
	tail   *logtail.Tail
	demos  *demos.Recorder
}

// Apply (re)starts a server with its saved settings, or stops it when it is disabled.
func (f *fleet) Apply(id int64) error {
	f.stop(id)
	c, ok, err := f.st.ServerConf(id)
	if err != nil || !ok || !c.Enabled || !c.Managed {
		return err
	}
	return f.start(c)
}

func (f *fleet) stop(id int64) {
	f.mu.Lock()
	r := f.run[id]
	delete(f.run, id)
	f.mu.Unlock()
	if r == nil {
		return
	}
	r.cancel()
	select {
	case <-r.done:
	case <-time.After(10 * time.Second): // an FTP read in progress: it stops by itself afterwards
		slog.Warn("server still stopping", "server", r.name)
	}
	slog.Info("server stopped", "server", r.name)
}

func (f *fleet) start(c store.ServerConf) error {
	addr := net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	var dir *url.URL
	if c.LogURL != "" {
		u, err := url.Parse(c.LogURL)
		if err != nil || u.Scheme != "ftp" && u.Scheme != "ftps" && !(logtail.IsLocal(u) && logtail.LocalOK(u.Path)) {
			return fmt.Errorf("dossier des logs invalide (attendu ftp://user:pass@host:21/mods/<mod>/ ou un dossier dans %s)", logtail.LocalRoot)
		}
		if strings.HasSuffix(u.Path, ".log") { // older config pointed at games_mp.log itself
			u.Path = path.Dir(u.Path)
		}
		dir = u
	}
	loc := f.loc
	if c.Logs.TZ != "" {
		l, err := time.LoadLocation(c.Logs.TZ)
		if err != nil {
			return fmt.Errorf("fuseau horaire inconnu : %s", c.Logs.TZ)
		}
		loc = l
	}
	ctx, cancel := context.WithCancel(f.ctx)
	r := &running{name: c.Name, cancel: cancel, done: make(chan struct{})}
	if c.RconPass != "" {
		pass := c.RconPass
		r.rcon = func(cmd string) (string, error) { return query.Rcon(addr, pass, cmd, 3*time.Second) }
	}
	srv := watch.Server{ID: c.ID, Name: c.Name}
	crash := &watch.Crash{Mail: f.mailer, Store: f.st, Server: srv, SiteURL: f.siteURL, Push: f.push}
	if dir != nil {
		t := &logtail.Tail{ServerID: c.ID, Dir: dir, Files: c.Logs, Every: f.logEvery, SessionMaxAge: f.ftpSession, HistoryFTPMax: f.historyFTPMax, Loc: loc, Rcon: r.rcon, SiteURL: f.siteURL,
			OnJoin: func(guid, nameRaw string) { f.players.Joined(srv, guid, nameRaw, time.Now()) }}
		r.tail = t
		crash.Recent = t.Recent
		crash.ReadOld = func(ctx context.Context) ([]string, error) { return t.ReadEnd(ctx, t.ConsoleOld()) }
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		poller.Run(ctx, poller.Config{ID: c.ID, Name: c.Name, Addr: addr, Interval: f.interval, Timeout: f.timeout,
			DiscordWebhook: f.discord, PlayerThreshold: f.threshold, OfflineAfter: f.offlineAfter,
			OnDown: crash.Down, OnUp: crash.Up, OnRestart: crash.Restarted}, f.st, f.hub.Publish)
	}()
	if r.tail != nil {
		wg.Add(1)
		go func() { defer wg.Done(); r.tail.Run(ctx, f.st, f.hub.Publish) }()
	}
	if c.Logs.B3DB != "" {
		bans := &watch.B3Bans{Server: srv, Path: c.Logs.B3DB, Mail: f.mailer, SiteURL: f.siteURL, Publish: f.hub.Publish}
		wg.Add(1)
		go func() { defer wg.Done(); bans.Run(ctx) }()
	}
	// server-side demos: recorded while enough players are on, listed and cleaned in the server's demos/ folder
	r.demos = &demos.Recorder{ServerID: c.ID, Name: c.Name, Store: f.st, Rcon: r.rcon, LogURL: c.LogURL,
		Link: func(now time.Time) error { return api.LinkDemos(f.st, c.ID, now) }}
	wg.Add(1)
	go func() { defer wg.Done(); r.demos.Run(ctx) }()

	f.wg.Add(1)
	go func() { wg.Wait(); close(r.done); f.wg.Done() }()

	f.mu.Lock()
	f.run[c.ID] = r
	f.mu.Unlock()
	slog.Info("server started", "server", c.Name, "addr", addr, "logs", dir != nil, "rcon", r.rcon != nil)
	return nil
}

// Rcon of a running server (nil: none). The API calls it at every command, so an edited password applies at once.
func (f *fleet) Rcon(id int64) func(string) (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if r := f.run[id]; r != nil {
		return r.rcon
	}
	return nil
}

// DemoStatus: the recording status of a running server's demos.
func (f *fleet) DemoStatus(id int64) (demos.Status, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if r := f.run[id]; r != nil && r.demos != nil {
		return r.demos.Status(), true
	}
	return demos.Status{}, false
}

func (f *fleet) Tail(id int64) *logtail.Tail {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if r := f.run[id]; r != nil {
		return r.tail
	}
	return nil
}

// Servers: the running servers, for the weekly summary.
func (f *fleet) Servers() []watch.Server {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var out []watch.Server
	for id, r := range f.run {
		out = append(out, watch.Server{ID: id, Name: r.name})
	}
	return out
}

func (f *fleet) Count() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.run)
}

// Wait returns once every server has stopped (after the context of the fleet is cancelled).
func (f *fleet) Wait() { f.wg.Wait() }

// importServers copies the SERVERS variable into the database the first time: the servers are then managed from
// the dashboard. SERVERS="Name|host:port[|ftp://user:pass@host:21/mods/<mod>/ or a folder in LOCAL_LOGS_ROOT[|rcon_password]];Other|host:port"
func importServers(st *store.Store, servers string) error {
	for _, entry := range strings.Split(servers, ";") {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		f := append(strings.Split(strings.TrimSpace(entry), "|"), "", "", "")
		name, addr, logURL, rconPass := f[0], f[1], f[2], f[3]
		host, portS, err := net.SplitHostPort(addr)
		port, perr := strconv.Atoi(portS)
		if err != nil || perr != nil || len(f) > 7 {
			return fmt.Errorf("invalid SERVERS address, want Name|host:port")
		}
		if strings.HasPrefix(logURL, "/") { // folder on this VPS
			logURL = (&url.URL{Scheme: "file", Path: path.Clean(logURL) + "/"}).String()
		}
		if logURL != "" {
			if u, err := url.Parse(logURL); err != nil || u.Scheme != "ftp" && u.Scheme != "ftps" && !(logtail.IsLocal(u) && logtail.LocalOK(u.Path)) {
				return fmt.Errorf("invalid log URL for %s, want ftp://user:pass@host:21/mods/<mod>/ or a folder in %s", name, logtail.LocalRoot)
			}
		}
		id, err := st.UpsertServer(name, host, port)
		if err != nil {
			return err
		}
		if _, err := st.SaveServerConf(store.ServerConf{ID: id, Name: name, Host: host, Port: port, LogURL: logURL, RconPass: rconPass, Enabled: true}); err != nil {
			return err
		}
		slog.Info("server imported from SERVERS", "server", name)
	}
	return nil
}
