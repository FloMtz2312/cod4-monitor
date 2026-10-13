// Package poller queries one server on a ticker, stores results, publishes live events and sends Discord alerts.
package poller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"cod4monitor/internal/query"
	"cod4monitor/internal/store"
)

type Config struct {
	ID              int64
	Name, Addr      string
	Interval        time.Duration
	Timeout         time.Duration
	DiscordWebhook  string
	PlayerThreshold int // alert when player count reaches this (0 = off)
	OfflineAfter    int // consecutive failed polls before declaring offline (map loads don't answer)

	// Optional hooks (crash report): called in the background.
	OnDown    func(at, lastUp time.Time)                    // declared offline; lastUp = last answer
	OnUp      func(at, downSince time.Time, restarted bool) // answers again; restarted = its uptime started over
	OnRestart func(at, lastUp time.Time)                    // restarted between two polls, never seen offline
}

// Uptime converts CoD4X's "uptime" server info ("12 minutes", "5 hours", "3 days") to seconds, and the size of
// its unit; ok=false if the server doesn't give it.
func Uptime(v string) (secs, unit int64, ok bool) {
	var n int64
	var u string
	if _, err := fmt.Sscanf(v, "%d %s", &n, &u); err != nil {
		return 0, 0, false
	}
	switch u {
	case "minute", "minutes":
		unit = 60
	case "hour", "hours":
		unit = 3600
	case "day", "days":
		unit = 86400
	default:
		return 0, 0, false
	}
	return n * unit, unit, true
}

// Restarted: the uptime went back compared with what it should be now (prev seen `elapsed` ago).
func Restarted(prev, cur string, elapsed time.Duration) bool {
	p, _, ok1 := Uptime(prev)
	c, unit, ok2 := Uptime(cur)
	return ok1 && ok2 && c+unit < p+int64(elapsed.Seconds())
}

// Run polls until ctx is cancelled. publish receives a JSON event after each poll.
func Run(ctx context.Context, cfg Config, st *store.Store, publish func([]byte)) {
	log := slog.With("server", cfg.Name, "addr", cfg.Addr)
	t := time.NewTicker(cfg.Interval)
	defer t.Stop()
	var wasOnline *bool
	prevPlayers := 0
	var failed []time.Time // failed polls not yet recorded
	var lastUp, downSince time.Time
	var lastUptime string

	for {
		now := time.Now()
		status, err := query.Query(cfg.Addr, cfg.Timeout)
		if err != nil { // one retry: a single lost UDP packet is not an outage
			status, err = query.Query(cfg.Addr, cfg.Timeout)
		}
		online := err == nil
		toRecord := []time.Time{now}
		if !online {
			status = nil
			failed = append(failed, now)
			if (wasOnline == nil || *wasOnline) && len(failed) < cfg.OfflineAfter {
				log.Info("no answer, maybe a map change", "err", err, "fails", len(failed))
				if !wait(ctx, t) {
					return
				}
				continue
			}
			log.Warn("server offline", "err", err)
			toRecord, failed = failed, nil // confirmed: backfill the missed polls
		} else {
			log.Debug("polled", "players", len(status.Players), "map", status.Map, "latency", status.Latency)
			failed = nil // short blip: just a gap in the data
		}
		for _, ts := range toRecord {
			if err := st.Record(cfg.ID, ts, status); err != nil {
				log.Error("store failed", "err", err)
			}
		}

		ev := map[string]any{"type": "status", "server_id": cfg.ID, "ts": now.Unix(), "online": online}
		if online {
			ev["status"] = status
			ev["latency_ms"] = float64(status.Latency.Microseconds()) / 1000
		}
		b, _ := json.Marshal(ev)
		publish(b)

		if online {
			up := status.Vars["uptime"]
			switch {
			case wasOnline != nil && !*wasOnline:
				if cfg.OnUp != nil {
					go cfg.OnUp(now, downSince, lastUptime == "" || Restarted(lastUptime, up, now.Sub(lastUp)))
				}
			case !lastUp.IsZero() && Restarted(lastUptime, up, now.Sub(lastUp)):
				log.Warn("server restarted", "uptime_before", lastUptime, "uptime", up)
				if cfg.OnRestart != nil {
					go cfg.OnRestart(now, lastUp)
				}
			}
			lastUp, lastUptime = now, up
		} else if wasOnline == nil || *wasOnline {
			downSince = toRecord[0]
			if cfg.OnDown != nil && wasOnline != nil {
				go cfg.OnDown(now, lastUp)
			}
		}
		if wasOnline != nil && *wasOnline != online {
			if online {
				notify(cfg, log, fmt.Sprintf("✅ **%s** est de retour en ligne (%s)", cfg.Name, status.Map))
			} else {
				notify(cfg, log, fmt.Sprintf("🔴 **%s** ne répond plus", cfg.Name))
			}
		}
		if online {
			n := len(status.Players)
			if cfg.PlayerThreshold > 0 && n >= cfg.PlayerThreshold && prevPlayers < cfg.PlayerThreshold {
				notify(cfg, log, fmt.Sprintf("🎮 **%s** : %d joueurs connectés sur %s", cfg.Name, n, status.Map))
			}
			prevPlayers = n
		}
		wasOnline = &online

		if !wait(ctx, t) {
			return
		}
	}
}

func wait(ctx context.Context, t *time.Ticker) bool {
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func notify(cfg Config, log *slog.Logger, msg string) {
	if cfg.DiscordWebhook == "" {
		return
	}
	body, _ := json.Marshal(map[string]string{"content": msg})
	c := http.Client{Timeout: 5 * time.Second}
	resp, err := c.Post(cfg.DiscordWebhook, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Error("discord failed", "err", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Error("discord failed", "status", resp.StatusCode)
	}
}
