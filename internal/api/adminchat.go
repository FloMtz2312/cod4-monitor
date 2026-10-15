package api

import (
	"fmt"
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

// Admins' chat of a server (Moderation tab): for the dashboard accounts of the server whose player in game is a B3 admin
// (level 20 or more). An account links its player once (!link <code> in game). @12 mentions the admin with the B3 id
// 12 (as in B3's commands), @name an admin by account or player name, @all everyone; a mention is pushed to the phone
// of the admin and counted on the Moderation tab until read.

const (
	adminChatLevel   = 20 // B3 moderator
	adminChatMaxBody = 1000
	adminChatCache   = 30 * time.Second
)

// ChatAdmin: an account of the admins' chat.
type ChatAdmin struct {
	Username string `json:"username"`
	Name     string `json:"name"` // player name in game, with colors
	B3ID     int64  `json:"b3_id"`
	Level    int    `json:"level"`
	Group    string `json:"group"`
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
		id, numErr := strconv.ParseInt(tag, 10, 64)
		for _, a := range admins {
			if numErr == nil && a.B3ID == id || numErr != nil && (strings.ToLower(a.Username) == tag || mentionName(a.Name) == tag) {
				add(a.Username)
			}
		}
	}
	return out
}

// mentionName: a player name as typed after @ (no colors, no spaces, lower case).
func mentionName(raw string) string {
	return strings.ToLower(strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, query.StripColors(raw)))
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

func adminChatRoutes(g *gin.RouterGroup, st *store.Store, adminUser string, ps *push.Sender) {
	access := Access(st.DB, adminUser, false)
	member := func(user string, id int64) bool {
		if user == "" {
			return false
		}
		owner, perms := access(user, id)
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
	var mu sync.Mutex
	cache := map[int64]chatAdmins{}
	// the admins of the chat: linked accounts that still have access to the server, whose player is a B3 admin
	admins := func(id int64, fresh bool) (chatAdmins, error) {
		mu.Lock()
		cur, ok := cache[id]
		mu.Unlock()
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
				d, err := b3.Open(conf.Logs.B3DB)
				if err != nil {
					return cur, err
				}
				staff, err := d.Admins()
				d.Close()
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
		}
		mu.Lock()
		cache[id] = cur
		mu.Unlock()
		return cur, nil
	}
	self := func(list []ChatAdmin, user string) (ChatAdmin, bool) {
		i := slices.IndexFunc(list, func(a ChatAdmin) bool { return a.Username == user })
		if i < 0 {
			return ChatAdmin{}, false
		}
		return list[i], true
	}
	// loginOnly: the chat needs accounts (who wrote, who is mentioned); not on a dashboard without login
	loginOnly := func(c *gin.Context) bool {
		if !c.GetBool("loginmode") || c.GetString("user") == "" {
			c.JSON(403, gin.H{"error": "le tchat des admins demande la connexion par compte (DASHBOARD_PASSWORD)", "login": false})
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
		me, ok := self(list.admins, user)
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

	// unread messages and mentions, for the badge of the Moderation tab (0 when not an admin of the chat)
	g.GET("/adminchat/unread", func(c *gin.Context) {
		id, user := mustID(c), c.GetString("user")
		out := gin.H{"unread": 0, "mentions": 0}
		if !c.GetBool("loginmode") || user == "" || !member(user, id) {
			c.JSON(200, out)
			return
		}
		if _, linked, err := st.AdminLinkOf(user, id); err != nil || !linked {
			c.JSON(200, out)
			return
		}
		list, err := admins(id, false)
		if err != nil {
			c.JSON(200, out)
			return
		}
		if _, ok := self(list.admins, user); ok {
			unread, mentions, err := st.AdminChatUnread(user, id)
			if fail(c, err) {
				return
			}
			out["unread"], out["mentions"] = unread, mentions
		}
		c.JSON(200, out)
	})

	// a code to type in game (!link <code>) to link the account to its player
	g.POST("/adminchat/link", sameOrigin, func(c *gin.Context) {
		if !loginOnly(c) {
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
		mu.Lock()
		delete(cache, mustID(c))
		mu.Unlock()
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
		me, ok := self(list.admins, user)
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
			ps.Notify(push.Message{Kind: "mention", Server: id, To: m.Mentions, Urgent: true, Path: "tchat-admins", Tag: "adminchat-" + strconv.FormatInt(id, 10),
				Title: name + " t'a mentionné", TitleEN: name + " mentioned you", Body: body + " · " + server, BodyEN: body + " · " + server})
		}
		c.JSON(200, gin.H{"message": m})
	})
}
