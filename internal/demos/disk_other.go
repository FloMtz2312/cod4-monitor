//go:build !unix

package demos

import "net/url"

func diskSpace(*url.URL, string) (free, total int64) { return 0, 0 }
