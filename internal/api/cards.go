package api

import (
	"bytes"
	"database/sql"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"
	"time"

	"cod4monitor/internal/query"
	"github.com/gin-gonic/gin"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

func cardImage(title, subtitle string, labels, values []string) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, 1200, 630))
	bg := color.RGBA{14, 16, 24, 255}
	white := color.RGBA{242, 241, 250, 255}
	muted := color.RGBA{161, 165, 186, 255}
	accent := color.RGBA{174, 133, 255, 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(0, 0, 1200, 10), &image.Uniform{accent}, image.Point{}, draw.Src)
	ft, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil, err
	}
	text := func(x, y int, size float64, value string, col color.Color, maxWidth int) {
		face, e := opentype.NewFace(ft, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
		if e != nil {
			return
		}
		defer face.Close()
		value = query.StripColors(value)
		d := font.Drawer{Dst: img, Src: &image.Uniform{col}, Face: face, Dot: fixed.P(x, y)}
		for len([]rune(value)) > 1 && d.MeasureString(value).Ceil() > maxWidth {
			r := []rune(value)
			value = string(r[:len(r)-2]) + "…"
		}
		d.DrawString(value)
	}
	text(60, 78, 22, "COD4  /  MONITOR", accent, 1080)
	text(60, 180, 52, title, white, 1080)
	text(60, 230, 22, subtitle, muted, 1080)
	for i, label := range labels {
		x := 60 + i*370
		draw.Draw(img, image.Rect(x, 290, x+340, 485), &image.Uniform{color.RGBA{26, 28, 41, 255}}, image.Point{}, draw.Src)
		text(x+22, 350, 20, label, muted, 296)
		text(x+22, 433, 58, values[i], white, 296)
	}
	text(60, 565, 19, "Statistiques issues des événements enregistrés • COD4 Community", muted, 1080)
	var b bytes.Buffer
	err = png.Encode(&b, img)
	return b.Bytes(), err
}
func cardRoutes(g *gin.RouterGroup, db *sql.DB) {
	g.GET("/card.png", func(c *gin.Context) {
		id := c.Param("id")
		var server string
		if db.QueryRow(`SELECT name FROM servers WHERE id=?`, id).Scan(&server) != nil {
			c.Status(404)
			return
		}
		var title, subtitle string
		var labels, values []string
		switch c.Query("type") {
		case "player":
			key := resolveKey(db, id, c.Query("key"))
			if key == "" || len(key) > 200 {
				c.Status(400)
				return
			}
			// Never fall back to an internal identifier in the downloadable card.
			err := db.QueryRow(`SELECT name FROM (SELECT name_raw AS name,last_seen AS ts FROM player_names WHERE server_id=?1 AND guid=?2
    UNION ALL SELECT killer_name,ts FROM kills WHERE server_id=?1 AND killer_key=?2
    UNION ALL SELECT victim_name,ts FROM kills WHERE server_id=?1 AND victim_key=?2) ORDER BY ts DESC LIMIT 1`, id, key).Scan(&title)
			if err != nil {
				c.Status(404)
				return
			}
			now := time.Now().Unix()
			f, err := playerForm(db, id, key, now-7*86400, now)
			if err != nil {
				c.Status(500)
				return
			}
			subtitle = server + " • 7 derniers jours"
			labels = []string{"KILLS / MORTS", "K/D", "HEADSHOTS"}
			values = []string{fmt.Sprintf("%d / %d", f.Kills, f.Deaths), fmt.Sprintf("%.2f", f.KD), fmt.Sprint(f.Headshots)}
		case "match":
			rid := mustInt(c.Query("match"))
			var start int64
			if db.QueryRow(`SELECT started_at FROM rounds WHERE id=? AND server_id=?`, rid, id).Scan(&start) != nil {
				c.Status(404)
				return
			}
			ms, err := matchGroups(db, id, start-86400, start+86400)
			if err != nil {
				c.Status(500)
				return
			}
			var found *match
			for _, m := range ms {
				for _, r := range m.Rounds {
					if r == rid {
						found = m
					}
				}
			}
			if found == nil {
				c.Status(404)
				return
			}
			_, a, x := matchResult(db, found)
			score := "—"
			if a != nil && x != nil {
				score = fmt.Sprintf("%v–%v", a, x)
			}
			title = strings.ToUpper(strings.ReplaceAll(strings.TrimPrefix(found.Map, "mp_"), "_", " "))
			subtitle = server + " • " + strings.ToUpper(found.Gametype) + " • " + time.Unix(found.Start, 0).UTC().Format("02/01/2006 15:04 UTC")
			labels = []string{"SCORE ALLIÉS / AXE", "MANCHES", "KILLS"}
			values = []string{score, fmt.Sprint(found.RoundCount), fmt.Sprint(found.Kills)}
		default:
			c.Status(400)
			return
		}
		b, err := cardImage(title, subtitle, labels, values)
		if err != nil {
			c.Status(500)
			return
		}
		c.Header("Content-Disposition", `attachment; filename="cod4-stats.png"`)
		c.Header("Cache-Control", "private, max-age=60")
		c.Data(200, "image/png", b)
	})
}
