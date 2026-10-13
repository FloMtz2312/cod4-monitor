package watch

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"cod4monitor/internal/api"
	"cod4monitor/internal/mail"
	"cod4monitor/internal/query"
	"cod4monitor/internal/store"
)

// Server is one followed game server, for the mails.
type Server struct {
	ID   int64
	Name string
}

// Weekly sends the summary of the previous week (Monday to Sunday) on Monday from 9:00, once.
type Weekly struct {
	Mail    *mail.Mailer
	Store   *store.Store
	Servers []Server
	List    func() []Server // the servers at the time of the mail (they change from the dashboard); Servers when nil
	Loc     *time.Location
	SiteURL string // https://DOMAIN/, linked at the end and used for the map pictures ("" = none)
}

func (w *Weekly) Run(ctx context.Context) {
	for {
		w.tick(time.Now())
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Minute):
		}
	}
}

func (w *Weekly) tick(now time.Time) {
	from, to := api.LastWeek(now, w.Loc)
	if now.In(w.Loc).Hour() < 9 || w.Store.Meta("weekly_sent") == from.Format("2006-01-02") {
		return
	}
	subject, text, htm, err := w.Build(from, to)
	if err != nil {
		slog.Error("weekly summary", "err", err)
		return
	}
	if deliver(w.Mail, subject, text, htm) == nil {
		w.Store.SetMeta("weekly_sent", from.Format("2006-01-02"))
	}
}

// view of one server in the mail
type weekView struct {
	Name     string
	Empty    bool
	Facts    []fact
	Player   *playerView
	Records  []recordView
	Top      []topView
	Map      string
	MapImg   string
	MapLine  string
	PeakLine string
}

type fact struct{ Value, Label string }
type playerView struct {
	Name   template.HTML
	Detail string
}
type recordView struct {
	Icon, Label, Value string
	Name               template.HTML
}
type topView struct {
	Rank      string
	Name      template.HTML
	Kills, KD string
	Highlight bool
}

// mail colours of the CoD ^0-^9 codes: the dashboard's, darker where they would vanish on white
var codColors = [10]string{"#6b7385", "#e03131", "#2b8a3e", "#c48a00", "#2f6fde", "#0b8fa8", "#c2255c", "#1d2330", "#e8590c", "#868e96"}
var codRe = regexp.MustCompile(`\^([0-9])`)

// mapLabel: mp_crash_snow -> Crash Snow (as on the dashboard)
func mapLabel(m string) string {
	words := strings.Fields(strings.ReplaceAll(strings.TrimPrefix(m, "mp_"), "_", " "))
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// colorName turns a raw CoD name into coloured HTML.
func colorName(raw string) template.HTML {
	var b strings.Builder
	color := codColors[7]
	parts := codRe.Split(raw, -1)
	codes := codRe.FindAllStringSubmatch(raw, -1)
	for i, p := range parts {
		if i > 0 {
			n, _ := strconv.Atoi(codes[i-1][1])
			color = codColors[n]
		}
		if p != "" {
			fmt.Fprintf(&b, `<span style="color:%s">%s</span>`, color, template.HTMLEscapeString(p))
		}
	}
	if b.Len() == 0 {
		return "?"
	}
	return template.HTML(b.String())
}

// Build writes the summary mail of [from, to) for every server.
func (w *Weekly) Build(from, to time.Time) (subject, text, htm string, err error) {
	m := w.Mail
	day := func(t time.Time) string { return t.Format("02/01") }
	period := fmt.Sprintf(m.T("du %s au %s", "from %s to %s"), day(from), day(to.AddDate(0, 0, -1)))
	subject = m.T("📊 Résumé de la semaine ", "📊 Weekly summary ") + period
	var tb strings.Builder
	var views []weekView
	servers := w.Servers
	if w.List != nil {
		servers = w.List()
	}
	for _, s := range servers {
		wk, err := api.WeekSummary(w.Store.DB, strconv.FormatInt(s.ID, 10), from.Unix(), to.Unix())
		if err != nil {
			return "", "", "", err
		}
		v := w.view(s, wk, period)
		views = append(views, v)
		w.text(&tb, v, wk)
	}
	if w.SiteURL != "" {
		fmt.Fprintf(&tb, "%s %s\n", m.T("Tout le détail :", "All the details:"), w.SiteURL)
	}
	var hb bytes.Buffer
	err = weeklyTmpl.Execute(&hb, map[string]any{
		"Kicker": m.T("CoD4 MONITOR · RÉSUMÉ DE LA SEMAINE", "CoD4 MONITOR · WEEKLY SUMMARY"),
		"Period": strings.ToUpper(period[:1]) + period[1:],
		"Views":  views,
		"Site":   w.SiteURL,
		"Button": m.T("Ouvrir le dashboard →", "Open the dashboard →"),
		"Footer": m.T("Envoyé chaque lundi par ton CoD4 Monitor. Pour ne plus le recevoir : WEEKLY_MAIL=0 dans le .env.",
			"Sent every Monday by your CoD4 Monitor. To stop it: WEEKLY_MAIL=0 in the .env."),
		"Top":     m.T("Top 5 des tueurs", "Top 5 killers"),
		"Records": m.T("Records de la semaine", "Records of the week"),
		"Potw":    m.T("🏆 JOUEUR DE LA SEMAINE", "🏆 PLAYER OF THE WEEK"),
		"EmptyT":  m.T("Pas de partie cette semaine", "No games this week"),
		"EmptyD":  fmt.Sprintf(m.T("Personne n'a joué %s.", "Nobody played %s."), period),
	})
	return subject, tb.String(), hb.String(), err
}

func (w *Weekly) view(s Server, wk *api.Week, period string) weekView {
	m := w.Mail
	v := weekView{Name: s.Name, Empty: wk.Players == 0 && wk.Kills == 0}
	if v.Empty {
		return v
	}
	hours := func(secs int64) string {
		if secs < 3600 {
			return fmt.Sprintf("%d min", secs/60)
		}
		return fmt.Sprintf("%d h %02d", secs/3600, secs%3600/60)
	}
	kd := func(k, d int) string { return fmt.Sprintf("%.2f", float64(k)/float64(max(d, 1))) }
	v.Facts = []fact{
		{strconv.Itoa(wk.Players), m.T("joueurs", "players")},
		{hours(wk.PlaySecs), m.T("de jeu", "played")},
		{strconv.Itoa(wk.Peak), m.T("au maximum", "at peak")},
		{strconv.Itoa(wk.Matches), m.T("matchs", "matches")},
		{strconv.Itoa(wk.Kills), m.T("kills", "kills")},
	}
	if wk.Peak > 0 {
		v.PeakLine = fmt.Sprintf(m.T("Pic le %s à %s", "Peak on %s at %s"), time.Unix(wk.PeakAt, 0).In(w.Loc).Format("02/01"), time.Unix(wk.PeakAt, 0).In(w.Loc).Format("15:04"))
	}
	if len(wk.Top) > 0 {
		p := wk.Top[0]
		v.Player = &playerView{colorName(p.Name), fmt.Sprintf(m.T("%d kills · K/D %s · %d headshots", "%d kills · K/D %s · %d headshots"), p.Kills, kd(p.Kills, p.Deaths), p.Headshots)}
	}
	if r := wk.Longest; r != nil {
		v.Records = append(v.Records, recordView{"📏", m.T("Plus long kill", "Longest kill"), fmt.Sprintf("%.0f m", r.Value), colorName(r.Name)})
	}
	if r := wk.Noscopes; r != nil {
		v.Records = append(v.Records, recordView{"🎯", m.T("Noscopes", "Noscopes"), fmt.Sprintf("%.0f", r.Value), colorName(r.Name)})
	}
	if r := wk.Headshots; r != nil {
		v.Records = append(v.Records, recordView{"💀", m.T("Headshots", "Headshots"), fmt.Sprintf("%.0f", r.Value), colorName(r.Name)})
	}
	medals := []string{"🥇", "🥈", "🥉", "4", "5"}
	for i, p := range wk.Top {
		v.Top = append(v.Top, topView{medals[i], colorName(p.Name), strconv.Itoa(p.Kills), kd(p.Kills, p.Deaths), i == 0})
	}
	if wk.Map != "" {
		v.Map = mapLabel(wk.Map)
		v.MapLine = fmt.Sprintf(m.T("Map la plus fréquentée · %.1f joueurs en moyenne", "Most attended map · %.1f players on average"), wk.MapPlayers)
		if w.SiteURL != "" {
			v.MapImg = w.SiteURL + "maps/" + url.PathEscape(wk.Map) + ".jpg" // the dashboard's thumbnails, public
		}
	}
	return v
}

func (w *Weekly) text(tb *strings.Builder, v weekView, wk *api.Week) {
	m := w.Mail
	name := query.StripColors
	fmt.Fprintf(tb, "== %s ==\n", v.Name)
	if v.Empty {
		fmt.Fprintf(tb, "%s\n\n", m.T("Aucun joueur cette semaine.", "No players this week."))
		return
	}
	for _, f := range v.Facts {
		fmt.Fprintf(tb, "%s %s\n", f.Value, f.Label)
	}
	if v.Map != "" {
		fmt.Fprintf(tb, "%s : %s\n", m.T("Map la plus fréquentée", "Most attended map"), v.Map)
	}
	if len(wk.Top) > 0 {
		p := wk.Top[0]
		fmt.Fprintf(tb, "\n%s : %s (%s)\n", m.T("🏆 Joueur de la semaine", "🏆 Player of the week"), name(p.Name), v.Player.Detail)
	}
	for _, r := range v.Records {
		fmt.Fprintf(tb, "%s %s : %s\n", r.Icon, r.Label, r.Value)
	}
	if len(wk.Top) > 1 {
		fmt.Fprintf(tb, "\n%s\n", m.T("Top 5 (kills) :", "Top 5 (kills):"))
		for i, p := range wk.Top {
			fmt.Fprintf(tb, "%d. %s — %d kills, K/D %s\n", i+1, name(p.Name), p.Kills, v.Top[i].KD)
		}
	}
	tb.WriteString("\n")
}

// Email HTML: tables and inline styles only (what mail apps render reliably), 560 px wide at most.
var weeklyTmpl = template.Must(template.New("weekly").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><meta name="color-scheme" content="light"></head>
<body style="margin:0;padding:0;background:#eef0f4">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#eef0f4"><tr><td align="center" style="padding:16px 10px">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;font-family:-apple-system,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#1d2330">
<tr><td style="background:#0f1115;border-radius:14px 14px 0 0;padding:22px 24px">
  <div style="font-size:11px;letter-spacing:.14em;color:#f0a830;font-weight:700">{{.Kicker}}</div>
  <div style="font-size:24px;font-weight:700;color:#ffffff;margin-top:6px">{{.Period}}</div>
</td></tr>
<tr><td style="background:#ffffff;border-radius:0 0 14px 14px;padding:8px 24px 24px">
{{range .Views}}
  <div style="font-size:18px;font-weight:700;margin:20px 0 12px;padding-bottom:8px;border-bottom:3px solid #f0a830">{{.Name}}</div>
  {{if .Empty}}
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0"><tr><td align="center" style="background:#f6f7f9;border-radius:12px;padding:26px 16px">
    <div style="font-size:34px">😴</div>
    <div style="font-size:16px;font-weight:700;margin-top:6px">{{$.EmptyT}}</div>
    <div style="font-size:14px;color:#6b7385;margin-top:4px">{{$.EmptyD}}</div>
  </td></tr></table>
  {{else}}
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0"><tr>
    {{range .Facts}}<td align="center" style="padding:10px 2px;background:#f6f7f9;border:3px solid #ffffff;border-radius:10px">
      <div style="font-size:20px;font-weight:700;white-space:nowrap">{{.Value}}</div><div style="font-size:11px;color:#6b7385">{{.Label}}</div></td>{{end}}
  </tr></table>
  {{if .PeakLine}}<div style="font-size:12px;color:#6b7385;margin:4px 0 0 4px">{{.PeakLine}}</div>{{end}}

  {{with .Player}}
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin-top:16px"><tr><td style="background:#fff6e3;border:2px solid #f0a830;border-radius:12px;padding:16px 18px">
    <div style="font-size:11px;letter-spacing:.12em;color:#9a6a00;font-weight:700">{{$.Potw}}</div>
    <div style="font-size:26px;font-weight:800;margin:4px 0 2px">{{.Name}}</div>
    <div style="font-size:14px;color:#5c6270">{{.Detail}}</div>
  </td></tr></table>
  {{end}}

  {{if .Records}}
  <div style="font-size:11px;letter-spacing:.12em;color:#6b7385;font-weight:700;margin:20px 0 8px">{{$.Records}}</div>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0"><tr>
    {{range .Records}}<td valign="top" style="padding:10px 12px;background:#f6f7f9;border:3px solid #ffffff;border-radius:10px">
      <div style="font-size:12px;color:#6b7385">{{.Icon}} {{.Label}}</div>
      <div style="font-size:18px;font-weight:700;margin:2px 0">{{.Value}}</div>
      <div style="font-size:13px">{{.Name}}</div></td>{{end}}
  </tr></table>
  {{end}}

  {{if .Top}}
  <div style="font-size:11px;letter-spacing:.12em;color:#6b7385;font-weight:700;margin:20px 0 6px">{{$.Top}}</div>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="font-size:15px">
    {{range .Top}}<tr{{if .Highlight}} style="background:#fffaf0"{{end}}>
      <td width="34" align="center" style="padding:8px 4px;border-bottom:1px solid #eceef2;font-size:16px;color:#6b7385">{{.Rank}}</td>
      <td style="padding:8px 6px;border-bottom:1px solid #eceef2;font-weight:600">{{.Name}}</td>
      <td align="right" style="padding:8px 6px;border-bottom:1px solid #eceef2;white-space:nowrap"><b>{{.Kills}}</b> <span style="color:#6b7385;font-size:12px">kills</span></td>
      <td align="right" style="padding:8px 4px;border-bottom:1px solid #eceef2;white-space:nowrap;color:#6b7385;font-size:13px">K/D {{.KD}}</td>
    </tr>{{end}}
  </table>
  {{end}}

  {{if .Map}}
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin-top:20px;background:#0f1115;border-radius:12px"><tr><td style="padding:0">
    {{if .MapImg}}<img src="{{.MapImg}}" width="512" alt="{{.Map}}" style="display:block;width:100%;max-width:512px;height:auto;border-radius:12px 12px 0 0">{{end}}
    <div style="padding:12px 16px"><div style="font-size:18px;font-weight:700;color:#ffffff">🗺 {{.Map}}</div><div style="font-size:13px;color:#a7adba">{{.MapLine}}</div></div>
  </td></tr></table>
  {{end}}
  {{end}}
{{end}}
  {{if .Site}}
  <table role="presentation" cellpadding="0" cellspacing="0" style="margin:26px auto 4px"><tr><td align="center" style="background:#f0a830;border-radius:10px">
    <a href="{{.Site}}" style="display:inline-block;padding:13px 26px;color:#0f1115;font-weight:700;font-size:15px;text-decoration:none">{{.Button}}</a>
  </td></tr></table>
  {{end}}
</td></tr>
<tr><td align="center" style="padding:14px 10px;font-size:12px;color:#8a91a0">{{.Footer}}</td></tr>
</table>
</td></tr></table>
</body></html>`))
