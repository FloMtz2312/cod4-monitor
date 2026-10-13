package watch

import (
	"bytes"
	"html/template"
	"log/slog"
	"strings"

	"cod4monitor/internal/mail"
)

// alert is one alert mail (server down or back, VPS health, monitor restarts), written both as plain text and as
// HTML in the style of the weekly summary.
type alert struct {
	Tone     string // "down", "up", "warn" or "info": colour of the header
	Kicker   string // small caps line above the title
	Title    string // the server name
	Intro    string
	Rows     []alertRow
	Cause    *alertCause
	Console  []consoleLine
	ConsoleT string // title of the console block
	Cmds     []string
	CmdsT    string // title of the commands block ("what to do")
	Link     string // dashboard URL ("" = no button)
	Button   string
	Footer   string
}

type alertRow struct{ Label, Value string }

type alertCause struct {
	Tone  string // "bad", "ok" or "unknown"
	Label string
	Text  string
	Hint  string
	Mono  bool // Text is a console line
}

type consoleLine struct {
	Text  string
	Match bool // the line the cause comes from
}

var toneColors = map[string]struct{ Accent, Soft, Ink string }{
	"down":    {"#e03131", "#fff0f0", "#a51d1d"},
	"up":      {"#2b8a3e", "#ebfbee", "#1e6b2f"},
	"warn":    {"#f08c00", "#fff6e3", "#9a6a00"},
	"info":    {"#2f6fde", "#edf3ff", "#1d4fa8"},
	"bad":     {"#e03131", "#fff0f0", "#a51d1d"},
	"ok":      {"#2b8a3e", "#ebfbee", "#1e6b2f"},
	"unknown": {"#868e96", "#f6f7f9", "#5c6270"},
}

// sendAlert writes the alert and delivers it in the background.
func sendAlert(m *mail.Mailer, subject string, a alert) {
	if m == nil {
		return
	}
	text, htm := a.render()
	go deliver(m, subject, text, htm)
}

func (a alert) render() (text, htm string) {
	var t strings.Builder
	t.WriteString(a.Title + " — " + a.Kicker + "\n")
	t.WriteString(strings.Repeat("=", len([]rune(a.Title+" — "+a.Kicker))) + "\n\n")
	t.WriteString(a.Intro + "\n")
	if len(a.Rows) > 0 {
		t.WriteString("\n")
		w := 0
		for _, r := range a.Rows {
			w = max(w, len([]rune(r.Label)))
		}
		for _, r := range a.Rows {
			t.WriteString("  " + r.Label + strings.Repeat(" ", w-len([]rune(r.Label))) + " : " + r.Value + "\n")
		}
	}
	if c := a.Cause; c != nil {
		t.WriteString("\n" + c.Label + "\n  " + c.Text + "\n")
		if c.Hint != "" {
			t.WriteString("  → " + c.Hint + "\n")
		}
	}
	if len(a.Console) > 0 {
		t.WriteString("\n" + a.ConsoleT + "\n" + strings.Repeat("-", len([]rune(a.ConsoleT))) + "\n")
		for _, l := range a.Console {
			mark := "  "
			if l.Match {
				mark = "> "
			}
			t.WriteString(mark + l.Text + "\n")
		}
	}
	if len(a.Cmds) > 0 {
		t.WriteString("\n" + a.CmdsT + "\n")
		for _, c := range a.Cmds {
			t.WriteString("  $ " + c + "\n")
		}
	}
	if a.Link != "" {
		t.WriteString("\n" + a.Button + " : " + a.Link + "\n")
	}
	t.WriteString("\n-- \n" + a.Footer + "\n")

	var h bytes.Buffer
	if err := alertTmpl.Execute(&h, struct {
		alert
		Colors map[string]struct{ Accent, Soft, Ink string }
	}{a, toneColors}); err != nil {
		slog.Error("alert mail html", "err", err)
		return t.String(), ""
	}
	return t.String(), h.String()
}

// Email HTML: tables and inline styles only (what mail apps render reliably), 560 px wide at most.
var alertTmpl = template.Must(template.New("alert").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><meta name="color-scheme" content="light"></head>
<body style="margin:0;padding:0;background:#eef0f4">
{{$tone := index .Colors .Tone}}
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#eef0f4"><tr><td align="center" style="padding:16px 10px">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:560px;font-family:-apple-system,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#1d2330">
<tr><td style="background:#0f1115;border-radius:14px 14px 0 0;border-top:5px solid {{$tone.Accent}};padding:20px 24px">
  <div style="font-size:11px;letter-spacing:.14em;color:{{$tone.Accent}};font-weight:700;text-transform:uppercase">● {{.Kicker}}</div>
  <div style="font-size:24px;font-weight:700;color:#ffffff;margin-top:6px">{{.Title}}</div>
</td></tr>
<tr><td style="background:#ffffff;border-radius:0 0 14px 14px;padding:20px 24px 24px">
  <div style="font-size:15px;line-height:1.5">{{.Intro}}</div>

  {{if .Rows}}
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin-top:16px;font-size:14px">
    {{range .Rows}}<tr>
      <td style="padding:9px 12px 9px 0;border-bottom:1px solid #eceef2;color:#6b7385;white-space:nowrap">{{.Label}}</td>
      <td align="right" style="padding:9px 0;border-bottom:1px solid #eceef2;font-weight:600">{{.Value}}</td>
    </tr>{{end}}
  </table>
  {{end}}

  {{with .Cause}}{{$c := index $.Colors .Tone}}
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="margin-top:18px"><tr><td style="background:{{$c.Soft}};border-left:4px solid {{$c.Accent}};border-radius:8px;padding:14px 16px">
    <div style="font-size:11px;letter-spacing:.12em;color:{{$c.Ink}};font-weight:700;text-transform:uppercase">{{.Label}}</div>
    {{if .Mono}}<div style="font-family:Menlo,Consolas,'Courier New',monospace;font-size:13px;font-weight:700;margin-top:6px;word-break:break-word">{{.Text}}</div>
    {{else}}<div style="font-size:14px;line-height:1.5;margin-top:6px">{{.Text}}</div>{{end}}
    {{if .Hint}}<div style="font-size:14px;line-height:1.5;color:#3d4350;margin-top:8px">💡 {{.Hint}}</div>{{end}}
  </td></tr></table>
  {{end}}

  {{if .Console}}
  <div style="font-size:11px;letter-spacing:.12em;color:#6b7385;font-weight:700;margin:22px 0 8px;text-transform:uppercase">{{.ConsoleT}}</div>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="table-layout:fixed"><tr><td style="background:#0f1115;border-radius:10px;padding:12px 14px">
    <div style="font-family:Menlo,Consolas,'Courier New',monospace;font-size:12px;line-height:1.55;color:#c9ced8;word-break:break-all">
    {{range .Console}}{{if .Match}}<div style="color:#ff8787;background:#2a1416;font-weight:700">{{.Text}}</div>{{else}}<div>{{if .Text}}{{.Text}}{{else}}&nbsp;{{end}}</div>{{end}}{{end}}
    </div>
  </td></tr></table>
  {{end}}

  {{if .Cmds}}
  <div style="font-size:11px;letter-spacing:.12em;color:#6b7385;font-weight:700;margin:22px 0 8px;text-transform:uppercase">{{.CmdsT}}</div>
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="table-layout:fixed"><tr><td style="background:#f6f7f9;border:1px solid #e3e6ec;border-radius:10px;padding:12px 14px">
    <div style="font-family:Menlo,Consolas,'Courier New',monospace;font-size:12px;line-height:1.7;color:#1d2330;white-space:pre-wrap;overflow-wrap:break-word">{{range .Cmds}}<div style="padding-left:1.2em;text-indent:-1.2em"><span style="color:#8a91a0">$</span> {{.}}</div>{{end}}</div>
  </td></tr></table>
  {{end}}

  {{if .Link}}
  <table role="presentation" cellpadding="0" cellspacing="0" style="margin:24px auto 0"><tr><td align="center" style="background:#f0a830;border-radius:10px">
    <a href="{{.Link}}" style="display:inline-block;padding:12px 24px;color:#0f1115;font-weight:700;font-size:15px;text-decoration:none">{{.Button}}</a>
  </td></tr></table>
  {{end}}
</td></tr>
<tr><td align="center" style="padding:14px 10px;font-size:12px;color:#8a91a0">{{.Footer}}</td></tr>
</table>
</td></tr></table>
</body></html>`))
