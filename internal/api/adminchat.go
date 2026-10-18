package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"

	"cod4monitor/internal/b3"
	"cod4monitor/internal/push"
	"cod4monitor/internal/query"
	"cod4monitor/internal/store"
)

// Admins' chat of a server (Moderation tab). With B3: for the dashboard accounts of the server whose player in game is a
// B3 admin (level 20 or more); an account links its player once (!link <code> in game). Without B3: for the server's
// team (owner and members), no link. @pseudo mentions an admin (player name in game with B3, account name without; the
// account name works too) and @all everyone; a mention is pushed to the phone of the admin and counted on the
// Moderation tab until read.

const (
	adminChatLevel   = 20 // B3 moderator
	adminChatMaxBody = 1000
	adminChatCache   = 30 * time.Second
)

// ChatAdmin: an account of the admins' chat.
type ChatAdmin struct {
	Username string `json:"username"`
	Name     string `json:"name"` // player name in game, with colors
	Tag      string `json:"tag"`  // what follows @ to mention them: the pseudo, typable (see mentionName)
	B3ID     int64  `json:"b3_id"`
	Level    int    `json:"level"`
	Group    string `json:"group"` // B3 group name; without B3, "owner" or "team"
	guid     string
}

type chatAdmins struct {
	at     time.Time
	admins []ChatAdmin
	b3     bool
}

var mentionRe = regexp.MustCompile(`(?:^|[^\p{L}\p{N}_])@([\p{L}\p{N}_.\-]{1,32})`)

// mentioned: the accounts named by the @ of the text (not the author).
func mentioned(body, author string, admins []ChatAdmin) []string {
	var out []string
	add := func(u string) {
		if u != author && !slices.Contains(out, u) {
			out = append(out, u)
		}
	}
	for _, m := range mentionRe.FindAllStringSubmatch(body, -1) {
		tag := strings.ToLower(strings.TrimRight(m[1], ".-"))
		if tag == "all" || tag == "tous" || tag == "alle" {
			for _, a := range admins {
				add(a.Username)
			}
			continue
		}
		// the pseudo shown in the chat first (unique), else an account name, else a player name
		i := slices.IndexFunc(admins, func(a ChatAdmin) bool { return strings.ToLower(a.Tag) == tag })
		if i < 0 {
			i = slices.IndexFunc(admins, func(a ChatAdmin) bool { return strings.ToLower(a.Username) == tag })
		}
		if i < 0 {
			i = slices.IndexFunc(admins, func(a ChatAdmin) bool { return mentionName(a.Name) == tag })
		}
		if i >= 0 {
			add(admins[i].Username)
		}
	}
	return out
}

// mentionName: a player name as typed after @: no colors, only letters, digits and _ . - (UpT# Babou → uptbabou).
func mentionName(raw string) string {
	return strings.TrimRight(strings.ToLower(strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' || r == '-' {
			return r
		}
		return -1
	}, query.StripColors(raw))), ".-")
}

// withTags gives each admin a pseudo to mention them with, unique in the chat (else their account name).
func withTags(admins []ChatAdmin) []ChatAdmin {
	used := map[string]bool{}
	for i := range admins {
		tag := mentionName(admins[i].Name)
		if tag == "" || used[tag] || len([]rune(tag)) > 32 {
			tag = strings.ToLower(admins[i].Username)
		}
		used[tag] = true
		admins[i].Tag = tag
	}
	return admins
}

// cleanBody: the text of a message, without control characters (line breaks kept), at most adminChatMaxBody characters.
func cleanBody(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '\n' || !unicode.IsControl(r) {
			return r
		}
		return -1
	}, strings.ReplaceAll(s, "\r\n", "\n"))
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > adminChatMaxBody {
		s = string(r[:adminChatMaxBody])
	}
	return s
}

// chatDirectory: who is in the admins' chat of each server (cached a little: B3 is read at most every 30 s).
type chatDirectory struct {
	st     *store.Store
	access func(user string, id int64) (bool, map[string]bool)
	mu     sync.Mutex
	cache  map[int64]chatAdmins
}

func newChatDirectory(st *store.Store, adminUser string) *chatDirectory {
	return &chatDirectory{st: st, access: Access(st.DB, adminUser, false), cache: map[int64]chatAdmins{}}
}

// member: the account owns the server or is in its team.
func (d *chatDirectory) member(user string, id int64) bool {
	if user == "" {
		return false
	}
	owner, perms := d.access(user, id)
	if owner {
		return true
	}
	for _, ok := range perms {
		if ok {
			return true
		}
	}
	return false
}

func (d *chatDirectory) forget(id int64) {
	d.mu.Lock()
	delete(d.cache, id)
	d.mu.Unlock()
}

// admins of the chat: linked accounts that still have access to the server, whose player is a B3 admin; without B3,
// the owner and the members of the server
func (d *chatDirectory) admins(id int64, fresh bool) (chatAdmins, error) {
	st, member := d.st, d.member
	d.mu.Lock()
	cur, ok := d.cache[id]
	d.mu.Unlock()
	if ok && !fresh && time.Since(cur.at) < adminChatCache {
		return cur, nil
	}
	cur = chatAdmins{at: time.Now()}
	conf, found, err := st.ServerConf(id)
	if err != nil {
		return cur, err
	}
	if found && conf.Logs.B3DB != "" {
		cur.b3 = true
		links, err := st.AdminLinks(id)
		if err != nil {
			return cur, err
		}
		if len(links) > 0 {
			bdb, err := b3.Open(conf.Logs.B3DB)
			if err != nil {
				return cur, err
			}
			staff, err := bdb.Admins()
			bdb.Close()
			if err != nil {
				return cur, err
			}
			for _, l := range links {
				i := slices.IndexFunc(staff, func(c b3.Client) bool { return c.GUID == l.GUID })
				if i < 0 || staff[i].Group.Level < adminChatLevel || !member(l.Username, id) {
					continue
				}
				name := staff[i].Name
				if name == "" {
					name = l.NameRaw
				}
				cur.admins = append(cur.admins, ChatAdmin{Username: l.Username, Name: name, B3ID: staff[i].ID, Level: staff[i].Group.Level,
					Group: staff[i].Group.Name, guid: l.GUID})
			}
		}
	} else if found {
		team, err := rows(st.DB, `SELECT username, 'team' AS grp FROM server_members WHERE server_id=? UNION SELECT owner, 'owner' FROM servers WHERE id=? AND owner<>''
			ORDER BY 2, 1`, id, id)
		if err != nil {
			return cur, err
		}
		for _, r := range team { // read before member(): one connection to the database
			a := ChatAdmin{Username: fmt.Sprint(r["username"]), Group: fmt.Sprint(r["grp"])}
			if member(a.Username, id) && !slices.ContainsFunc(cur.admins, func(b ChatAdmin) bool { return b.Username == a.Username }) {
				a.Name = a.Username
				cur.admins = append(cur.admins, a)
			}
		}
	}
	cur.admins = withTags(cur.admins)
	d.mu.Lock()
	d.cache[id] = cur
	d.mu.Unlock()
	return cur, nil
}

func adminChatRoutes(g *gin.RouterGroup, st *store.Store, adminUser string, ps *push.Sender) {
	dir := newChatDirectory(st, adminUser)
	member, admins := dir.member, dir.admins
	// the account in the chat; without B3 any member of the server (the super administrator is in no team)
	self := func(list chatAdmins, user string) (ChatAdmin, bool) {
		i := slices.IndexFunc(list.admins, func(a ChatAdmin) bool { return a.Username == user })
		if i < 0 {
			return ChatAdmin{Username: user, Name: user, Tag: strings.ToLower(user), Group: "team"}, !list.b3
		}
		return list.admins[i], true
	}
	// loginOnly: the chat needs accounts (who wrote, who is mentioned); not on a dashboard without login
	loginOnly := func(c *gin.Context) bool {
		if !c.GetBool("loginmode") || c.GetString("user") == "" {
			c.JSON(403, gin.H{"error": "le chat des admins demande la connexion par compte (DASHBOARD_PASSWORD)", "login": false})
			return false
		}
		if !member(c.GetString("user"), mustID(c)) {
			c.JSON(403, gin.H{"error": "accès non autorisé pour ce serveur"})
			return false
		}
		return true
	}
	fail := func(c *gin.Context, err error) bool {
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
		}
		return err != nil
	}

	// state of the account (linked player, admin or not), the admins, and the messages (after= for the new ones)
	g.GET("/adminchat", func(c *gin.Context) {
		if !loginOnly(c) {
			return
		}
		id, user := mustID(c), c.GetString("user")
		link, linked, err := st.AdminLinkOf(user, id)
		if fail(c, err) {
			return
		}
		list, err := admins(id, c.Query("fresh") == "1")
		if fail(c, err) {
			return
		}
		out := gin.H{"login": true, "b3": list.b3, "linked": linked, "admin": false, "me": user, "level_min": adminChatLevel}
		if linked {
			out["player"] = link.NameRaw
		}
		me, ok := self(list, user)
		if !ok {
			c.JSON(200, out)
			return
		}
		after, _ := strconv.ParseInt(c.Query("after"), 10, 64)
		before, _ := strconv.ParseInt(c.Query("before"), 10, 64)
		msgs, err := st.AdminMessages(id, after, before, 100)
		if fail(c, err) {
			return
		}
		out["admin"], out["self"], out["admins"], out["messages"] = true, me, list.admins, msgs
		c.JSON(200, out)
	})

	// whether the account is in the chat (its tab in the menu), unread messages and mentions (its badge)
	g.GET("/adminchat/unread", func(c *gin.Context) {
		id, user := mustID(c), c.GetString("user")
		out := gin.H{"admin": false, "unread": 0, "mentions": 0}
		if !c.GetBool("loginmode") || user == "" || !member(user, id) {
			c.JSON(200, out)
			return
		}
		list, err := admins(id, false)
		if err != nil {
			c.JSON(200, out)
			return
		}
		if _, ok := self(list, user); ok {
			unread, mentions, err := st.AdminChatUnread(user, id)
			if fail(c, err) {
				return
			}
			out["admin"], out["unread"], out["mentions"] = true, unread, mentions
		}
		c.JSON(200, out)
	})

	// a code to type in game (!link <code>) to link the account to its player
	g.POST("/adminchat/link", sameOrigin, func(c *gin.Context) {
		if !loginOnly(c) {
			return
		}
		if list, err := admins(mustID(c), false); err == nil && !list.b3 {
			c.JSON(400, gin.H{"error": "pas de B3 sur ce serveur : rien à relier"})
			return
		}
		code, err := st.AdminLinkCode(c.GetString("user"), mustID(c), time.Now())
		if fail(c, err) {
			return
		}
		c.JSON(200, gin.H{"code": code, "ttl_s": int(store.AdminLinkTTL.Seconds())})
	})

	g.DELETE("/adminchat/link", sameOrigin, func(c *gin.Context) {
		if !loginOnly(c) {
			return
		}
		if fail(c, st.DeleteAdminLink(c.GetString("user"), mustID(c))) {
			return
		}
		dir.forget(mustID(c))
		c.JSON(200, gin.H{"ok": true})
	})

	g.POST("/adminchat/read", sameOrigin, func(c *gin.Context) {
		var in struct {
			ID int64 `json:"id"`
		}
		if !loginOnly(c) {
			return
		}
		if c.ShouldBindJSON(&in) != nil {
			c.Status(400)
			return
		}
		if fail(c, st.AdminChatRead(c.GetString("user"), mustID(c), in.ID)) {
			return
		}
		c.JSON(200, gin.H{"ok": true})
	})

	g.POST("/adminchat", sameOrigin, func(c *gin.Context) {
		if !loginOnly(c) {
			return
		}
		var in struct {
			Body string `json:"body"`
		}
		if c.ShouldBindJSON(&in) != nil {
			c.Status(400)
			return
		}
		body := cleanBody(in.Body)
		if body == "" {
			c.JSON(400, gin.H{"error": "message vide"})
			return
		}
		id, user := mustID(c), c.GetString("user")
		list, err := admins(id, false)
		if fail(c, err) {
			return
		}
		me, ok := self(list, user)
		if !ok {
			c.JSON(403, gin.H{"error": fmt.Sprintf("réservé aux admins B3 (niveau %d ou plus) dont le compte est relié à leur joueur", adminChatLevel)})
			return
		}
		m := store.AdminMessage{TS: time.Now().Unix(), Username: user, NameRaw: me.Name, B3ID: me.B3ID, Level: me.Level, Body: body,
			Mentions: mentioned(body, user, list.admins)}
		if m.ID, err = st.AddAdminMessage(id, m); fail(c, err) {
			return
		}
		st.AdminChatRead(user, id, m.ID)
		if len(m.Mentions) > 0 {
			var server string
			if conf, found, err := st.ServerConf(id); err == nil && found {
				server = query.StripColors(conf.Name)
			}
			name := strings.TrimSpace(query.StripColors(me.Name))
			if name == "" {
				name = user
			}
			ps.Notify(push.Message{Kind: "mention", Server: id, To: m.Mentions, Urgent: true, Path: "chat-admins", Tag: "adminchat-" + strconv.FormatInt(id, 10),
				Title: name + " t'a mentionné", TitleEN: name + " mentioned you", Body: body + " · " + server, BodyEN: body + " · " + server})
		}
		c.JSON(200, gin.H{"message": m})
	})
}

// RunCallAdmin writes each !calladmin typed in game in the admins' chat of the server, mentioning all its admins, and
// pushes it to their phones; without anyone in the chat, to the staff who handle the reports.
func RunCallAdmin(ctx context.Context, st *store.Store, hub *Hub, ps *push.Sender, adminUser string) {
	events := hub.sub()
	defer hub.unsub(events)
	dir := newChatDirectory(st, adminUser)
	for {
		select {
		case <-ctx.Done():
			return
		case b := <-events:
			var ev struct {
				Type     string `json:"type"`
				ServerID int64  `json:"server_id"`
				Key      string `json:"key"`
				NameRaw  string `json:"name_raw"`
				Reason   string `json:"reason"`
				TS       int64  `json:"ts"`
			}
			if json.Unmarshal(b, &ev) != nil || ev.Type != "calladmin" || ev.ServerID <= 0 {
				continue
			}
			if err := callAdmin(st, dir, ps, ev.ServerID, ev.Key, ev.NameRaw, ev.Reason, time.Unix(ev.TS, 0)); err != nil {
				slog.Error("calladmin", "server_id", ev.ServerID, "err", err)
			}
		}
	}
}

func callAdmin(st *store.Store, dir *chatDirectory, ps *push.Sender, id int64, key, nameRaw, reason string, at time.Time) error {
	list, err := dir.admins(id, true)
	if err != nil {
		return err
	}
	var to []string
	for _, a := range list.admins {
		to = append(to, a.Username)
	}
	reason = cleanBody(reason)
	m := store.AdminMessage{Kind: "calladmin", PlayerKey: key, TS: at.Unix(), NameRaw: nameRaw, Body: reason, Mentions: to}
	if m.ID, err = st.AddAdminMessage(id, m); err != nil {
		return err
	}
	var server string
	if conf, found, err := st.ServerConf(id); err == nil && found {
		server = query.StripColors(conf.Name)
	}
	name := strings.TrimSpace(query.StripColors(nameRaw))
	body, bodyEN := "Sans raison", "No reason"
	if reason != "" {
		body, bodyEN = "« "+reason+" »", `"`+reason+`"`
	}
	msg := push.Message{Kind: "report", Server: id, Need: []string{"reports", "private"}, Urgent: true, Path: "chat-admins",
		Tag: "calladmin-" + strconv.FormatInt(m.ID, 10), Title: "Appel admin : " + name, TitleEN: "Admin call: " + name,
		Body: body + " · " + server, BodyEN: bodyEN + " · " + server}
	if len(to) > 0 {
		msg.Kind, msg.To = "mention", to
	}
	ps.Notify(msg)
	return nil
}
