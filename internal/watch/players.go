package watch

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"cod4monitor/internal/mail"
	"cod4monitor/internal/query"
	"cod4monitor/internal/store"
)

// Players mails the admin when a player to keep an eye on connects: an admin note tagged "suspect" or
// "surveiller", or a !report not handled yet (last 30 days) whose target matches the name.
// At most one mail per player and server every 6 hours (a J line comes at every map change).
type Players struct {
	Mail    *mail.Mailer
	DB      *sql.DB
	SiteURL string

	mu   sync.Mutex
	last map[string]time.Time
}

const playerCooldown = 6 * time.Hour

// Joined is called for each J line (live).
func (p *Players) Joined(server Server, guid, nameRaw string, now time.Time) {
	if p.Mail == nil {
		return
	}
	clean := query.StripColors(nameRaw)
	key := store.PlayerKey(guid, clean)
	m := p.Mail
	var reasons []string

	var tag, note sql.NullString
	if p.DB.QueryRow(`SELECT tag, note FROM player_notes WHERE server_id=? AND key IN (?, ?) AND tag IN ('suspect', 'surveiller')
		ORDER BY key=? DESC LIMIT 1`, server.ID, key, "n:"+clean, key).Scan(&tag, &note) == nil {
		label := map[string]string{"suspect": m.T("⚠ Suspect", "⚠ Suspect"), "surveiller": m.T("👁 À surveiller", "👁 To watch")}[tag.String]
		r := m.T("Note d'admin : ", "Admin note: ") + label
		if note.String != "" {
			r += " — " + note.String
		}
		reasons = append(reasons, r)
	}
	rs, err := p.DB.Query(`SELECT ts, reporter_name, target, reason FROM reports WHERE server_id=? AND handled_at IS NULL AND ts>=?
		AND length(target)>=3 AND ? LIKE '%'||target||'%' ORDER BY ts DESC LIMIT 5`, server.ID, now.Add(-30*24*time.Hour).Unix(), clean)
	if err == nil {
		for rs.Next() {
			var ts int64
			var reporter, target, reason string
			if rs.Scan(&ts, &reporter, &target, &reason) == nil {
				reasons = append(reasons, fmt.Sprintf(m.T("Signalé le %s par %s (« %s ») : %s", "Reported on %s by %s (\"%s\"): %s"),
					time.Unix(ts, 0).Format("02/01 15:04"), query.StripColors(reporter), target, reason))
			}
		}
		rs.Close()
	}
	if len(reasons) == 0 {
		return
	}

	p.mu.Lock()
	if p.last == nil {
		p.last = map[string]time.Time{}
	}
	id := fmt.Sprint(server.ID) + "|" + key
	if now.Sub(p.last[id]) < playerCooldown {
		p.mu.Unlock()
		return
	}
	p.last[id] = now
	p.mu.Unlock()

	subject := fmt.Sprintf(m.T("👁 %s vient de se connecter sur %s", "👁 %s just joined %s"), clean, server.Name)
	body := strings.Join(reasons, "\n")
	if p.SiteURL != "" {
		body += "\n\n" + m.T("Sa fiche : ", "Profile: ") + p.SiteURL + "#/joueur/" + url.PathEscape(key)
	}
	send(m, subject, body)
}
