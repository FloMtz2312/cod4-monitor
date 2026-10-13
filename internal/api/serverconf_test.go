package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"cod4monitor/internal/logtail"
	"cod4monitor/internal/store"
)

type applied []int64

func (a *applied) Apply(id int64) error { *a = append(*a, id); return nil }

func TestServerConfRoutes(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	var app applied
	r := gin.New()
	serverConfRoutes(r, st, &app)
	do := func(method, path, body string) (int, map[string]any) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Host = "localhost" // no login in this test: changes are only accepted locally
		r.ServeHTTP(w, req)
		var out map[string]any
		json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}

	if code, out := do("POST", "/api/serverconf", `{"name":"S","address":"1.2.3.4"}`); code != 400 || !strings.Contains(out["error"].(string), "ip:port") {
		t.Fatalf("bad address accepted: %d %v", code, out)
	}
	code, out := do("POST", "/api/serverconf", `{"name":"S","address":"1.2.3.4:28960","ftp_host":"ftp.host","ftp_user":"u","ftp_pass":"p@ss","ftp_path":"mods/m/games_mp.log","rcon":"secret"}`)
	if code != 200 {
		t.Fatalf("create: %d %v", code, out)
	}
	id := int64(out["id"].(float64))
	c, _, _ := st.ServerConf(id)
	if c.LogURL != "ftp://u:p%40ss@ftp.host:21/mods/m/" || c.RconPass != "secret" || !c.Enabled {
		t.Fatalf("saved: %+v", c)
	}
	// edit without the passwords: they are kept
	if code, out := do("PUT", "/api/serverconf/1", `{"name":"S2","address":"1.2.3.4:28960","ftp_host":"ftp.host","ftp_port":2121,"ftp_user":"u","ftp_path":"/mods/m/"}`); code != 200 {
		t.Fatalf("edit: %d %v", code, out)
	}
	c, _, _ = st.ServerConf(id)
	if c.Name != "S2" || c.LogURL != "ftp://u:p%40ss@ftp.host:2121/mods/m/" || c.RconPass != "secret" {
		t.Fatalf("edited: %+v", c)
	}
	// the list never contains the passwords
	_, list := do("GET", "/api/serverconf", "")
	b, _ := json.Marshal(list)
	if strings.Contains(string(b), "p@ss") || strings.Contains(string(b), "secret") || !strings.Contains(string(b), `"has_rcon":true`) {
		t.Fatalf("list: %s", b)
	}
	if code, _ := do("POST", "/api/serverconf/1/enabled", `{"enabled":false}`); code != 200 {
		t.Fatal("disable")
	}
	if c, _, _ := st.ServerConf(id); c.Enabled {
		t.Fatal("still enabled")
	}
	if code, _ := do("POST", "/api/serverconf", `{"name":"Dup","address":"1.2.3.4:28960"}`); code != 409 {
		t.Fatalf("duplicate address: %d", code)
	}
	if len(app) != 3 {
		t.Fatalf("applied %v, want 3 times", app)
	}
	// game server on the same machine: logs read in a local folder, only inside LOCAL_LOGS_ROOT
	defer func(old string) { logtail.LocalRoot = old }(logtail.LocalRoot)
	logtail.SetLocalRoot("/cod4/")
	if code, out := do("POST", "/api/serverconf", `{"name":"L","address":"127.0.0.1:28961","log_dir":"/data/"}`); code != 400 {
		t.Fatalf("local folder outside /cod4/ accepted: %d %v", code, out)
	}
	code, out = do("POST", "/api/serverconf", `{"name":"L","address":"127.0.0.1:28961","log_dir":"/cod4/mods/m/games_mp.log"}`)
	if code != 200 {
		t.Fatalf("local: %d %v", code, out)
	}
	if c, _, _ := st.ServerConf(int64(out["id"].(float64))); c.LogURL != "file:///cod4/mods/m/" {
		t.Fatalf("local saved: %+v", c)
	}
	if _, list := do("GET", "/api/serverconf", ""); !strings.Contains(fmtJSON(list), `"log_dir":"/cod4/mods/m/"`) {
		t.Fatalf("local list: %v", list)
	}
	// advanced settings: other log names / paths, time zone
	for _, bad := range []string{`"games_log":"../../etc/passwd"`, `"console_log":"/etc/passwd"`, `"log_tz":"Mars/Base"`} {
		if code, out := do("PUT", "/api/serverconf/2", `{"name":"L","address":"127.0.0.1:28961","log_dir":"/cod4/mods/m/",`+bad+`}`); code != 400 {
			t.Fatalf("%s accepted: %d %v", bad, code, out)
		}
	}
	if code, out := do("PUT", "/api/serverconf/2", `{"name":"L","address":"127.0.0.1:28961","log_dir":"/cod4/mods/m/","games_log":"logs/games.log","console_log":"/cod4/console.log","log_tz":"Europe/Paris"}`); code != 200 {
		t.Fatalf("advanced: %d %v", code, out)
	}
	if c, _, _ := st.ServerConf(2); c.Logs != (store.LogFiles{Games: "logs/games.log", Console: "/cod4/console.log", TZ: "Europe/Paris"}) {
		t.Fatalf("advanced saved: %+v", c.Logs)
	}
}

func fmtJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
