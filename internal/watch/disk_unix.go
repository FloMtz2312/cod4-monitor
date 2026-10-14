//go:build unix

package watch

import "syscall"

// diskPct: how full the disk holding dir is, in %.
func diskPct(dir string) (float64, bool) {
	var fs syscall.Statfs_t
	if syscall.Statfs(dir, &fs) != nil || fs.Blocks == 0 {
		return 0, false
	}
	used := float64(fs.Blocks - fs.Bfree)
	return 100 * used / (used + float64(fs.Bavail)), true
}
