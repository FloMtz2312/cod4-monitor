//go:build !unix

package watch

// diskPct: not measured outside Unix systems (the VPS alerts are for a Linux server).
func diskPct(string) (float64, bool) { return 0, false }
