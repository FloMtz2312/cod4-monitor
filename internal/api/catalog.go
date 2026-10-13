package api

import (
	"context"
	"io/fs"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// stockMaps: CoD4 multiplayer maps (DLC included) with their in-game names.
var stockMaps = map[string]string{
	"mp_backlot": "Backlot", "mp_bloc": "Bloc", "mp_bog": "Bog", "mp_broadcast": "Broadcast", "mp_carentan": "Chinatown",
	"mp_cargoship": "Wet Work", "mp_citystreets": "District", "mp_convoy": "Ambush", "mp_countdown": "Countdown",
	"mp_crash": "Crash", "mp_crash_snow": "Winter Crash", "mp_creek": "Creek", "mp_crossfire": "Crossfire",
	"mp_farm": "Downpour", "mp_killhouse": "Killhouse", "mp_overgrown": "Overgrown", "mp_pipeline": "Pipeline",
	"mp_shipment": "Shipment", "mp_showdown": "Showdown", "mp_strike": "Strike", "mp_vacant": "Vacant",
}

// customTitle turns "mp_dust2_classic" into "Dust2 Classic".
func customTitle(name string) string {
	words := strings.Fields(strings.ReplaceAll(strings.TrimPrefix(name, "mp_"), "_", " "))
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// catalogRoutes: maps playable on the server (stock + its usermaps/) and its mods, for the map picker.
func catalogRoutes(g *gin.RouterGroup, web fs.FS, gameDirs func(context.Context, int64) ([]string, []string, error)) {
	type entry struct {
		at             time.Time
		mods, usermaps []string
		err            error
	}
	var mu sync.Mutex
	cache := map[int64]entry{} // ponytail: FTP listing cached 5 min, new uploads show up after that

	g.GET("/catalog", func(c *gin.Context) {
		id := mustID(c)
		mu.Lock()
		e, ok := cache[id]
		mu.Unlock()
		if !ok || time.Since(e.at) > 5*time.Minute || c.Query("refresh") != "" {
			e = entry{at: time.Now()}
			if gameDirs != nil {
				ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
				e.mods, e.usermaps, e.err = gameDirs(ctx, id)
				cancel()
			}
			mu.Lock()
			cache[id] = e
			mu.Unlock()
		}
		type m struct {
			Name   string `json:"name"`
			Title  string `json:"title"`
			Custom bool   `json:"custom"`
			Image  string `json:"image,omitempty"`
		}
		var maps []m
		add := func(name, title string, custom bool) {
			img := "maps/" + name + ".jpg"
			if web == nil {
				img = ""
			} else if _, err := fs.Stat(web, img); err != nil {
				img = ""
			}
			maps = append(maps, m{name, title, custom, img})
		}
		for name, title := range stockMaps {
			add(name, title, false)
		}
		for _, name := range e.usermaps {
			if _, stock := stockMaps[name]; !stock && strings.HasPrefix(name, "mp_") {
				add(name, customTitle(name), true)
			}
		}
		sort.Slice(maps, func(i, j int) bool {
			if maps[i].Custom != maps[j].Custom {
				return maps[i].Custom // custom maps first: that's what this server is about
			}
			return maps[i].Title < maps[j].Title
		})
		out := gin.H{"maps": maps, "mods": e.mods}
		if e.err != nil {
			out["error"] = "liste FTP indisponible : " + e.err.Error()
		}
		c.JSON(200, out)
	})
}
