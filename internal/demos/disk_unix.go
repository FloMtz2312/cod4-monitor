//go:build unix

package demos

import (
	"net/url"
	"syscall"

	"cod4monitor/internal/logtail"
)

// diskSpace: free and total bytes of the disk holding the demos, when they are on this machine (0, 0 otherwise).
func diskSpace(base *url.URL, dir string) (free, total int64) {
	if !logtail.IsLocal(base) {
		return 0, 0
	}
	var st syscall.Statfs_t
	if syscall.Statfs(dir, &st) != nil {
		return 0, 0
	}
	return int64(st.Bavail) * int64(st.Bsize), int64(st.Blocks) * int64(st.Bsize)
}
