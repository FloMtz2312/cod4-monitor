package query

import (
	"net"
	"strconv"
	"testing"
	"time"
)

// A server on the same machine, queried through another of its addresses, answers from a different address (Docker
// bridge): the reply must still be read.
func TestReplyFromOtherAddress(t *testing.T) {
	in, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	port := in.LocalAddr().(*net.UDPAddr).Port
	out, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 2), Port: port})
	if err != nil {
		t.Skip("127.0.0.2 unavailable:", err)
	}
	defer out.Close()
	go func() {
		buf := make([]byte, 2048)
		for {
			n, from, err := in.ReadFromUDP(buf)
			if err != nil {
				return
			}
			reply := "\xff\xff\xff\xffstatusResponse\n\\sv_hostname\\Same machine\\mapname\\mp_crash\n"
			if string(buf[4:9]) == "rcon " {
				reply = "\xff\xff\xff\xffprint\nok " + string(buf[4:n])
			}
			out.WriteToUDP([]byte(reply), from) // from 127.0.0.2, not the queried 127.0.0.1
		}
	}()
	addr := "127.0.0.1:" + strconv.Itoa(port)
	st, err := Query(addr, 2*time.Second)
	if err != nil || st.Hostname != "Same machine" {
		t.Fatalf("query: %+v %v", st, err)
	}
	r, err := Rcon(addr, "pw", "status", 2*time.Second)
	if err != nil || r != "ok rcon pw status" {
		t.Fatalf("rcon: %q %v", r, err)
	}
}
