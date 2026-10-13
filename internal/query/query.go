// Package query implements the Quake 3 / CoD4 "getstatus" UDP query.
package query

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Player struct {
	NameRaw   string `json:"name_raw"`
	NameClean string `json:"name"`
	Score     int    `json:"score"`
	Ping      int    `json:"ping"`
}

type Status struct {
	Vars       map[string]string `json:"vars"`
	Players    []Player          `json:"players"`
	Hostname   string            `json:"hostname"`
	Map        string            `json:"map"`
	Gametype   string            `json:"gametype"`
	MaxPlayers int               `json:"max_players"`
	Latency    time.Duration     `json:"-"`
}

var (
	header     = []byte("\xff\xff\xff\xffstatusResponse")
	colorRe    = regexp.MustCompile(`\^[0-9]`)
	playerRe   = regexp.MustCompile(`^(-?\d+)\s+(-?\d+)\s+"(.*)"$`)
	ErrBadResp = errors.New("not a statusResponse")
)

// HostOf returns the IP of a player address: "1.2.3.4:28960" → "1.2.3.4", "[2a01:e0a::1]:28960" → "2a01:e0a::1".
func HostOf(addr string) string {
	if h, _, err := net.SplitHostPort(addr); err == nil {
		return h
	}
	return strings.Trim(addr, "[]")
}

// StripColors removes CoD color codes (^0..^9).
func StripColors(s string) string { return colorRe.ReplaceAllString(s, "") }

// Query sends getstatus to addr and parses the reply. A timeout means the server is offline.
func Query(addr string, timeout time.Duration) (*Status, error) {
	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	conn, err := dial(raddr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(timeout))

	start := time.Now()
	if _, err := conn.Write([]byte("\xff\xff\xff\xffgetstatus\n")); err != nil {
		return nil, err
	}
	buf := make([]byte, 16384)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	st, err := Parse(buf[:n])
	if err != nil {
		return nil, err
	}
	st.Latency = time.Since(start)
	return st, nil
}

// Parse decodes a raw statusResponse packet.
func Parse(b []byte) (*Status, error) {
	if !bytes.HasPrefix(b, header) {
		return nil, ErrBadResp
	}
	lines := strings.Split(strings.TrimRight(string(b[len(header):]), "\n\x00"), "\n")
	// lines[0] is the empty rest of the header line.
	if len(lines) < 2 {
		return nil, fmt.Errorf("%w: missing info line", ErrBadResp)
	}
	st := &Status{Vars: map[string]string{}, Players: []Player{}}
	kv := strings.Split(strings.TrimPrefix(lines[1], `\`), `\`)
	for i := 0; i+1 < len(kv); i += 2 {
		st.Vars[kv[i]] = kv[i+1]
	}
	st.Hostname = st.Vars["sv_hostname"]
	st.Map = st.Vars["mapname"]
	st.Gametype = st.Vars["g_gametype"]
	st.MaxPlayers, _ = strconv.Atoi(st.Vars["sv_maxclients"])

	for _, l := range lines[2:] {
		m := playerRe.FindStringSubmatch(strings.TrimSpace(l))
		if m == nil {
			continue // truncated/garbage line
		}
		score, _ := strconv.Atoi(m[1])
		ping, _ := strconv.Atoi(m[2])
		st.Players = append(st.Players, Player{NameRaw: m[3], NameClean: StripColors(m[3]), Score: score, Ping: ping})
	}
	sort.SliceStable(st.Players, func(i, j int) bool { return st.Players[i].Score > st.Players[j].Score })
	return st, nil
}

// Rcon runs a console command. The reply ("print\n...") can span several packets: read until the server goes quiet.
func Rcon(addr, password, cmd string, timeout time.Duration) (string, error) {
	if strings.ContainsAny(cmd, "\n\r") {
		return "", errors.New("command must be a single line")
	}
	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return "", err
	}
	conn, err := dial(raddr)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("\xff\xff\xff\xffrcon " + password + " " + cmd)); err != nil {
		return "", err
	}
	var out strings.Builder
	buf := make([]byte, 16384)
	conn.SetReadDeadline(time.Now().Add(timeout))
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if out.Len() > 0 { // timeout after at least one packet = end of reply
				break
			}
			return "", err
		}
		out.WriteString(rconPayload(buf[:n]))
		conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	}
	return out.String(), nil
}

// udpConn talks to one game server without connecting the socket: a server on the same machine, reached through its
// public address from a Docker container, answers from another address of the machine (the Docker bridge). A connected
// socket would drop those replies, so replies are taken from any address, as long as they come from the server's port.
type udpConn struct {
	*net.UDPConn
	raddr *net.UDPAddr
}

func dial(raddr *net.UDPAddr) (*udpConn, error) {
	network := "udp4"
	if raddr.IP.To4() == nil {
		network = "udp6"
	}
	c, err := net.ListenUDP(network, nil)
	if err != nil {
		return nil, err
	}
	return &udpConn{c, raddr}, nil
}

func (c *udpConn) Write(b []byte) (int, error) { return c.WriteToUDP(b, c.raddr) }

func (c *udpConn) Read(b []byte) (int, error) {
	for {
		n, from, err := c.ReadFromUDP(b)
		if err != nil || from.Port == c.raddr.Port {
			return n, err
		}
	}
}

func rconPayload(b []byte) string {
	s := strings.TrimPrefix(string(b), "\xff\xff\xff\xff")
	return strings.TrimPrefix(s, "print\n")
}

type RconPlayer struct {
	Num        int
	GUID, Name string // Name keeps color codes
	Addr       string
}

// Rows of the rcon "status" command. Legacy (stock CoD4, CoD4X with sv_legacymode 1):
//
//	num score ping guid name lastmsg address qport rate
//
// CoD4X 1.8: a steamid column after the player id:
//
//	num score ping playerid steamid name lastmsg address qport rate
var (
	statusRe      = regexp.MustCompile(`^\s*(\d+)\s+-?\d+\s+(?:\d+|CNCT|ZMBI|PRIM)\s+(\S+)\s+(.*?)\s+\d+\s+(\S+)\s+-?\d+\s+\d+\s*$`)
	statusSteamRe = regexp.MustCompile(`^\s*(\d+)\s+-?\d+\s+(?:\d+|CNCT|ZMBI|PRIM)\s+(\S+)\s+\S+\s+(.*?)\s+\d+\s+(\S+)\s+-?\d+\s+\d+\s*$`)
)

// ParseRconStatus reads the player table of the rcon "status" command, in either layout (picked from the header).
func ParseRconStatus(out string) []RconPlayer {
	var ps []RconPlayer
	re := statusRe
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "num ") {
			if strings.Contains(l, "steamid") {
				re = statusSteamRe
			}
			continue
		}
		m := re.FindStringSubmatch(strings.TrimRight(l, "\r"))
		if m == nil {
			continue
		}
		num, _ := strconv.Atoi(m[1])
		ps = append(ps, RconPlayer{Num: num, GUID: m[2], Name: strings.TrimSuffix(m[3], "^7"), Addr: m[4]})
	}
	return ps
}

var dateRe = regexp.MustCompile(`\b(?:Mon|Tue|Wed|Thu|Fri|Sat|Sun) (?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec) +\d{1,2} \d\d:\d\d:\d\d \d{4}\b`)

// LocalizeDates rewrites the server's "Tue Oct  6 12:23:46 2026" dates (clock in zone from) as "06/10/2026 14:23:46" in zone to.
func LocalizeDates(s string, from, to *time.Location) string {
	return dateRe.ReplaceAllStringFunc(s, func(d string) string {
		t, err := time.ParseInLocation("Mon Jan _2 15:04:05 2006", d, from)
		if err != nil {
			return d
		}
		return t.In(to).Format("02/01/2006 15:04:05")
	})
}
