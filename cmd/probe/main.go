// probe: one-shot query, e.g. go run ./cmd/probe 203.0.113.10:55555
// rcon:  RCON_PASSWORD=... go run ./cmd/probe -rcon status 203.0.113.10:55555
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"cod4monitor/internal/query"
)

func main() {
	rcon := flag.String("rcon", "", "rcon command to run (password from RCON_PASSWORD)")
	flag.Parse()
	addr := "203.0.113.10:55555"
	if flag.NArg() > 0 {
		addr = flag.Arg(0)
	}
	if *rcon != "" {
		out, err := query.Rcon(addr, os.Getenv("RCON_PASSWORD"), *rcon, 2*time.Second)
		if err != nil {
			fmt.Println("rcon failed:", err)
			os.Exit(1)
		}
		fmt.Print(out)
		return
	}
	st, err := query.Query(addr, 2*time.Second)
	if err != nil {
		fmt.Println("offline:", err)
		os.Exit(1)
	}
	fmt.Println("latency:", st.Latency)
	out, _ := json.MarshalIndent(st, "", "  ")
	fmt.Println(string(out))
}
