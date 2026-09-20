//go:build !windows

package cache

import "os"

func openCacheFile(path string) (*os.File, error) {
	return os.Open(path)
}
