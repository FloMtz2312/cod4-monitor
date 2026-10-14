// Package watch sends the monitor's emails: VPS health alerts, the weekly summary and player alerts.
package watch

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"cod4monitor/internal/mail"
	"cod4monitor/internal/store"
)

// VPS checks the machine the monitor runs on, every minute: disk, memory, load. An alert mail goes out when a
// problem starts (memory and load must last 5 checks in a row), a reminder every 24 h while it lasts, and a mail
// when it is over. /proc and the disk of the data folder are the host's, even from inside the container.
type VPS struct {
	Mail    *mail.Mailer
	Dir     string  // folder whose disk is checked (the database's)
	DiskPct float64 // used disk above this: alert (VPS_DISK_ALERT, 90)
	MemPct  float64 // used memory above this (VPS_MEM_ALERT, 90)
	Load    float64 // 15-min load per CPU above this (VPS_LOAD_ALERT, 2)
	SiteURL string  // dashboard linked from the mails ("" = none)

	state map[string]*alertState
}

type alertState struct {
	streak   int
	active   bool
	lastSent time.Time
	since    time.Time // first alert of the current problem
}

const (
	sustain  = 5
	reminder = 24 * time.Hour
)

// Reading is one check of the machine; -1 when unknown.
type Reading struct {
	DiskPct    float64 `json:"disk_pct"`
	MemPct     float64 `json:"mem_pct"`
	LoadPerCPU float64 `json:"load_per_cpu"`
}

func Read(dir string) Reading {
	r := Reading{-1, -1, -1}
	if pct, ok := diskPct(dir); ok {
		r.DiskPct = pct
	}
	if b, err := os.ReadFile("/proc/meminfo"); err == nil {
		kb := map[string]float64{}
		for _, l := range strings.Split(string(b), "\n") {
			if k, v, ok := strings.Cut(l, ":"); ok {
				f := strings.Fields(v)
				if len(f) > 0 {
					kb[k], _ = strconv.ParseFloat(f[0], 64)
				}
			}
		}
		if kb["MemTotal"] > 0 {
			r.MemPct = 100 * (1 - kb["MemAvailable"]/kb["MemTotal"])
		}
	}
	if b, err := os.ReadFile("/proc/loadavg"); err == nil {
		if f := strings.Fields(string(b)); len(f) >= 3 {
			if l, err := strconv.ParseFloat(f[2], 64); err == nil {
				r.LoadPerCPU = l / float64(runtime.NumCPU())
			}
		}
	}
	return r
}

func (v *VPS) Run(ctx context.Context) {
	for {
		v.Check(Read(v.Dir), time.Now())
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Minute):
		}
	}
}

// Check compares one reading with the thresholds and sends what has to be sent.
func (v *VPS) Check(r Reading, now time.Time) {
	m := v.Mail
	rows := v.rows(r, now)
	look := m.T("Pour voir ce qui consomme, sur le VPS", "To see what uses it, on the VPS")
	v.check("disk", r.DiskPct >= v.DiskPct, 1, now, rows,
		v.alert(fmt.Sprintf(m.T("Disque du VPS rempli à %.0f %%", "VPS disk %.0f%% full"), r.DiskPct),
			m.T("Disque presque plein", "Disk almost full"),
			m.T("Le disque du VPS est presque plein. Le monitor, la base de données et les sauvegardes risquent de ne plus pouvoir écrire.",
				"The VPS disk is almost full. The monitor, the database and the backups may stop being able to write."),
			m.T("Pour voir ce qui prend de la place, sur le VPS", "To see what takes space, on the VPS"),
			"df -h", "docker system df", m.T("docker system prune   # supprime les images inutilisées", "docker system prune   # removes unused images")),
		v.alert(fmt.Sprintf(m.T("Disque du VPS revenu à %.0f %%", "VPS disk back to %.0f%%"), r.DiskPct),
			m.T("Retour à la normale", "Back to normal"),
			fmt.Sprintf(m.T("Le disque du VPS est repassé sous le seuil d'alerte (%.0f %%).", "The VPS disk is back under the alert threshold (%.0f%%)."), v.DiskPct), ""))
	v.check("mem", r.MemPct >= v.MemPct, sustain, now, rows,
		v.alert(fmt.Sprintf(m.T("Mémoire du VPS utilisée à %.0f %%", "VPS memory %.0f%% used"), r.MemPct),
			m.T("Mémoire presque saturée", "Memory almost full"),
			m.T("La mémoire du VPS est presque saturée depuis plus de 5 minutes. Le serveur peut ralentir, et le système peut arrêter un programme (le monitor ou le serveur de jeu).",
				"The VPS memory has been almost full for more than 5 minutes. The server may slow down, and the system may kill a program (the monitor or the game server)."),
			look, "docker stats", "top"),
		v.alert(fmt.Sprintf(m.T("Mémoire du VPS revenue à %.0f %%", "VPS memory back to %.0f%%"), r.MemPct),
			m.T("Retour à la normale", "Back to normal"),
			fmt.Sprintf(m.T("La mémoire du VPS est repassée sous le seuil d'alerte (%.0f %%).", "The VPS memory is back under the alert threshold (%.0f%%)."), v.MemPct), ""))
	v.check("load", r.LoadPerCPU >= v.Load, sustain, now, rows,
		v.alert(fmt.Sprintf(m.T("Processeur du VPS surchargé (charge %.1f par cœur)", "VPS CPU overloaded (load %.1f per core)"), r.LoadPerCPU),
			m.T("Processeur surchargé", "CPU overloaded"),
			m.T("Le processeur du VPS est surchargé depuis plus de 5 minutes (charge moyenne sur 15 minutes). Les joueurs peuvent ressentir du lag si le serveur de jeu tourne sur ce VPS.",
				"The VPS CPU has been overloaded for more than 5 minutes (15-minute load average). Players may feel lag if the game server runs on this VPS."),
			look, "docker stats", "top"),
		v.alert(fmt.Sprintf(m.T("Processeur du VPS revenu à la normale (%.1f par cœur)", "VPS CPU back to normal (%.1f per core)"), r.LoadPerCPU),
			m.T("Retour à la normale", "Back to normal"),
			fmt.Sprintf(m.T("La charge du processeur est repassée sous le seuil d'alerte (%.1f par cœur).", "The CPU load is back under the alert threshold (%.1f per core)."), v.Load), ""))
}

// rows: the three readings with their thresholds, and the time of the check.
func (v *VPS) rows(r Reading, now time.Time) []alertRow {
	m := v.Mail
	pct := func(x, limit float64) string {
		if x < 0 {
			return m.T("inconnu", "unknown")
		}
		return fmt.Sprintf(m.T("%.0f %% (alerte à %.0f %%)", "%.0f%% (alert at %.0f%%)"), x, limit)
	}
	load := m.T("inconnue", "unknown")
	if r.LoadPerCPU >= 0 {
		load = fmt.Sprintf(m.T("%.1f par cœur (alerte à %.1f)", "%.1f per core (alert at %.1f)"), r.LoadPerCPU, v.Load)
	}
	return []alertRow{{m.T("Disque utilisé", "Disk used"), pct(r.DiskPct, v.DiskPct)},
		{m.T("Mémoire utilisée", "Memory used"), pct(r.MemPct, v.MemPct)},
		{m.T("Charge du processeur", "CPU load"), load},
		{m.T("Mesuré le", "Measured on"), when(now)}}
}

// alert: an alert (cmds given) or a recovery (no cmds) about the VPS.
func (v *VPS) alert(title, kicker, intro, cmdsT string, cmds ...string) alert {
	m := v.Mail
	tone := "warn"
	if len(cmds) == 0 {
		tone = "up"
	}
	a := alert{Tone: tone, Kicker: kicker, Title: title, Intro: intro, CmdsT: cmdsT, Cmds: cmds,
		Footer: m.T("Envoyé par ton CoD4 Monitor. Seuils réglables dans le .env : VPS_DISK_ALERT, VPS_MEM_ALERT, VPS_LOAD_ALERT.",
			"Sent by your CoD4 Monitor. Thresholds in the .env: VPS_DISK_ALERT, VPS_MEM_ALERT, VPS_LOAD_ALERT.")}
	if v.SiteURL != "" {
		a.Link, a.Button = v.SiteURL, m.T("Ouvrir le dashboard", "Open the dashboard")
	}
	return a
}

func (v *VPS) check(key string, bad bool, need int, now time.Time, rows []alertRow, a, ok alert) {
	if v.state == nil {
		v.state = map[string]*alertState{}
	}
	s := v.state[key]
	if s == nil {
		s = &alertState{}
		v.state[key] = s
	}
	if !bad {
		s.streak = 0
		if s.active {
			s.active = false
			ok.Rows = rows
			sendAlert(v.Mail, "✓ "+ok.Title, ok)
		}
		return
	}
	s.streak++
	if s.streak < need {
		return
	}
	if !s.active || now.Sub(s.lastSent) >= reminder {
		if s.active {
			a.Kicker = v.Mail.T("Rappel · toujours en cours", "Reminder · still going on")
			a.Rows = append([]alertRow{{v.Mail.T("Alerte depuis", "Alert since"), when(s.since)}}, rows...)
		} else {
			s.since = now
			a.Rows = rows
		}
		s.active, s.lastSent = true, now
		sendAlert(v.Mail, "⚠ "+a.Title, a)
	}
}

// sendMail is replaced in tests.
var sendMail = func(m *mail.Mailer, subject, text, html string) error { return m.Send(subject, text, html) }

// deliver sends now and logs the outcome.
func deliver(m *mail.Mailer, subject, text, html string) error {
	if m == nil {
		return nil
	}
	err := sendMail(m, subject, text, html)
	if err != nil {
		slog.Error("mail", "subject", subject, "err", err)
	} else {
		slog.Info("mail sent", "subject", subject)
	}
	return err
}

// send delivers in the background (alerts must not block the checks).
func send(m *mail.Mailer, subject, text string) { go deliver(m, subject, text, "") }

// Started records a monitor start. Three starts within 15 minutes mean a restart loop: one mail, at most once an hour.
// It also sends a one-time "mails work" message when the mail settings change, so the setup can be checked
// (synchronous: run it in a goroutine).
func Started(st *store.Store, m *mail.Mailer, siteURL string, now time.Time) {
	var starts []int64
	for _, f := range strings.Split(st.Meta("starts"), ",") {
		if t, err := strconv.ParseInt(f, 10, 64); err == nil && now.Unix()-t < 900 {
			starts = append(starts, t)
		}
	}
	starts = append(starts, now.Unix())
	parts := make([]string, len(starts))
	for i, t := range starts {
		parts[i] = strconv.FormatInt(t, 10)
	}
	st.SetMeta("starts", strings.Join(parts, ","))
	if m == nil {
		return
	}
	monitor := func(tone, kicker, title, intro string) alert {
		a := alert{Tone: tone, Kicker: kicker, Title: title, Intro: intro, Footer: m.T("Envoyé par ton CoD4 Monitor.", "Sent by your CoD4 Monitor.")}
		if siteURL != "" {
			a.Link, a.Button = siteURL, m.T("Ouvrir le dashboard", "Open the dashboard")
		}
		return a
	}
	last, _ := strconv.ParseInt(st.Meta("restart_alert"), 10, 64)
	if len(starts) >= 3 && now.Unix()-last >= 3600 {
		st.SetMeta("restart_alert", strconv.FormatInt(now.Unix(), 10))
		a := monitor("down", m.T("Monitor instable", "Monitor unstable"), m.T("Le monitor redémarre en boucle", "The monitor keeps restarting"),
			m.T("Le monitor plante au démarrage ou juste après. Tant que ce n'est pas réglé, le dashboard et les alertes peuvent être coupés.",
				"The monitor crashes at startup or right after. Until it is fixed, the dashboard and the alerts may be down."))
		a.Rows = []alertRow{{m.T("Démarrages", "Starts"), fmt.Sprintf(m.T("%d en 15 minutes", "%d in 15 minutes"), len(starts))},
			{m.T("Dernier démarrage", "Last start"), when(now)}}
		a.CmdsT = m.T("Pour voir l'erreur, sur le VPS", "To see the error, on the VPS")
		a.Cmds = []string{"docker logs --tail 50 cod4-monitor-monitor-1"}
		sendAlert(m, m.T("⚠ Le monitor redémarre en boucle", "⚠ The monitor keeps restarting"), a)
	}
	if cfg := m.Host + "|" + strings.Join(m.To, ","); st.Meta("mail_cfg") != cfg {
		a := monitor("info", m.T("Configuration", "Setup"), m.T("Les mails fonctionnent", "Emails work"),
			m.T("Ce mail confirme que l'envoi fonctionne. Voici ce que tu recevras à cette adresse :",
				"This email confirms that sending works. Here is what you will get at this address:"))
		weekly := m.T("le lundi matin", "on Monday morning")
		if os.Getenv("WEEKLY_MAIL") == "0" {
			weekly = m.T("désactivé (WEEKLY_MAIL=0)", "off (WEEKLY_MAIL=0)")
		}
		a.Rows = []alertRow{{m.T("Pannes du serveur de jeu", "Game server outages"), m.T("hors ligne, retour, crash", "offline, back, crash")},
			{m.T("Santé du VPS", "VPS health"), m.T("disque, mémoire, processeur", "disk, memory, CPU")},
			{m.T("Joueurs à surveiller", "Players to watch"), m.T("à leur connexion", "when they join")},
			{m.T("Résumé de la semaine", "Weekly summary"), weekly}}
		a.Cause = &alertCause{Tone: "unknown", Label: m.T("Réglages utilisés", "Settings in use"),
			Text: fmt.Sprintf(m.T("Serveur SMTP %s:%d · envoyé à %s", "SMTP server %s:%d · sent to %s"), m.Host, m.Port, strings.Join(m.To, ", "))}
		text, htm := a.render()
		if deliver(m, m.T("✓ Mails du CoD4 Monitor configurés", "✓ CoD4 Monitor emails are set up"), text, htm) == nil {
			st.SetMeta("mail_cfg", cfg) // sent: not again until the settings change; failed: tried again at the next start
		}
	}
}
