package watch

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"cod4monitor/internal/b3"
	"cod4monitor/internal/mail"
	"cod4monitor/internal/query"
)

// B3Bans watches the database of the server's B3 admin bot and announces each new ban or temp ban: a live alert on
// the dashboard, and a mail. Only bans given after the monitor started are announced.
type B3Bans struct {
	Server  Server
	Path    string // b3.db
	Every   time.Duration
	Mail    *mail.Mailer // nil: no mail
	SiteURL string
	Publish func([]byte)
}

func (w *B3Bans) Run(ctx context.Context) {
	if w.Every <= 0 {
		w.Every = 20 * time.Second
	}
	last := int64(-1) // unknown until the database could be read once
	t := time.NewTicker(w.Every)
	defer t.Stop()
	for {
		last = w.check(last)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (w *B3Bans) check(last int64) int64 {
	d, err := b3.Open(w.Path)
	if err != nil {
		slog.Warn("b3 database", "server", w.Server.Name, "err", err)
		return last
	}
	defer d.Close()
	if last < 0 {
		id, err := d.LastID()
		if err != nil {
			return last
		}
		return id
	}
	bans, err := d.BansAfter(last, time.Now())
	if err != nil {
		slog.Warn("b3 bans", "server", w.Server.Name, "err", err)
		return last
	}
	for _, b := range bans {
		last = b.ID
		w.announce(b)
	}
	return last
}

func (w *B3Bans) announce(b b3.Penalty) {
	if w.Publish != nil {
		ev, _ := json.Marshal(map[string]any{"type": "b3ban", "server_id": w.Server.ID, "name": b.Name, "penalty": b.Type,
			"reason": b.Reason, "admin": b.Admin, "expire": b.Expire})
		w.Publish(ev)
	}
	if w.Mail == nil {
		return
	}
	m := w.Mail
	name := query.StripColors(b.Name)
	kind, until := m.T("Ban définitif", "Permanent ban"), m.T("jamais", "never")
	if b.Type == b3.TempBan {
		kind, until = m.T("Ban temporaire", "Temporary ban"), time.Unix(b.Expire, 0).Format("02/01/2006 15:04")
	}
	a := alert{Tone: "warn", Kicker: m.T("B3 · sanction", "B3 · penalty"), Title: w.Server.Name,
		Intro: fmt.Sprintf(m.T("%s a été banni.", "%s was banned."), name),
		Rows: []alertRow{
			{m.T("Joueur", "Player"), name},
			{m.T("Sanction", "Penalty"), kind},
			{m.T("Raison", "Reason"), b.Reason},
			{m.T("Par", "By"), b.Admin},
			{m.T("Fin", "Ends"), until},
			{"GUID", b.GUID},
		},
		Footer: m.T("Envoyé par ton CoD4 Monitor.", "Sent by your CoD4 Monitor.")}
	if w.SiteURL != "" {
		a.Link, a.Button = w.SiteURL+"#/moderation", m.T("Voir les sanctions", "See the penalties")
	}
	sendAlert(m, fmt.Sprintf(m.T("⛔ %s banni sur %s", "⛔ %s banned on %s"), name, w.Server.Name), a)
}
