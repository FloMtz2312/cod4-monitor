package demos

import (
	"time"

	"cod4monitor/internal/store"
)

// DefaultMBPerHour: size of one player's demo per hour of play, until the server's own demos are measured
// (cod4-demo: 8 to 20 MB for a 15-20 minute match).
const DefaultMBPerHour = 45.0

// Estimate guides the choice of the keep time: what the demos would take, and how long the space allows.
//
//	space per day ≈ average players × hours a day above the threshold × MB per player and hour
//	possible time ≈ space for the demos ÷ space per day
type Estimate struct {
	HoursPerDay float64 `json:"hours_per_day"` // above the threshold, last 30 days
	AvgPlayers  float64 `json:"avg_players"`   // during those hours
	HistoryDays float64 `json:"history_days"`  // how much history the attendance comes from
	MBPerHour   float64 `json:"mb_per_hour"`
	Measured    bool    `json:"measured"` // MBPerHour comes from this server's demos
	GBPerDay    float64 `json:"gb_per_day"`
	KeepGB      float64 `json:"keep_gb"`            // for the chosen keep time
	SpaceGB     float64 `json:"space_gb,omitempty"` // usable for the demos (free disk + current demos - 20 % margin, or the reserve)
	MaxDays     float64 `json:"max_days,omitempty"` // advised maximum keep time
	Source      string  `json:"source"`             // "disk", "reserve" or "" (unknown space)
	Over        bool    `json:"over"`               // the keep time exceeds the space
}

// Estimate for a threshold, a keep time and the space known for the demos (disk of this machine, else the reserve).
func Calc(st *store.Store, id int64, min, keepHours int, reserveGB float64, status Status, now time.Time) (Estimate, error) {
	var e Estimate
	var err error
	e.HoursPerDay, e.AvgPlayers, e.HistoryDays, err = st.DemoAttendance(id, max(min, 1), now)
	if err != nil {
		return e, err
	}
	u, err := st.DemoUsage(id, now)
	if err != nil {
		return e, err
	}
	e.MBPerHour, e.Measured = DefaultMBPerHour, false
	if u.Measured >= 3 && u.MBPerHour > 0 {
		e.MBPerHour, e.Measured = u.MBPerHour, true
	}
	e.GBPerDay = e.AvgPlayers * e.HoursPerDay * e.MBPerHour / 1000
	e.KeepGB = e.GBPerDay * float64(keepHours) / 24
	switch {
	case status.DiskTotal > 0:
		e.Source = "disk"
		e.SpaceGB = max(0, float64(status.DiskFree+u.Bytes)-0.2*float64(status.DiskTotal)) / 1e9
	case reserveGB > 0:
		e.Source, e.SpaceGB = "reserve", reserveGB
	}
	if e.Source != "" && e.GBPerDay > 0 {
		e.MaxDays = e.SpaceGB / e.GBPerDay
		e.Over = e.KeepGB > e.SpaceGB
	}
	return e, nil
}
