package watch

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"cod4monitor/internal/mail"
	"cod4monitor/internal/push"
	"cod4monitor/internal/query"
	"cod4monitor/internal/store"
)

// Crash explains why the game server went down or restarted, from the end of its console log, stores it with the
// outage and mails it. Recent / ReadOld are nil when the server has no FTP log folder (the mail then says so).
type Crash struct {
	Mail    *mail.Mailer
	Store   *store.Store
	Server  Server
	Recent  func() []string                             // console lines the monitor has in memory
	ReadOld func(ctx context.Context) ([]string, error) // end of qconsole.log.old (the previous run, after a restart)
	SiteURL string                                      // dashboard linked from the mails ("" = none)
	Push    *push.Sender                                // notifications of the staff (nil = none)
}

const consoleKeep = 40 // console lines stored with the outage and put in the mail

var (
	fatalRe = regexp.MustCompile(`(?i)\bERROR\b|Sys_Error|Exceeded limit|Segmentation|\bsignal\b|SIGSEGV|Hunk_|out of memory|` +
		`G_Spawn|assert|exception|fatal`)
	shutdownRe = regexp.MustCompile(`(?i)Server Shutdown|^\s*\]?\s*quit\s*$|Sys_Quit`)
	errorLine  = regexp.MustCompile(`(?i)^\s*ERROR:\s*`)
)

// Diagnose reads the last console lines of a run: the most telling error line, and whether the server stopped
// normally (shutdown without any error: quit command, host restart).
func Diagnose(lines []string) (reason string, clean bool) {
	if len(lines) > 120 {
		lines = lines[len(lines)-120:]
	}
	shutdown := false
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(query.StripColors(lines[i]))
		if l == "" || strings.Trim(l, "*-= ") == "" {
			continue
		}
		if shutdownRe.MatchString(l) {
			shutdown = true
			continue
		}
		// an "ERROR:" line beats anything else; otherwise keep the last matching line
		if errorLine.MatchString(l) {
			return truncate(l, 300), false
		}
		if reason == "" && fatalRe.MatchString(l) {
			reason = truncate(l, 300)
		}
	}
	return reason, reason == "" && shutdown
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// Hint explains a known crash message in plain words ("" if unknown).
func Hint(m *mail.Mailer, reason string) string {
	r := strings.ToLower(reason)
	switch {
	case strings.Contains(r, "exceeded limit"):
		return m.T("Une limite du moteur a été dépassée (trop de modèles, sons, effets ou matériaux chargés pour cette map avec le mod). Retire des assets du mod ou la map concernée.",
			"An engine limit was exceeded (too many models, sounds, effects or materials loaded for this map with the mod). Remove assets from the mod or that map.")
	case strings.Contains(r, "hunk_") || strings.Contains(r, "out of memory"):
		return m.T("Le serveur a manqué de mémoire (hunk). Souvent une map ou un mod trop lourd.",
			"The server ran out of memory (hunk). Often a map or mod that is too heavy.")
	case strings.Contains(r, "g_spawn"):
		return m.T("Plus d'entités libres : trop d'objets créés en même temps (armes au sol, effets…).",
			"No free entities left: too many objects created at the same time (dropped weapons, effects...).")
	case strings.Contains(r, "segmentation") || strings.Contains(r, "signal") || strings.Contains(r, "sigsegv"):
		return m.T("Le programme du serveur a planté (erreur mémoire). Si ça se répète, note la map et ce qui se passait.",
			"The server program crashed (memory error). If it happens again, note the map and what was going on.")
	case strings.Contains(r, "script"):
		return m.T("Erreur dans un script du mod : la ligne indique le fichier en cause.",
			"Error in a mod script: the line names the file at fault.")
	case strings.Contains(r, "couldn't find") || strings.Contains(r, "could not find") || strings.Contains(r, "can't find"):
		return m.T("Un fichier manque sur le serveur (map, .ff ou .iwd du mod).",
			"A file is missing on the server (map, mod .ff or .iwd).")
	}
	return ""
}

func lastLines(lines []string) []string {
	if len(lines) > consoleKeep {
		return lines[len(lines)-consoleKeep:]
	}
	return lines
}

// Down: the server stopped answering. The console the monitor has read so far is the end of its run.
func (c *Crash) Down(at, lastUp time.Time) {
	var lines []string
	if c.Recent != nil {
		lines = lastLines(c.Recent())
	}
	reason, clean := Diagnose(lines)
	c.save(reason, lines, clean)
	c.notify("Ne répond plus", "Not answering", causeFR(reason, clean), causeEN(reason, clean), true)
	m := c.Mail
	if m == nil {
		return
	}
	a := c.alert("down", m.T("Serveur hors ligne", "Server offline"), m.T("Le serveur ne répond plus.", "The server is not answering."))
	if !lastUp.IsZero() {
		a.Rows = append(a.Rows, alertRow{m.T("Dernière réponse", "Last answer"), when(lastUp)})
	}
	a.Rows = append(a.Rows, alertRow{m.T("Déclaré hors ligne", "Declared offline"), when(at)})
	c.explain(&a, reason, clean, lines, c.Recent != nil)
	sendAlert(m, fmt.Sprintf(m.T("🔴 %s ne répond plus", "🔴 %s is not answering"), c.Server.Name), a)
}

// Up: the server answers again after being declared offline. If it restarted, the end of its previous console log
// (qconsole.log.old) says why it went down.
func (c *Crash) Up(at, downSince time.Time, restarted bool) {
	var lines []string
	if restarted {
		lines = c.oldConsole()
	}
	fr, en := "Coupure", "Outage"
	if !downSince.IsZero() {
		fr, en = "Coupure de "+span(at.Sub(downSince)), "Outage of "+span(at.Sub(downSince))
	}
	if restarted {
		fr, en = fr+", il a redémarré", en+", it restarted"
	}
	if lines != nil {
		reason, clean := Diagnose(lines)
		c.save(reason, lines, clean)
		fr, en = fr+" · "+causeFR(reason, clean), en+" · "+causeEN(reason, clean)
	}
	c.notify("De nouveau en ligne", "Back online", fr, en, false)
	m := c.Mail
	if m == nil {
		return
	}
	intro := m.T("Le serveur répond de nouveau.", "The server answers again.")
	if restarted {
		intro = m.T("Le serveur répond de nouveau. Il a redémarré pendant la coupure.", "The server answers again. It restarted during the outage.")
	}
	a := c.alert("up", m.T("De retour en ligne", "Back online"), intro)
	if !downSince.IsZero() {
		a.Rows = append(a.Rows, alertRow{m.T("Hors ligne depuis", "Offline since"), when(downSince)},
			alertRow{m.T("De retour à", "Back at"), when(at)},
			alertRow{m.T("Durée de la coupure", "Outage length"), human(m, at.Sub(downSince))})
	} else {
		a.Rows = append(a.Rows, alertRow{m.T("De retour à", "Back at"), when(at)})
	}
	if lines != nil {
		reason, clean := Diagnose(lines)
		c.explain(&a, reason, clean, lines, true)
		a.ConsoleT = m.T("Fin de la console avant le redémarrage (qconsole.log.old)", "End of the console before the restart (qconsole.log.old)")
	}
	sendAlert(m, fmt.Sprintf(m.T("✅ %s est de nouveau en ligne", "✅ %s is back online"), c.Server.Name), a)
}

// Restarted: the uptime started over between two polls (crash + automatic restart, or a restart by the host).
// A normal shutdown is only recorded; a crash is mailed.
func (c *Crash) Restarted(at, lastUp time.Time) {
	lines := c.oldConsole()
	reason, clean := Diagnose(lines)
	if err := c.Store.AddRestart(c.Server.ID, lastUp, at, reason, strings.Join(lines, "\n"), clean); err != nil {
		slog.Error("store restart", "err", err)
	}
	if !clean {
		c.notify("A redémarré", "Restarted", "Sans arrêt normal, probablement un crash · "+causeFR(reason, clean),
			"Without a normal shutdown, probably a crash · "+causeEN(reason, clean), true)
	}
	m := c.Mail
	if m == nil || clean {
		return
	}
	a := c.alert("warn", m.T("Redémarrage inattendu", "Unexpected restart"),
		m.T("Le serveur a redémarré sans arrêt normal : probablement un crash suivi d'un redémarrage automatique. Il est de nouveau en ligne.",
			"The server restarted without a normal shutdown: probably a crash followed by an automatic restart. It is back online."))
	a.Rows = []alertRow{{m.T("Dernière réponse avant", "Last answer before"), when(lastUp)}, {m.T("Redémarrage constaté", "Restart seen at"), when(at)}}
	c.explain(&a, reason, clean, lines, c.ReadOld != nil)
	a.ConsoleT = m.T("Fin de la console avant le redémarrage (qconsole.log.old)", "End of the console before the restart (qconsole.log.old)")
	sendAlert(m, fmt.Sprintf(m.T("🔁 %s a redémarré", "🔁 %s restarted"), c.Server.Name), a)
}

// notify the staff of the server: "<server> · <what>", the latest one replacing the previous on the phone.
func (c *Crash) notify(fr, en, bodyFR, bodyEN string, urgent bool) {
	name := query.StripColors(c.Server.Name)
	c.Push.Notify(push.Message{Kind: "down", Server: c.Server.ID, Title: name + " · " + fr, TitleEN: name + " · " + en,
		Body: bodyFR, BodyEN: bodyEN, Path: "etat", Tag: fmt.Sprintf("down-%d", c.Server.ID), Urgent: urgent})
}

func causeFR(reason string, clean bool) string {
	switch {
	case reason != "":
		return "Cause probable : " + reason
	case clean:
		return "Arrêt normal (quit ou redémarrage par l'hébergeur)"
	}
	return "Cause inconnue"
}

func causeEN(reason string, clean bool) string {
	switch {
	case reason != "":
		return "Probable cause: " + reason
	case clean:
		return "Normal shutdown (quit or restart by the host)"
	}
	return "Unknown cause"
}

// span: 45 s, 12 min, 2 h 05 (the same in both languages).
func span(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d s", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	}
	return fmt.Sprintf("%d h %02d", int(d.Hours()), int(d.Minutes())%60)
}

func (c *Crash) oldConsole() []string {
	if c.ReadOld == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	lines, err := c.ReadOld(ctx)
	if err != nil {
		slog.Warn("read qconsole.log.old", "server", c.Server.Name, "err", err)
		return nil
	}
	return lastLines(lines)
}

func (c *Crash) save(reason string, lines []string, clean bool) {
	if err := c.Store.SetOutageCause(c.Server.ID, reason, strings.Join(lines, "\n"), clean); err != nil {
		slog.Error("store outage cause", "err", err)
	}
}

func (c *Crash) alert(tone, kicker, intro string) alert {
	m := c.Mail
	a := alert{Tone: tone, Kicker: kicker, Title: c.Server.Name, Intro: intro,
		ConsoleT: m.T("Dernières lignes de la console", "Last console lines"),
		Footer:   m.T("Envoyé par ton CoD4 Monitor.", "Sent by your CoD4 Monitor.")}
	if c.SiteURL != "" {
		a.Link, a.Button = c.SiteURL, m.T("Ouvrir le dashboard", "Open the dashboard")
	}
	return a
}

// explain adds the probable cause and the end of the console to the alert.
func (c *Crash) explain(a *alert, reason string, clean bool, lines []string, hasLog bool) {
	m := c.Mail
	switch {
	case !hasLog:
		a.Cause = &alertCause{Tone: "unknown", Label: m.T("Cause inconnue", "Unknown cause"),
			Text: m.T("Ce serveur n'a pas de dossier de logs FTP configuré : le monitor ne lit pas sa console.",
				"This server has no FTP log folder set up: the monitor doesn't read its console.")}
	case reason != "":
		a.Cause = &alertCause{Tone: "bad", Label: m.T("Cause probable", "Probable cause"), Text: reason, Hint: Hint(m, reason), Mono: true}
	case clean:
		a.Cause = &alertCause{Tone: "ok", Label: m.T("Arrêt normal", "Normal shutdown"),
			Text: m.T("Commande quit ou redémarrage par l'hébergeur : ce n'est pas un crash.",
				"Quit command or restart by the host: this is not a crash.")}
	default:
		a.Cause = &alertCause{Tone: "unknown", Label: m.T("Cause inconnue", "Unknown cause"),
			Text: m.T("Aucune erreur dans la fin de la console : coupure réseau, arrêt de la machine par l'hébergeur, ou plantage sans message.",
				"No error at the end of the console: network cut, machine stopped by the host, or a crash without any message.")}
	}
	key := strings.TrimSuffix(reason, "…")
	for _, l := range lines {
		l = strings.TrimRight(query.StripColors(l), " \t\r")
		a.Console = append(a.Console, consoleLine{Text: l, Match: key != "" && strings.HasPrefix(strings.TrimSpace(l), key)})
	}
}

// when: 08/10/2026 21:04:05
func when(t time.Time) string { return t.Format("02/01/2006 15:04:05") }

func human(m *mail.Mailer, d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d s", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	}
	return fmt.Sprintf(m.T("%d h %02d", "%dh%02d"), int(d.Hours()), int(d.Minutes())%60)
}
