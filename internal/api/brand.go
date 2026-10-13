package api

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"image/png"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// The color and the logo of a server, seen by every visitor: set by its owner. The interface turns any picture
// into a square PNG before sending it; only a PNG is kept, so the logo can never carry a script (as an SVG could).

var accentRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

const (
	logoMaxBytes = 256 << 10
	logoMaxSide  = 512
)

func brandRoutes(g *gin.RouterGroup, db *sql.DB) {
	// accent: "#RRGGBB", "" = default. logo: absent = unchanged, "" = removed, "data:image/png;base64,…" = new.
	g.PUT("/brand", sameOrigin, func(c *gin.Context) {
		var req struct {
			Accent string  `json:"accent"`
			Logo   *string `json:"logo"`
		}
		if c.ShouldBindJSON(&req) != nil || (req.Accent != "" && !accentRe.MatchString(req.Accent)) {
			c.JSON(400, gin.H{"error": "couleur invalide (#RRGGBB)"})
			return
		}
		id := mustID(c)
		var accent any
		if req.Accent != "" {
			accent = strings.ToUpper(req.Accent)
		}
		if _, err := db.Exec(`UPDATE servers SET accent=? WHERE id=?`, accent, id); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		detail := "couleur " + map[bool]string{true: req.Accent, false: "par défaut"}[req.Accent != ""]
		if req.Logo != nil {
			var logo []byte
			if *req.Logo != "" {
				b, err := decodeLogo(*req.Logo)
				if err != nil {
					c.JSON(400, gin.H{"error": err.Error()})
					return
				}
				logo = b
			}
			var ts any
			if logo != nil {
				ts = time.Now().Unix()
			}
			if _, err := db.Exec(`UPDATE servers SET logo=?, logo_ts=? WHERE id=?`, logo, ts, id); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			detail += map[bool]string{true: ", nouveau logo", false: ", logo retiré"}[logo != nil]
		}
		c.Set("audit", detail)
		c.JSON(200, gin.H{"ok": true})
	})

	g.GET("/logo.png", func(c *gin.Context) {
		var logo []byte
		if db.QueryRow(`SELECT logo FROM servers WHERE id=?`, mustID(c)).Scan(&logo) != nil || len(logo) == 0 {
			c.Status(404)
			return
		}
		c.Header("Cache-Control", "public, max-age=604800") // the URL carries the version (?v=logo_ts)
		c.Data(200, "image/png", logo)
	})
}

type logoError string

func (e logoError) Error() string { return string(e) }

// decodeLogo accepts a PNG data URL of a reasonable size.
func decodeLogo(s string) ([]byte, error) {
	raw, ok := strings.CutPrefix(s, "data:image/png;base64,")
	if !ok {
		return nil, logoError("logo : image PNG attendue")
	}
	b, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(b) > logoMaxBytes {
		return nil, logoError("logo : image illisible ou trop lourde (256 Ko max)")
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(b))
	if err != nil || cfg.Width > logoMaxSide || cfg.Height > logoMaxSide || cfg.Width < 16 || cfg.Height < 16 {
		return nil, logoError("logo : PNG de 16 à 512 pixels de côté attendu")
	}
	return b, nil
}
